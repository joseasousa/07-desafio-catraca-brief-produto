import { expect, test } from '@playwright/test';

test.use({ baseURL: process.env.CATRACA_WEB });

const events = '/api/partner/events';
const ticket = (id) => '/api/partner/tickets/' + id;
const purchase = (id) => events + '/' + id + '/purchases';
const headers = { 'X-Api-Key': 'catraca-local-partner-key' };

async function organizer(page) {
  await page.goto('/entrar');
  await page.getByLabel('E-mail').fill('org-a@example.com');
  await page.getByLabel('Senha').fill('CatracaDemo123!');
  await page.getByRole('button', { name: 'Entrar' }).click();
}

async function createEvent(page, { name, capacity, published }) {
  await page.goto('/organizador/eventos/novo');
  const date = new Date(Date.now() + 7 * 86400000).toISOString().slice(0, 10);
  await page.getByLabel('Nome').fill(name);
  await page.getByLabel('Data e hora de início').fill(date + 'T21:00');
  await page.getByLabel('Local').fill('Porão');
  await page.getByLabel('Lotação').fill(String(capacity));
  await page.getByLabel('Preço em reais').fill('100,00');
  await page.getByRole('button', { name: 'Salvar rascunho' }).click();
  await expect(page.getByRole('heading', { name })).toBeVisible();
  if (published) {
    await page.getByRole('button', { name: 'Publicar' }).click();
    await expect(page.getByText('Publicado', { exact: true })).toBeVisible();
  }
  return { id: new URL(page.url()).pathname.split('/').at(-1), date };
}

async function expectJSON(response, status, body) {
  expect(response.status()).toBe(status);
  expect(response.headers()['content-type']).toContain('application/json');
  expect(await response.json()).toEqual(body);
}

test('F05: autenticação, precedência, compra, consulta e vagas', async ({ page, request }) => {
  await organizer(page);
  const draft = await createEvent(page, { name: 'F05 rascunho', capacity: 1, published: false });
  const ready = await createEvent(page, { name: 'F05 publicado', capacity: 1, published: true });
  const expectedEvent = {
    id: ready.id, name: 'F05 publicado', startsAt: ready.date + 'T21:00:00-03:00',
    priceCents: 10000, availableSeats: 1,
  };
  const list = await request.get(events, { headers });
  expect(list.status()).toBe(200);
  const listed = await list.json();
  expect(listed).toContainEqual(expectedEvent);
  expect(listed.some((entry) => entry.id === draft.id)).toBe(false);

  for (const invalidKey of [{}, { 'X-Api-Key': 'errada' }]) {
    const unauthorized = { error: 'unauthorized' };
    await expectJSON(await request.get(events, { headers: invalidKey }), 401, unauthorized);
    await expectJSON(await request.post(purchase('missing'), { headers: invalidKey, data: '{' }), 401, unauthorized);
    await expectJSON(await request.get(ticket('missing'), { headers: invalidKey }), 401, unauthorized);
  }

  const buyerEmail = 'f05@example.com';
  const cardNumber = '4000000000000003';
  for (const data of [
    { buyerEmail: 'inválido', cardNumber },
    { cardNumber },
    { buyerEmail, cardNumber: '123' },
    '{',
  ]) {
    await expectJSON(await request.post(purchase('missing'), { headers, data }), 422, { error: 'invalid_request' });
  }

  const data = { buyerEmail, cardNumber };
  for (const id of ['missing', draft.id]) {
    await expectJSON(await request.post(purchase(id), { headers, data }), 404, { error: 'event_not_available' });
  }
  const accepted = await request.post(purchase(ready.id), { headers, data });
  expect(accepted.status()).toBe(202);
  const pending = await accepted.json();
  expect(pending).toEqual({ ticketId: expect.any(String), code: expect.any(String), status: 'pending' });
  await expectJSON(await request.get(ticket(pending.ticketId), { headers }), 200, {
    ...pending, eventId: ready.id, checkedIn: false,
  });
  const updated = await (await request.get(events, { headers })).json();
  expect(updated).toContainEqual({ ...expectedEvent, availableSeats: 0 });
  await expectJSON(await request.post(purchase(ready.id), { headers, data }), 409, { error: 'sold_out' });
  await expectJSON(await request.get(ticket('missing'), { headers }), 404, { error: 'ticket_not_found' });
});

test('F05: duas rajadas de 30 compras HTTP disputam cinco lugares', async ({ page, request }) => {
  await organizer(page);
  for (let round = 1; round <= 2; round++) {
    const { id } = await createEvent(page, { name: 'F05 rajada ' + round, capacity: 5, published: true });
    const responses = await Promise.all(Array.from({ length: 30 }, (_, index) =>
      request.post(purchase(id), {
        headers, data: { buyerEmail: 'carga' + index + '@example.com', cardNumber: '4000000000000001' },
      }),
    ));
    expect(responses.map((response) => response.status()).sort()).toEqual([
      ...Array(5).fill(202), ...Array(25).fill(409),
    ]);
    const accepted = await Promise.all(responses.filter((response) => response.status() === 202).map((response) => response.json()));
    await expect.poll(async () => {
      const tickets = await Promise.all(accepted.map(async ({ ticketId }) =>
        (await request.get(ticket(ticketId), { headers })).json(),
      ));
      return tickets.map(({ status }) => status);
    }, { timeout: 8000 }).toEqual(Array(5).fill('confirmed'));
    const listed = await (await request.get(events, { headers })).json();
    expect(listed.find((entry) => entry.id === id).availableSeats).toBe(0);
  }
});
