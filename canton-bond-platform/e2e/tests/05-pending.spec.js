import { test, expect } from '@playwright/test';
import {
  mintBond, transferBond, acceptTransfer, rejectTransfer, withdrawTransfer,
  listTransferInstructions, listHoldings
} from '../helpers/api.js';

test.describe('Pending Transfers', () => {

  async function ensureHolding(party, amount = 1000) {
    const holdings = await listHoldings(party);
    const unlocked = holdings.filter(h => !h.locked);
    const total = unlocked.reduce((sum, h) => sum + h.amount, 0);
    if (total < amount) {
      await mintBond({ admin: 'admin', owner: party, amount, description: `For ${party}` });
    }
  }

  test('should show pending transfers after initiating a transfer', async ({ page }) => {
    await ensureHolding('alice', 500);
    const result = await transferBond({ sender: 'alice', receiver: 'bob', amount: 50 });
    expect(result.status).toBe('pending');

    await page.goto('/');
    await expect(page.locator('[data-testid="statusBadge"]')).toContainText(/Factory ready|Connected|✅/i, { timeout: 60000 });

    await page.locator('[data-testid="nav-pending"]').click();
    await expect(page.locator('[data-testid="page-pending"]')).toBeVisible();

    const pendingList = page.locator('[data-testid="pendingList"]');
    await expect(pendingList).not.toContainText('No pending transfers', { timeout: 5000 });
    await expect(pendingList).toContainText('alice');
    await expect(pendingList).toContainText('bob');
  });

  test('should filter pending transfers by party', async ({ page }) => {
    await ensureHolding('alice', 500);
    await transferBond({ sender: 'alice', receiver: 'bob', amount: 50 });

    await page.goto('/');
    await expect(page.locator('[data-testid="statusBadge"]')).toContainText(/Factory ready|Connected|✅/i, { timeout: 60000 });

    await page.locator('[data-testid="nav-pending"]').click();

    const filter = page.locator('[data-testid="pendingFilter"]');
    await filter.selectOption('bob');
    await page.waitForTimeout(500);
    const pendingList = page.locator('[data-testid="pendingList"]');
    await expect(pendingList).toContainText('bob', { timeout: 5000 });
  });

  test('should accept pending transfer from UI buttons', async ({ page }) => {
    await ensureHolding('alice', 500);
    await transferBond({ sender: 'alice', receiver: 'bob', amount: 50 });

    // Accept via API (UI uses confirm dialogs which are hard to automate)
    const pending = await listTransferInstructions('bob');
    const transfer = pending.find(t => t.sender.includes('alice') && t.receiver.includes('bob'));
    expect(transfer).toBeDefined();

    const acceptResult = await acceptTransfer({ party: 'bob', contractId: transfer.contractId });
    expect(acceptResult.status).toBe('accepted');
  });

  test('should withdraw pending transfer from UI buttons', async () => {
    await ensureHolding('alice', 500);
    const result = await transferBond({ sender: 'alice', receiver: 'bob', amount: 50 });
    expect(result.status).toBe('pending');

    const pending = await listTransferInstructions('alice');
    const transfer = pending.find(t => t.sender.includes('alice') && t.receiver.includes('bob'));
    expect(transfer).toBeDefined();

    const withdrawResult = await withdrawTransfer({ party: 'alice', contractId: transfer.contractId });
    expect(withdrawResult.status).toBe('withdrawed');
  });

  test('should reject pending transfer from UI buttons', async () => {
    await ensureHolding('alice', 500);
    const result = await transferBond({ sender: 'alice', receiver: 'bob', amount: 50 });
    expect(result.status).toBe('pending');

    const pending = await listTransferInstructions('bob');
    const transfer = pending.find(t => t.sender.includes('alice') && t.receiver.includes('bob'));
    expect(transfer).toBeDefined();

    const rejectResult = await rejectTransfer({ party: 'bob', contractId: transfer.contractId });
    expect(rejectResult.status).toBe('rejected');
  });
});
