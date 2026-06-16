import { test, expect } from '@playwright/test';

test.describe('Mint Bonds', () => {

  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('[data-testid="statusBadge"]')).toContainText(/Factory ready|Connected|✅/i, { timeout: 60000 });
  });

  test('should mint a bond from admin to alice', async ({ page }) => {
    await page.locator('[data-testid="nav-mint"]').click();
    await expect(page.locator('[data-testid="page-mint"]')).toBeVisible();

    // Fill mint form
    await page.locator('[data-testid="mintAdmin"]').selectOption('admin');
    await page.locator('[data-testid="mintOwner"]').selectOption('alice');
    await page.locator('[data-testid="mintAmount"]').fill('1000');
    await page.locator('[data-testid="mintCoupon"]').fill('5');
    await page.locator('[data-testid="mintMaturity"]').fill('2028-12-31');
    await page.locator('[data-testid="mintDescription"]').fill('Corporate Bond A');
    await page.locator('[data-testid="mintSubmit"]').click();

    const result = page.locator('[data-testid="mintResult"]');
    await expect(result).toContainText(/✅|Bond minted/i, { timeout: 10000 });
  });

  test('should mint a bond from admin to admin', async ({ page }) => {
    await page.locator('[data-testid="nav-mint"]').click();

    await page.locator('[data-testid="mintAdmin"]').selectOption('admin');
    await page.locator('[data-testid="mintOwner"]').selectOption('admin');
    await page.locator('[data-testid="mintAmount"]').fill('500');
    await page.locator('[data-testid="mintCoupon"]').fill('3.5');
    await page.locator('[data-testid="mintMaturity"]').fill('2029-06-30');
    await page.locator('[data-testid="mintDescription"]').fill('Treasury Bond');
    await page.locator('[data-testid="mintSubmit"]').click();

    const result = page.locator('[data-testid="mintResult"]');
    await expect(result).toContainText(/✅|Bond minted/i, { timeout: 10000 });
  });

  test('should show error when minting cross-participant (admin on p1, bob on p2)', async ({ page }) => {
    await page.locator('[data-testid="nav-mint"]').click();

    await page.locator('[data-testid="mintAdmin"]').selectOption('admin');
    await page.locator('[data-testid="mintOwner"]').selectOption('bob');
    await page.locator('[data-testid="mintAmount"]').fill('750');
    await page.locator('[data-testid="mintCoupon"]').fill('4.5');
    await page.locator('[data-testid="mintMaturity"]').fill('2028-06-15');
    await page.locator('[data-testid="mintDescription"]').fill('Bob Bond');
    await page.locator('[data-testid="mintSubmit"]').click();

    const result = page.locator('[data-testid="mintResult"]');
    await expect(result).toContainText(/admin and owner must be on the same participant/i, { timeout: 10000 });
  });

  test('should show error when minting cross-participant (admin on p1, charlie on p3)', async ({ page }) => {
    await page.locator('[data-testid="nav-mint"]').click();

    await page.locator('[data-testid="mintAdmin"]').selectOption('admin');
    await page.locator('[data-testid="mintOwner"]').selectOption('charlie');
    await page.locator('[data-testid="mintAmount"]').fill('600');
    await page.locator('[data-testid="mintCoupon"]').fill('5.5');
    await page.locator('[data-testid="mintMaturity"]').fill('2027-12-31');
    await page.locator('[data-testid="mintDescription"]').fill('Charlie Bond');
    await page.locator('[data-testid="mintSubmit"]').click();

    const result = page.locator('[data-testid="mintResult"]');
    await expect(result).toContainText(/admin and owner must be on the same participant/i, { timeout: 10000 });
  });

  test('should show updated dashboard after minting multiple bonds', async ({ page }) => {
    // Mint a bond
    await page.locator('[data-testid="nav-mint"]').click();
    await page.locator('[data-testid="mintAdmin"]').selectOption('admin');
    await page.locator('[data-testid="mintOwner"]').selectOption('alice');
    await page.locator('[data-testid="mintAmount"]').fill('1000');
    await page.locator('[data-testid="mintCoupon"]').fill('5');
    await page.locator('[data-testid="mintMaturity"]').fill('2028-12-31');
    await page.locator('[data-testid="mintDescription"]').fill('Test Bond');
    await page.locator('[data-testid="mintSubmit"]').click();
    await expect(page.locator('[data-testid="mintResult"]')).toContainText(/✅|Bond minted/i, { timeout: 10000 });

    // Wait then check dashboard
    await page.waitForTimeout(2000);
    await page.locator('[data-testid="nav-dashboard"]').click();

    // Should have at least 1 bond
    const totalBonds = page.locator('[data-testid="stat-totalBonds"] p');
    await expect(totalBonds).not.toHaveText('-');
    await expect(totalBonds).not.toHaveText('0');
  });
});
