import { test, expect } from '@playwright/test';
import {
  mintBond, transferBond, acceptTransfer, rejectTransfer, withdrawTransfer,
  listHoldings, listTransferInstructions, getAllHoldings
} from '../helpers/api.js';

test.describe('Transfer All Combinations', () => {

  // Helper: ensure at least one holding exists for a party.
  // Mint only works when admin and owner are on the same participant (participant1).
  // For cross-participant transfers, mint to alice first, then transfer to the target party.
  async function ensureHolding(party, amount = 1000) {
    const holdings = await listHoldings(party);
    const unlocked = holdings.filter(h => !h.locked);
    const total = unlocked.reduce((sum, h) => sum + h.amount, 0);
    if (total >= amount) return;
    // Mint to alice (on p1) then transfer to the target party if needed
    await mintBond({ admin: 'admin', owner: 'alice', amount, description: `Holding for transfer` });
    if (party !== 'alice') {
      await transferBond({ sender: 'alice', receiver: party, amount });
      // Accept the transfer as the receiver
      const pending = await listTransferInstructions(party);
      const transfer = pending.find(t => t.sender.includes('alice') && t.receiver.includes(party));
      if (transfer) {
        await acceptTransfer({ party, contractId: transfer.contractId });
      }
    }
  }

  test.describe('Cross-participant: alice (p1) → bob (p2)', () => {
    test('should transfer from alice to bob and bob accepts', async () => {
      await ensureHolding('alice', 500);

      const result = await transferBond({ sender: 'alice', receiver: 'bob', amount: 100 });
      expect(result.status).toBe('pending');

      // Bob accepts
      const pending = await listTransferInstructions('bob');
      expect(pending.length).toBeGreaterThan(0);
      const transfer = pending.find(t => t.sender.includes('alice') && t.receiver.includes('bob'));
      expect(transfer).toBeDefined();

      const acceptResult = await acceptTransfer({ party: 'bob', contractId: transfer.contractId });
      expect(acceptResult.status).toBe('accepted');
    });
  });

  test.describe('Cross-participant: bob (p2) → alice (p1)', () => {
    test('should transfer from bob to alice and alice accepts', async () => {
      await ensureHolding('bob', 500);

      const result = await transferBond({ sender: 'bob', receiver: 'alice', amount: 100 });
      expect(result.status).toBe('pending');

      const pending = await listTransferInstructions('alice');
      const transfer = pending.find(t => t.sender.includes('bob') && t.receiver.includes('alice'));
      expect(transfer).toBeDefined();

      const acceptResult = await acceptTransfer({ party: 'alice', contractId: transfer.contractId });
      expect(acceptResult.status).toBe('accepted');
    });
  });

  test.describe('Cross-participant: bob (p2) → charlie (p3)', () => {
    test('should transfer from bob to charlie and charlie accepts', async () => {
      await ensureHolding('bob', 500);

      const result = await transferBond({ sender: 'bob', receiver: 'charlie', amount: 100 });
      expect(result.status).toBe('pending');

      const pending = await listTransferInstructions('charlie');
      const transfer = pending.find(t => t.sender.includes('bob') && t.receiver.includes('charlie'));
      expect(transfer).toBeDefined();

      const acceptResult = await acceptTransfer({ party: 'charlie', contractId: transfer.contractId });
      expect(acceptResult.status).toBe('accepted');
    });
  });

  test.describe('Cross-participant: charlie (p3) → alice (p1)', () => {
    test('should transfer from charlie to alice and alice accepts', async () => {
      await ensureHolding('charlie', 500);

      const result = await transferBond({ sender: 'charlie', receiver: 'alice', amount: 100 });
      expect(result.status).toBe('pending');

      const pending = await listTransferInstructions('alice');
      const transfer = pending.find(t => t.sender.includes('charlie') && t.receiver.includes('alice'));
      expect(transfer).toBeDefined();

      const acceptResult = await acceptTransfer({ party: 'alice', contractId: transfer.contractId });
      expect(acceptResult.status).toBe('accepted');
    });
  });

  test.describe('Withdraw transfer: alice (p1) → bob (p2)', () => {
    test('should withdraw a pending transfer', async () => {
      await ensureHolding('alice', 500);

      const result = await transferBond({ sender: 'alice', receiver: 'bob', amount: 100 });
      expect(result.status).toBe('pending');

      const pending = await listTransferInstructions('alice');
      const transfer = pending.find(t => t.sender.includes('alice') && t.receiver.includes('bob'));
      expect(transfer).toBeDefined();

      const withdrawResult = await withdrawTransfer({ party: 'alice', contractId: transfer.contractId });
      expect(withdrawResult.status).toBe('withdrawed');
    });
  });

  test.describe('Reject transfer: bob (p2) → alice (p1) rejected by alice', () => {
    test('should reject a pending transfer', async () => {
      await ensureHolding('bob', 500);

      const result = await transferBond({ sender: 'bob', receiver: 'alice', amount: 100 });
      expect(result.status).toBe('pending');

      const pending = await listTransferInstructions('alice');
      const transfer = pending.find(t => t.sender.includes('bob') && t.receiver.includes('alice'));
      expect(transfer).toBeDefined();

      const rejectResult = await rejectTransfer({ party: 'alice', contractId: transfer.contractId });
      expect(rejectResult.status).toBe('rejected');
    });
  });

  test.describe('Same-participant: alice (p1) → executor (p1)', () => {
    test('should transfer from alice to executor and executor accepts', async () => {
      await ensureHolding('alice', 500);

      const result = await transferBond({ sender: 'alice', receiver: 'executor', amount: 100 });
      expect(result.status).toBe('pending');

      const pending = await listTransferInstructions('executor');
      const transfer = pending.find(t => t.sender.includes('alice') && t.receiver.includes('executor'));
      expect(transfer).toBeDefined();

      const acceptResult = await acceptTransfer({ party: 'executor', contractId: transfer.contractId });
      expect(acceptResult.status).toBe('accepted');
    });
  });

  test.describe('Frontend: initiate transfer via UI', () => {
    test('should transfer from alice to bob via the Transfer page', async ({ page }) => {
      await ensureHolding('alice', 500);

      await page.goto('/');
      const startBtn = page.locator('[data-testid="startFactoryBtn"]');
      await expect(startBtn).toBeVisible({ timeout: 15000 });
      const statusBadge = page.locator('[data-testid="statusBadge"]');
      const text = await statusBadge.textContent();
      if (text && !text.includes('Factory ready') && !text.includes('Connected')) {
        await startBtn.click();
        await expect(statusBadge).toContainText(/Factory ready|Connected/i, { timeout: 30000 });
      }

      await page.locator('[data-testid="nav-transfer"]').click();
      await expect(page.locator('[data-testid="page-transfer"]')).toBeVisible();

      await page.locator('[data-testid="transferSender"]').selectOption('alice');
      await page.locator('[data-testid="transferReceiver"]').selectOption('bob');
      await page.locator('[data-testid="transferAmount"]').fill('50');
      await page.locator('[data-testid="transferSubmit"]').click();

      const result = page.locator('[data-testid="transferResult"]');
      await expect(result).toContainText(/✅|Transfer initiated|pending/i, { timeout: 15000 });
    });
  });
});
