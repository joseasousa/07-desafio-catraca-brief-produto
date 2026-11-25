import { expect, test } from '@playwright/test';

test.use({ baseURL: process.env.CATRACA_WEB });

const events = '/api/partner/events';
const headers = { 'X-Api-Key': 'catraca-local-partner-key' };
const approved = '4000000000000001';
const delay = (ms) => new Promise(resolve => setTimeout(resolve, ms));

async function login(page, email) {
  await page.goto('/entrar');
  await page.getByLabel('E-mail').fill(email);
  await page.getByLabel('Senha').fill('CatracaDemo123!');
  await page.getByRole('button', { name: 'Entrar' }).click();
}

async function createEvent(page, name, capacity, price) {
  await page.goto('/organizador/eventos/novo');
  const date = new Date(Date.now() + 7 * 86400000).toISOString().slice(0, 10);
  await page.getByLabel('Nome').fill(name);
  await page.getByLabel('Data e hora de início').fill(date + 'T21:00');
  await page.getByLabel('Local').fill('Porão');
  await page.getByLabel('Lotação').fill(String(capacity));
  await page.getByLabel('Preço em reais').fill(price);
  await page.getByRole('button', { name: 'Salvar rascunho' }).click();
  await page.getByRole('button', { name: 'Publicar' }).click();
  await expect(page.getByText('Publicado', { exact: true })).toBeVisible();
  const path = new URL(page.url()).pathname;
  return { id: path.split('/').at(-1), path, name };
}

function purchase(request, event, cardNumber) {
  return request.post(events + '/' + event.id + '/purchases', {
    headers, data: { buyerEmail: 'operacao@example.com', cardNumber },
  });
}

async function bought(request, event, cardNumber) {
  const response = await purchase(request, event, cardNumber);
  expect(response.status()).toBe(202);
  const ticket = await response.json();
  expect(ticket.status).toBe('pending');
  return ticket;
}

async function lookup(request, ticket) {
  const response = await request.get('/api/partner/tickets/' + ticket.ticketId, { headers });
  expect(response.status()).toBe(200);
  return response.json();
}

async function listed(request, event) {
  const response = await request.get(events, { headers });
  expect(response.status()).toBe(200);
  return (await response.json()).find(item => item.id === event.id);
}

async function checkIn(page, code, message) {
  await page.getByLabel('Código do ingresso').fill(code);
  await page.getByRole('button', { name: 'Fazer check-in' }).click();
  await expect(page.getByRole('status')).toHaveText(message);
}

async function panel(page, values) {
  const names = ['Confirmados', 'Pendentes', 'Check-ins', 'Vagas disponíveis', 'Receita', 'Estornados'];
  for (const [index, name] of names.entries()) {
    const row = page.locator('dl > div').filter({ has: page.getByText(name, { exact: true }) });
    await expect(row.locator('dd')).toHaveText(String(values[index]));
  }
}

test('F06: web e API compartilham vagas, check-in, painel e cancelamento', async ({ page, browser, baseURL, request }) => {
  test.setTimeout(120000);
  await login(page, 'org-a@example.com');
  const event = await createEvent(page, 'Jazz integrado', 2, '100,00');
  const buyer = await browser.newPage({ baseURL });
  await login(buyer, 'participante@example.com');
  const offer = buyer.locator('article').filter({ has: buyer.getByRole('heading', { name: event.name }) });
  await offer.getByLabel('Cartão').fill(approved);
  await offer.getByRole('button', { name: 'Comprar ingresso' }).click();
  const webTicket = buyer.locator('article').filter({ has: buyer.getByRole('heading', { name: event.name }) });
  await expect(webTicket.getByText('Status: pendente')).toBeVisible();
  const webCode = await webTicket.locator('code').innerText();
  await expect.poll(async () => {
    await buyer.reload();
    return webTicket.getByText('Status: confirmado').count();
  }, { timeout: 5000 }).toBe(1);

  const rejected = await bought(request, event, '4000000000000004');
  expect((await listed(request, event)).availableSeats).toBe(0);
  const soldOut = await purchase(request, event, approved);
  expect(soldOut.status()).toBe(409);
  expect(await soldOut.json()).toEqual({ error: 'sold_out' });
  await buyer.goto('/');
  await expect(offer.getByText('Ingressos esgotados')).toBeVisible();
  await expect.poll(async () => (await lookup(request, rejected)).status,
    { timeout: 45000, intervals: [100, 500, 1000] }).toBe('declined');
  expect((await listed(request, event)).availableSeats).toBe(1);
  const second = await bought(request, event, approved);
  await expect.poll(async () => (await lookup(request, second)).status, { timeout: 5000 }).toBe('confirmed');

  await page.getByLabel('Lotação').fill('1');
  await page.getByRole('button', { name: 'Salvar alterações' }).click();
  await expect(page.getByRole('alert')).toHaveText('Lotação menor que as vagas ocupadas');
  await page.getByLabel('Lotação').fill('3');
  await page.getByLabel('Preço em reais').fill('150,00');
  await page.getByRole('button', { name: 'Salvar alterações' }).click();
  const third = await bought(request, event, approved);
  await expect.poll(async () => (await lookup(request, third)).status, { timeout: 5000 }).toBe('confirmed');
  await page.goto(event.path);
  await panel(page, [3, 0, 0, 0, 'R$ 350,00', 0]);

  const other = await browser.newPage({ baseURL });
  await login(other, 'org-b@example.com');
  const otherEvent = await createEvent(other, 'Segundo evento', 1, '20,00');
  const otherTicket = await bought(request, otherEvent, approved);
  const foreignResponse = await other.goto(event.path);
  expect(foreignResponse.status()).toBe(404);
  await other.goto(otherEvent.path);
  const csrf = await other.locator('input[name=csrf]').first().inputValue();
  for (const endpoint of ['check-in', 'cancelar']) {
    const blocked = await other.request.post(event.path + '/' + endpoint, {
      form: { csrf, code: webCode }, maxRedirects: 0,
    });
    expect(blocked.status()).toBe(404);
  }

  await checkIn(page, webCode, 'Entrada liberada');
  await checkIn(page, webCode, 'Ingresso já utilizado');
  await checkIn(page, rejected.code, 'Ingresso não confirmado');
  await checkIn(page, 'ZZZZZZZZ', 'Ingresso inválido para este evento');
  await checkIn(page, otherTicket.code, 'Ingresso inválido para este evento');
  await checkIn(page, second.code, 'Entrada liberada');
  expect((await lookup(request, second)).checkedIn).toBe(true);
  await panel(page, [3, 0, 2, 0, 'R$ 350,00', 0]);

  const slowEvent = await createEvent(page, 'Evento cancelado na operação', 5, '50,00');
  const slowStarted = Date.now();
  const slow = await bought(request, slowEvent, '4000000000000003');
  const fast = await bought(request, slowEvent, approved);
  await expect.poll(async () => (await lookup(request, fast)).status, { timeout: 5000 }).toBe('confirmed');
  const editForm = new URLSearchParams(await page.locator('form').filter({
    has: page.getByRole('button', { name: 'Salvar alterações' }),
  }).evaluate(form => [...new FormData(form)]));
  await page.getByRole('button', { name: 'Cancelar evento' }).click();
  await expect(page.getByText('Evento cancelado', { exact: true })).toBeVisible();
  expect(await listed(request, slowEvent)).toBeUndefined();
  await buyer.goto('/');
  await expect(buyer.getByRole('heading', { name: slowEvent.name })).toHaveCount(0);
  const unavailable = await purchase(request, slowEvent, approved);
  expect(unavailable.status()).toBe(404);
  expect(await unavailable.json()).toEqual({ error: 'event_not_available' });
  await expect(page.getByRole('button', { name: 'Salvar alterações' })).toHaveCount(0);
  await expect(page.getByLabel('Código do ingresso')).toHaveCount(0);
  for (const path of [slowEvent.path, slowEvent.path + '/publicar']) {
    expect((await page.request.post(path, { form: Object.fromEntries(editForm) })).status()).toBe(409);
  }
  expect((await lookup(request, slow)).status).toBe('cancelled');
  expect((await lookup(request, fast)).status).toBe('refunded');
  await panel(page, [0, 0, 0, 0, 'R$ 0,00', 1]);

  await page.goto(event.path);
  await page.getByRole('button', { name: 'Cancelar evento' }).click();
  await buyer.goto('/meus-ingressos');
  await expect(webTicket.getByText('Status: estornado')).toBeVisible();
  expect(await lookup(request, second)).toMatchObject({ status: 'refunded', checkedIn: true });
  await panel(page, [0, 0, 0, 0, 'R$ 0,00', 3]);

  await delay(Math.max(0, slowStarted + 45000 - Date.now()));
  expect((await lookup(request, slow)).status).toBe('cancelled');
  expect((await lookup(request, fast)).status).toBe('refunded');
  await buyer.close();
  await other.close();
});
