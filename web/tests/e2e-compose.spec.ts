import { test, expect } from '@playwright/test';

// Compose-level end-to-end smoke.
// Gated on E2E_COMPOSE=1 — the default CI run skips this.
// Before running: docker compose up -d && wait ~2 min for Oracle to become healthy.
//
// Run locally:
//   E2E_COMPOSE=1 WEBUI_URL=http://localhost:3000 npx playwright test tests/e2e-compose.spec.ts

const composeEnabled = process.env.E2E_COMPOSE === '1';
const baseURL = process.env.WEBUI_URL ?? 'http://localhost:3000';

test.describe('compose-level smoke', () => {
	test.skip(!composeEnabled, 'Set E2E_COMPOSE=1 to enable compose-level e2e tests');

	test('webui responds on root', async ({ page }) => {
		const response = await page.goto(baseURL);
		expect(response?.ok()).toBeTruthy();
	});

	test('login form renders and auth succeeds', async ({ page }) => {
		await page.goto(baseURL);
		await page.getByLabel(/username/i).fill('demo');
		await page.getByLabel(/password/i).fill('demo');
		await page.getByRole('button', { name: /sign in|log in/i }).click();
		await expect(page.getByRole('main')).toBeVisible({ timeout: 15_000 });
	});

	test('sessions sidebar loads from picooraclaw', async ({ page }) => {
		await page.goto(baseURL);
		await page.getByLabel(/username/i).fill('demo');
		await page.getByLabel(/password/i).fill('demo');
		await page.getByRole('button', { name: /sign in|log in/i }).click();
		await expect(page.getByRole('complementary')).toBeVisible({ timeout: 15_000 });
	});

	test('memory drawer opens and accepts a search query', async ({ page }) => {
		await page.goto(baseURL);
		await page.getByLabel(/username/i).fill('demo');
		await page.getByLabel(/password/i).fill('demo');
		await page.getByRole('button', { name: /sign in|log in/i }).click();
		const memoryButton = page.getByRole('button', { name: /memory/i });
		await memoryButton.click();
		await page.getByPlaceholder(/search memory/i).fill('oracle');
		await page.keyboard.press('Enter');
		// Drawer should stay visible after the query returns.
		await expect(page.getByRole('dialog')).toBeVisible({ timeout: 10_000 });
	});
});
