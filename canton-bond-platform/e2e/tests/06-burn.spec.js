import { test, expect } from '@playwright/test';
import { mintBond, listHoldings, burnBond } from '../helpers/api.js';

test.describe('Burn Bonds', () => {

  async function ensureHolding(party, amount = 1000) {
    const holdings = await listHoldings(party);
    const unlocked = holdings.filter(h => !h.locked);
    const total = unlocked.reduce((sum, h) => sum + h.amount, 0);
    if (total < amount) {
      await mintBond({ admin: 'admin', owner: party, amount, description: `Burn test for ${party}` });
    }
  }

  test('should burn a bond as the owner', async () => {
    await ensureHolding('alice', 500);

    const holdings = await listHoldings('alice');
    const unlocked = holdings.filter(h => !h.locked);
    expect(unlocked.length).toBeGreaterThan(0);

    const toBurn = unlocked[0];
    const result = await burnBond({ party: 'alice', contractId: toBurn.contractId, asAdmin: false });
    expect(result.status).toBe('burned');
  });

  test('should burn a bond as admin', async () => {
    await ensureHolding('admin', 500);

    const holdings = await listHoldings('admin');
    const unlocked = holdings.filter(h => !h.locked);
    expect(unlocked.length).toBeGreaterThan(0);

    const toBurn = unlocked[0];
    const result = await burnBond({ party: 'admin', contractId: toBurn.contractId, asAdmin: true });
    expect(result.status).toBe('burned');
  });

  test('should fail to burn a non-existent contract ID', async () => {
    const fakeCid = 'nonexistent-contract-id';
    await expect(
      burnBond({ party: 'alice', contractId: fakeCid })
    ).rejects.toThrow();
  });

  test('should show burn page with holdings list', async ({ page }) => {
    await ensureHolding('alice', 500);

    await page.goto('/');
    await expect(page.locator('[data-testid="statusBadge"]')).toContainText(/Factory ready|Connected|✅/i, { timeout: 60000 });

    await page.locator('[data-testid="nav-burn"]').click();
    await expect(page.locator('[data-testid="page-burn"]')).toBeVisible();

    const burnHoldings = page.locator('[data-testid="burnHoldingsList"]');
    await expect(burnHoldings).toBeVisible({ timeout: 5000 });
  });

  test('should burn a bond from the frontend', async ({ page }) => {
    await ensureHolding('alice', 500);

    await page.goto('/');
    await expect(page.locator('[data-testid="statusBadge"]')).toContainText(/Factory ready|Connected|✅/i, { timeout: 60000 });

    // Use API to get a valid contract ID
    const holdings = await listHoldings('alice');
    const unlocked = holdings.filter(h => !h.locked);
    expect(unlocked.length).toBeGreaterThan(0);

    await page.locator('[data-testid="nav-burn"]').click();
    await page.locator('[data-testid="burnParty"]').selectOption('alice');
    await page.waitForTimeout(500);

    await page.locator('[data-testid="burnContractId"]').fill(unlocked[0].contractId);
    await page.locator('[data-testid="burnSubmit"]').click();

    const result = page.locator('[data-testid="burnResult"]');
    await expect(result).toContainText(/✅|Bond burned/i, { timeout: 15000 });
  });
});
