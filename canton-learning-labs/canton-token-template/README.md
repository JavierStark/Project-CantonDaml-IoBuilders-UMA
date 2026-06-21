# Bond Token (Daml)

A minimal bond token built on the CIP-056 token standard interface, with mint, transfer, and burn capabilities.

## Project Structure

```
simple-token/          Core Daml library (the bond token logic)
  daml/SimpleToken/
    Holding.daml             Bond holding templates
    Rules.daml               Factory contract with Mint, TransferFactory, AllocationFactory
    TransferInstruction.daml Two-step transfer flow
    Allocation.daml          DvP allocation/settlement
    ContextUtils.daml        Choice context helpers

simple-token-test/     Test suite
  daml/SimpleToken/
    Testing/SimpleRegistry.daml  Test harness
    Testing/WalletClient.daml    Wallet query helpers
    Test/Setup.daml              Common test environment
    Test/Bond.daml               Bond lifecycle tests
    Test/Transfer.daml           Transfer flow tests
    Test/Allocation.daml         Allocation flow tests
    Test/Negative.daml           Error/edge case tests

dars/                  Splice API DAR dependencies (token standard interfaces)
```

## Learning Roadmap

Read the source files in this order to understand the full system:

### Phase 1 — Data Model

| Order | File | What to focus on |
|-------|------|------------------|
| 1 | `Holding.daml` | `SimpleHolding` fields (especially `couponRate`, `maturityDate`, `description`), signatory model (`admin, owner`), `TransferOwnership`, `Burn`, `BurnByAdmin`. Then `LockedSimpleHolding` with its lock mechanism and `LockedSimpleHolding_Unlock`. |
| 2 | `ContextUtils.daml` | The expire-lock context key pattern used by reject/withdraw flows. Short file, read quickly. |

### Phase 2 — Operations

| Order | File | What to focus on |
|-------|------|------------------|
| 3 | `Rules.daml` | `SimpleTokenRules` template and its signatory. The `Mint` choice (controller `admin, owner`). Then the two interface instances: `TransferFactory` (auth model for interface choices, self-transfer vs two-step dispatch) and `AllocationFactory` (locked holding creation, sender change). Internal helpers `selfTransfer`, `twoStepTransfer`, `archiveAndSumInputs`. |
| 4 | `TransferInstruction.daml` | `SimpleTransferInstruction` signatory/observer. Interface choice `acceptImpl` (creates receiver holding with bond fields preserved), `rejectImpl`/`withdrawImpl` (both call `returnLockedFundsToSender`). The expire-lock pattern for owner-unlock-then-reject. |
| 5 | `Allocation.daml` | `SimpleAllocation` signatory/observer. Interface choices `executeTransferImpl` (atomic transfer), `cancelImpl`, `withdrawImpl`. How bond fields propagate from locked holdings. |

### Phase 3 — Tests

| Order | File | What to focus on |
|-------|------|------------------|
| 6 | `Testing/SimpleRegistry.daml` | Test harness architecture: `SimpleRegistry`, `EnrichedChoice`, `getTransferFactory`/`getAllocationFactory`, `createHolding`. Learn how tests submit transactions with disclosures and multi-party auth. |
| 7 | `Testing/WalletClient.daml` | `checkBalance`, `listHoldings`, `listTransferOffers`. Simple query utilities. |
| 8 | `Test/Setup.daml` | `setupTestEnv`, `fundParty`. Standard test environment setup. |
| 9 | `Test/Bond.daml` | The 9 bond-specific tests. Start here to see the full mint → transfer → burn lifecycle end-to-end, plus negative tests. |
| 10 | `Test/Transfer.daml` | Self-transfer (merge) and two-step transfer (pending → accept/reject/withdraw). |
| 11 | `Test/Allocation.daml` | Allocation execute, cancel, withdraw, and DvP with two legs. |
| 12 | `Test/Negative.daml` | The most thorough test file: 18 tests covering authorization, expiration, contention, invariants. Read the invariant comments in Rules.daml alongside these tests. |

## Key Patterns to Notice

- **Authorization**: Interface choices provide implicit signatory auth; template choices require explicit controller auth. Compare `Mint` (controller `admin, owner`) vs `TransferFactory_Transfer` (interface auth).
- **Two-step transfer**: Locked holding + transfer instruction. The lock prevents double-spend; the instruction enables receiver acceptance/rejection.
- **Bond field propagation**: `couponRate`, `maturityDate`, `description` flow from the original holding through locked holdings into final receiver holdings.
- **Expire-lock pattern**: If a lock expires, the owner can unlock. Subsequent reject/withdraw detect this via the `expireLockContextKey` to avoid double-returning funds.

## Build & Test

```sh
# Build the core library
cd simple-token && dpm build

# Build and run all tests
cd ../simple-token-test && dpm build && dpm test
```
