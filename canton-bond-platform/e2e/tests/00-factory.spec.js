import { test, expect } from '@playwright/test';

test.describe('Factory Initialization', () => {

  test('should show factory ready status after start', async ({ page }) => {
    await page.goto('/');
    await page.waitForFunction(() => document.body.classList.contains('factory-locked'));

    const statusBadge = page.locator('[data-testid="statusBadge"]');
    await expect(statusBadge).toContainText(/Factory not started|Not started|press Start Factory/i);

    const startBtn = page.locator('[data-testid="startFactoryBtn"]');
    await expect(startBtn).toBeVisible();
    await expect(startBtn).toBeEnabled();

    await startBtn.click();

    await expect(statusBadge).toContainText(/Factory ready|Connected|✅|Retrying/i, { timeout: 30000 });

    await expect(statusBadge).toContainText(/Factory ready|Connected/i, { timeout: 30000 });
  });

  test('should show dashboard with stats after factory start', async ({ page }) => {
    await page.goto('/');

    const startBtn = page.locator('[data-testid="startFactoryBtn"]');
    // Wait for button to be clickable
    await expect(startBtn).toBeVisible({ timeout: 15000 });

    const statusBadge = page.locator('[data-testid="statusBadge"]');
    const text = await statusBadge.textContent();

    // Only click if factory is not ready
    if (text && !text.includes('Factory ready') && !text.includes('Connected')) {
      await startBtn.click();
      await expect(statusBadge).toContainText(/Factory ready|Connected/i, { timeout: 30000 });
    }

    await expect(page.locator('[data-testid="stat-totalBonds"]')).toBeVisible();
    await expect(page.locator('[data-testid="stat-totalValue"]')).toBeVisible();
    await expect(page.locator('[data-testid="stat-activeParties"]')).toBeVisible();
    await expect(page.locator('[data-testid="stat-pendingTransfers"]')).toBeVisible();
  });

  test('should have valid stat values after factory start', async ({ page }) => {
    await page.goto('/');

    const startBtn = page.locator('[data-testid="startFactoryBtn"]');
    await expect(startBtn).toBeVisible({ timeout: 15000 });

    const statusBadge = page.locator('[data-testid="statusBadge"]');
    const text = await statusBadge.textContent();

    if (text && !text.includes('Factory ready') && !text.includes('Connected')) {
      await startBtn.click();
      await expect(statusBadge).toContainText(/Factory ready|Connected/i, { timeout: 30000 });
    }

    const statParties = page.locator('[data-testid="stat-activeParties"] p');
    await expect(statParties).not.toHaveText('-');
    await expect(statParties).not.toHaveText('0');
  });
});
