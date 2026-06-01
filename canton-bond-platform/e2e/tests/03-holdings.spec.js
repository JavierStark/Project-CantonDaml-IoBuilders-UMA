import { test, expect } from '@playwright/test';
import { mintBond, transferBond, acceptTransfer, listTransferInstructions } from '../helpers/api.js';

// Helper to seed a holding for a cross-participant party
async function seedHoldingFor(party, amount = 500) {
  try {
    await mintBond({ admin: 'admin', owner: 'alice', amount, description: `For ${party}` });
    if (party !== 'alice') {
      await transferBond({ sender: 'alice', receiver: party, amount });
      const pending = await listTransferInstructions(party);
      const transfer = pending.find(t => t.sender.includes('alice') && t.receiver.includes(party));
      if (transfer) {
        await acceptTransfer({ party, contractId: transfer.contractId });
      }
    }
  } catch (_) {}
}

test.describe('Holdings View', () => {

  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    const startBtn = page.locator('[data-testid="startFactoryBtn"]');
    await expect(startBtn).toBeVisible({ timeout: 15000 });
    const statusBadge = page.locator('[data-testid="statusBadge"]');
    const text = await statusBadge.textContent();
    if (text && !text.includes('Factory ready') && !text.includes('Connected')) {
      await startBtn.click();
      await expect(statusBadge).toContainText(/Factory ready|Connected/i, { timeout: 30000 });
    }
  });

  test('should show holdings page with no filter', async ({ page }) => {
    await seedHoldingFor('alice');

    await page.locator('[data-testid="nav-holdings"]').click();
    await expect(page.locator('[data-testid="page-holdings"]')).toBeVisible();

    const holdingsList = page.locator('[data-testid="holdingsList"]');
    await expect(holdingsList).toBeVisible({ timeout: 5000 });
  });

  test('should filter holdings by party', async ({ page }) => {
    await seedHoldingFor('alice');

    await page.locator('[data-testid="nav-holdings"]').click();

    // Filter by alice
    const filter = page.locator('[data-testid="holdingsFilter"]');
    await filter.selectOption('alice');
    await page.waitForTimeout(1500);

    const holdingsList = page.locator('[data-testid="holdingsList"]');
    await expect(holdingsList).toContainText('alice', { timeout: 5000 });
  });
});
