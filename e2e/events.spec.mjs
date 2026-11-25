import { expect, test } from '@playwright/test';

test.use({ baseURL: process.env.CATRACA_WEB });

async function login(page, email) {
  await page.goto('/entrar');
  await page.getByLabel('E-mail').fill(email);
  await page.getByLabel('Senha').fill('CatracaDemo123!');
  await page.getByRole('button', { name: 'Entrar' }).click();
}

test('F02: eventos publicados e gestão exclusiva do organizador dono', async ({ page, browser, baseURL }) => {
  const name = 'Jazz no Porão';
  const future = new Date(Date.now() + 7 * 86400000).toISOString().slice(0, 10) + 'T21:00';
  const past = new Date(Date.now() - 2 * 86400000).toISOString().slice(0, 10) + 'T21:00';
  await login(page, 'org-a@example.com');
  await page.getByRole('link', { name: 'Criar evento' }).click();
  await page.getByLabel('Nome').fill(name);
  await page.getByLabel('Data e hora de início').fill(past);
  await page.getByLabel('Local').fill('Porão');
  await page.getByLabel('Lotação').fill('2');
  await page.getByLabel('Preço em reais').fill('100,00');
  await page.getByRole('button', { name: 'Salvar rascunho' }).click();
  await expect(page.getByRole('alert')).toContainText('início no futuro');
  await page.getByLabel('Data e hora de início').fill(future);
  await page.getByRole('button', { name: 'Salvar rascunho' }).click();
  await expect(page.getByText('Rascunho', { exact: true })).toBeVisible();
  const management = page.url();

  const visitor = await browser.newPage({ baseURL });
  await visitor.goto('/');
  await expect(visitor.getByRole('heading', { name })).toHaveCount(0);
  await page.getByRole('button', { name: 'Publicar' }).click();
  await expect(page.getByText('Publicado', { exact: true })).toBeVisible();
  await expect(page).toHaveURL(management);
  await visitor.reload();
  await expect(visitor.getByRole('heading', { name })).toBeVisible();
  await expect(visitor.getByText('Vagas disponíveis: 2')).toBeVisible();

  await page.getByLabel('Data e hora de início').fill(past);
  await page.getByRole('button', { name: 'Salvar alterações' }).click();
  await expect(page.getByRole('alert')).toContainText('início no futuro');
  await page.reload();
  await expect(page.getByLabel('Data e hora de início')).toHaveValue(future);
  await page.getByLabel('Lotação').fill('3');
  await page.getByRole('button', { name: 'Salvar alterações' }).click();
  await expect(page.getByLabel('Lotação')).toHaveValue('3');
  await visitor.reload();
  await expect(visitor.getByText('Vagas disponíveis: 3')).toBeVisible();

  await visitor.goto(management);
  await expect(visitor).toHaveURL(/\/entrar$/);
  await expect(visitor.getByText(name)).toHaveCount(0);
  await visitor.close();

  for (const [email, status] of [['org-b@example.com', 404], ['participante@example.com', 403]]) {
    const other = await browser.newPage({ baseURL });
    await login(other, email);
    const list = await other.goto('/organizador');
    expect(list.status()).toBe(status === 404 ? 200 : 403);
    await expect(other.getByText(name)).toHaveCount(0);
    const response = await other.goto(management);
    expect(response.status()).toBe(status);
    await expect(other.getByText(name)).toHaveCount(0);
    await other.close();
  }
});
