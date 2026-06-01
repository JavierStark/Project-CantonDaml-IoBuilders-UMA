import { test, expect } from '@playwright/test';
import { mintBond, listHoldings, transferBond, acceptTransfer, listTransferInstructions } from '../helpers/api.js';

test.describe('Self-Transfer (Merge Holdings)', () => {

  async function ensureHolding(party, amount = 1000) {
    const holdings = await listHoldings(party);
    const unlocked = holdings.filter(h => !h.locked);
    const total = unlocked.reduce((sum, h) => sum + h.amount, 0);
    if (total >= amount) return;
    // Mint to alice on p1, then transfer to target party if needed
    await mintBond({ admin: 'admin', owner: 'alice', amount, description: `Self-test` });
    if (party !== 'alice') {
      await transferBond({ sender: 'alice', receiver: party, amount });
      const pending = await listTransferInstructions(party);
      const transfer = pending.find(t => t.sender.includes('alice') && t.receiver.includes(party));
      if (transfer) {
        await acceptTransfer({ party, contractId: transfer.contractId });
      }
    }
  }

  test('should self-transfer (merge) alice holdings', async () => {
    await ensureHolding('alice', 1000);
    await mintBond({ admin: 'admin', owner: 'alice', amount: 200, description: 'Second Alice holding' });

    const before = await listHoldings('alice');
    const unlockedBefore = before.filter(h => !h.locked);
    expect(unlockedBefore.length).toBeGreaterThanOrEqual(2);

    const result = await transferBond({ sender: 'alice', receiver: 'alice', amount: 500 });
    expect(result.status).toBeDefined();
  });

  test('should self-transfer from charlie to charlie', async () => {
    await ensureHolding('charlie', 1000);
    // Transfer another bond from alice to charlie to create a second holding
    await mintBond({ admin: 'admin', owner: 'alice', amount: 300, description: 'Second Charlie holding' });
    await transferBond({ sender: 'alice', receiver: 'charlie', amount: 300 });
    const pending = await listTransferInstructions('charlie');
    const transfer = pending.find(t => t.sender.includes('alice') && t.receiver.includes('charlie'));
    if (transfer) {
      await acceptTransfer({ party: 'charlie', contractId: transfer.contractId });
    }

    const before = await listHoldings('charlie');
    const unlockedBefore = before.filter(h => !h.locked);
    expect(unlockedBefore.length).toBeGreaterThanOrEqual(2);

    const result = await transferBond({ sender: 'charlie', receiver: 'charlie', amount: 400 });
    expect(result.status).toBeDefined();
  });
});
