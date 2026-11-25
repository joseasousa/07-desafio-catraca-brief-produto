import { expect, test } from '@playwright/test';

test.use({ baseURL: process.env.CATRACA_WEB });

test('F04: reserva a última vaga e só mostra o ingresso ao comprador', async ({ page, browser, baseURL }) => {
  const name = 'Workshop de cerâmica';
  const future = new Date(Date.now() + 7 * 86400000).toISOString().slice(0, 10) + 'T20:00';
  await page.goto('/entrar');
  await page.getByLabel('E-mail').fill('org-a@example.com');
  await page.getByLabel('Senha').fill('CatracaDemo123!');
  await page.getByRole('button', { name: 'Entrar' }).click();
  await page.getByRole('link', { name: 'Criar evento' }).click();
  await page.getByLabel('Nome').fill(name);
  await page.getByLabel('Data e hora de início').fill(future);
  await page.getByLabel('Local').fill('Ateliê');
  await page.getByLabel('Lotação').fill('1');
  await page.getByLabel('Preço em reais').fill('50,00');
  await page.getByRole('button', { name: 'Salvar rascunho' }).click();
  await page.getByRole('button', { name: 'Publicar' }).click();
  await expect(page.getByText('Publicado', { exact: true })).toBeVisible();
  const eventID = new URL(page.url()).pathname.split('/').at(-1);
  const purchaseURL = `/eventos/${eventID}/comprar`;
  const organizerAttempt = await page.request.post(purchaseURL);
  expect(organizerAttempt.status()).toBe(403);
  const organizerTickets = await page.goto('/meus-ingressos');
  expect(organizerTickets.status()).toBe(403);

  const buyer = await browser.newPage({ baseURL });
  await buyer.goto('/');
  await expect(buyer.getByRole('link', { name: 'Entre para comprar' })).toBeVisible();
  const visitorAttempt = await buyer.request.post(purchaseURL, { maxRedirects: 0 });
  expect(visitorAttempt.status()).toBe(303);
  expect(visitorAttempt.headers().location).toBe('/entrar');
  await buyer.goto('/meus-ingressos');
  await expect(buyer).toHaveURL(/\/entrar$/);

  await buyer.getByRole('link', { name: 'Criar conta' }).click();
  await buyer.getByLabel('Nome').fill('Participante do workshop');
  await buyer.getByLabel('E-mail').fill('checkout@example.com');
  await buyer.getByLabel('Senha').fill('senha de demonstração');
  await buyer.getByRole('button', { name: 'Criar conta' }).click();

  const other = await browser.newPage({ baseURL });
  await other.goto('/entrar');
  await other.getByLabel('E-mail').fill('participante@example.com');
  await other.getByLabel('Senha').fill('CatracaDemo123!');
  await other.getByRole('button', { name: 'Entrar' }).click();
  const staleForm = other.locator('article').filter({ has: other.getByRole('heading', { name }) });
  await staleForm.getByLabel('Cartão').fill('4000000000000001');

  const offer = buyer.locator('article').filter({ has: buyer.getByRole('heading', { name }) });
  await offer.getByLabel('Cartão').fill('4000000000000001');
  await offer.getByRole('button', { name: 'Comprar ingresso' }).click();
  await expect(buyer).toHaveURL(/\/meus-ingressos$/);
  const ticket = buyer.locator('article').filter({ has: buyer.getByRole('heading', { name }) });
  await expect(ticket.getByText('Status: pendente')).toBeVisible();
  await expect(ticket.locator('code')).not.toBeEmpty();
  const code = await ticket.locator('code').innerText();

  await staleForm.getByRole('button', { name: 'Comprar ingresso' }).click();
  await expect(other.getByRole('alert')).toHaveText('Ingressos esgotados');
  await other.getByRole('link', { name: 'Vitrine', exact: true }).click();
  await expect(staleForm.getByText('Ingressos esgotados')).toBeVisible();
  await expect(staleForm.getByRole('button', { name: 'Comprar ingresso' })).toHaveCount(0);
  await other.getByRole('link', { name: 'Meus ingressos' }).click();
  await expect(other.getByText('Você ainda não comprou ingressos.')).toBeVisible();
  await expect(other.getByText(code)).toHaveCount(0);

  await expect(async () => {
    await buyer.reload();
    await expect(ticket.getByText('Status: confirmado')).toBeVisible({ timeout: 300 });
  }).toPass({ timeout: 5000 });
  await expect(ticket.locator('code')).toHaveText(code);
  await buyer.close();
  await other.close();
});
