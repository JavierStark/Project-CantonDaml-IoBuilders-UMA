import { test, expect } from '@playwright/test';

test.describe('Factory Initialization', () => {

  test('should auto-initialize factory on page load', async ({ page }) => {
    await page.goto('/');
    await page.waitForFunction(() =>
      document.body.classList.contains('factory-locked') ||
      document.querySelector('[data-testid="statusBadge"]')?.textContent?.match(/Factory ready|Connected|✅/i)
    );

    const statusBadge = page.locator('[data-testid="statusBadge"]');
    await expect(statusBadge).toContainText(/Factory ready|Connected|✅/i, { timeout: 60000 });
  });

  test('should show dashboard with stats after factory start', async ({ page }) => {
    await page.goto('/');

    const statusBadge = page.locator('[data-testid="statusBadge"]');
    await expect(statusBadge).toContainText(/Factory ready|Connected|✅/i, { timeout: 60000 });

    await expect(page.locator('[data-testid="stat-totalBonds"]')).toBeVisible();
    await expect(page.locator('[data-testid="stat-totalValue"]')).toBeVisible();
    await expect(page.locator('[data-testid="stat-activeParties"]')).toBeVisible();
    await expect(page.locator('[data-testid="stat-pendingTransfers"]')).toBeVisible();
  });

  test('should have valid stat values after factory start', async ({ page }) => {
    await page.goto('/');

    const statusBadge = page.locator('[data-testid="statusBadge"]');
    await expect(statusBadge).toContainText(/Factory ready|Connected|✅/i, { timeout: 60000 });

    const statParties = page.locator('[data-testid="stat-activeParties"] p');
    await expect(statParties).not.toHaveText('-');
  });
});
