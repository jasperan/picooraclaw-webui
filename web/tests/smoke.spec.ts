import { test, expect } from '@playwright/test';

test('loads and shows login or chat', async ({ page }) => {
	await page.goto('/');
	// Either login form or chat feed must be visible.
	const hasLogin = (await page.locator('input[type=password]').count()) > 0;
	const hasFeed = (await page.locator('.feed, main').count()) > 0;
	expect(hasLogin || hasFeed).toBe(true);
});
