# Complete Tutorial: CIP-0056 Bond Token on Daml

## Table of Contents

1. [What Is This Project?](#1-what-is-this-project)
2. [CIP-0056: The Token Standard](#2-cip-0056-the-token-standard)
3. [Project Structure](#3-project-structure)
4. [Foundation: Splice API Metadata (MetadataV1)](#4-foundation-splice-api-metadata)
5. [The Holding Interface (HoldingV1)](#5-the-holding-interface)
6. [The Transfer Factory Interface (TransferInstructionV1)](#6-the-transfer-factory-interface)
7. [The Allocation & Settlement Interfaces](#7-the-allocation--settlement-interfaces)
8. [Our Implementation: SimpleToken.Holding](#8-our-implementation-simpletokenholding)
9. [Choice Context Utilities (ContextUtils)](#9-choice-context-utilities)
10. [The Factory Contract: SimpleTokenRules](#10-the-factory-contract-simpletokenrules)
11. [Two-Step Transfer: SimpleTransferInstruction](#11-two-step-transfer-simpletransferinstruction)
12. [DvP Settlement: SimpleAllocation](#12-dvp-settlement-simpleallocation)
13. [Test Harness: SimpleRegistry & WalletClient](#13-test-harness)
14. [Test Suite Walkthrough](#14-test-suite-walkthrough)
15. [Build & Run](#15-build--run)

---

## 1. What Is This Project?

This is a **zero-coupon bond token** built on Daml using the **CIP-0056 token standard** from the Splice network. It implements:

- **Minting** bonds with financial metadata (coupon rate, maturity date, ISIN-like identifier)
- **Self-transfers** (merge/fragment holdings)
- **Two-step transfers** (sender locks funds, receiver accepts)
- **DvP settlement** (Delivery vs Payment — atomic exchange of assets)
- **Ownership unlocking** of expired locks

The project separates into a **core library** (`simple-token/`) and a **test suite** (`simple-token-test/`).

---

## 2. CIP-0056: The Token Standard

CIP-0056 (Canton Improvement Proposal 56) defines a **fractional, fungible, UTXO-style token standard** for the Splice/Canton ecosystem. Think of it as "ERC-20 meets UTXO" for Daml.

### The UTXO Model

Unlike a monolithic balance (e.g., "Alice has 100 tokens" stored in one contract), CIP-0056 uses **multiple holdings**:

```
Alice's holdings:
  Holding #1: 60 tokens
  Holding #2: 40 tokens
  Total: 100 tokens
```

Each holding is a separate contract. This enables:
- **Parallel spending** of different holdings
- **Partial transfers** (spend 60 from a 100-token holding, get 40 change back)
- **Locking** individual holdings for atomic settlement

### Key Concepts

| Concept | Daml Type | Purpose |
|---------|-----------|---------|
| **Holding** | `interface Holding` | Represents ownership of a token quantity |
| **Instrument ID** | `InstrumentId` | Identifies the token type (admin + text ID) |
| **Lock** | `Lock` | Freezes a holding for pending operations |
| **Transfer** | `Transfer` | Describes who sends how much of what to whom |
| **Transfer Instruction** | `interface TransferInstruction` | A pending two-step transfer |
| **Allocation** | `interface Allocation` | A funded settlement leg for DvP |
| **Transfer Factory** | `interface TransferFactory` | Creates transfers atomically |
| **Allocation Factory** | `interface AllocationFactory` | Creates allocations atomically |

### The 6 Interface DARs

The project consumes 6 pre-built DARs from Splice:

```
dars/
  splice-api-token-metadata-v1-1.0.0.dar       # Types: AnyValue, Metadata, ExtraArgs, etc.
  splice-api-token-holding-v1-1.0.0.dar         # interface Holding, InstrumentId, Lock
  splice-api-token-transfer-instruction-v1-1.0.0.dar  # interface TransferInstruction + TransferFactory
  splice-api-token-allocation-v1-1.0.0.dar      # interface Allocation, SettlementInfo, TransferLeg
  splice-api-token-allocation-instruction-v1-1.0.0.dar  # interface AllocationFactory + AllocationInstruction
  splice-api-token-allocation-request-v1-1.0.0.dar      # interface AllocationRequest (not implemented here)
```

---

## 3. Project Structure

```
canton-token-template/
├── dars/                          # Pre-built Splice API DARs (CIP-0056 interfaces)
├── simple-token/                  # Core library — our implementation
│   ├── daml.yaml
│   └── daml/SimpleToken/
│       ├── Holding.daml           # SimpleHolding + LockedSimpleHolding
│       ├── Rules.daml             # SimpleTokenRules (factory contract)
│       ├── TransferInstruction.daml  # SimpleTransferInstruction
│       ├── Allocation.daml        # SimpleAllocation
│       └── ContextUtils.daml      # Choice context helpers
└── simple-token-test/             # Test suite
    ├── daml.yaml
    └── daml/SimpleToken/
        ├── Testing/
        │   ├── SimpleRegistry.daml   # Test harness
        │   └── WalletClient.daml     # Wallet query helpers
        └── Test/
            ├── Setup.daml            # Common test env
            ├── Bond.daml             # Bond lifecycle tests (9 tests)
            ├── Transfer.daml         # Transfer flow tests (7 tests)
            ├── Allocation.daml       # DvP tests (5 tests)
            └── Negative.daml         # Edge cases (18 tests)
```

---
# Complete Tutorial: CIP-0056 Bond Token on Daml

## Table of Contents

1. [What Is This Project?](#1-what-is-this-project)
2. [CIP-0056: The Token Standard](#2-cip-0056-the-token-standard)
3. [Project Structure](#3-project-structure)
4. [Foundation: Splice API Metadata (MetadataV1)](#4-foundation-splice-api-metadata)
5. [The Holding Interface (HoldingV1)](#5-the-holding-interface)
6. [Our Implementation: SimpleToken.Holding](#6-our-implementation-simpletokenholding)
7. [The Transfer Factory Interface (TransferInstructionV1)](#7-the-transfer-factory-interface)
8. [The Factory Contract: SimpleTokenRules](#8-the-factory-contract-simpletokenrules)
9. [Two-Step Transfer: SimpleTransferInstruction](#9-two-step-transfer-simpletransferinstruction)
10. [The Allocation & Settlement Interfaces](#10-the-allocation--settlement-interfaces)
11. [DvP Settlement: SimpleAllocation](#11-dvp-settlement-simpleallocation)
12. [Choice Context Utilities (ContextUtils)](#12-choice-context-utilities)
13. [Test Harness: SimpleRegistry & WalletClient](#13-test-harness)
14. [Test Suite Walkthrough](#14-test-suite-walkthrough)
15. [Build & Run](#15-build--run)

---

## 1. What Is This Project?

This is a **zero-coupon bond token** built on Daml using the **CIP-0056 token standard** from the Splice network. It implements:

- **Minting** bonds with financial metadata (coupon rate, maturity date, ISIN-like identifier)
- **Self-transfers** (merge/fragment holdings)
- **Two-step transfers** (sender locks funds, receiver accepts)
- **DvP settlement** (Delivery vs Payment — atomic exchange of assets)
- **Ownership unlocking** of expired locks

The project separates into a **core library** (`simple-token/`) and a **test suite** (`simple-token-test/`).

---

## 2. CIP-0056: The Token Standard

CIP-0056 (Canton Improvement Proposal 56) defines a **fractional, fungible, UTXO-style token standard** for the Splice/Canton ecosystem. Think of it as "ERC-20 meets UTXO" for Daml.

### The UTXO Model

Unlike a monolithic balance (e.g., "Alice has 100 tokens" stored in one contract), CIP-0056 uses **multiple holdings**:

```
Alice's holdings:
  Holding #1: 60 tokens
  Holding #2: 40 tokens
  Total: 100 tokens
```

Each holding is a separate contract. This enables:
- **Parallel spending** of different holdings
- **Partial transfers** (spend 60 from a 100-token holding, get 40 change back)
- **Locking** individual holdings for atomic settlement

### Key Concepts

| Concept | Daml Type | Purpose |
|---------|-----------|---------|
| **Holding** | `interface Holding` | Represents ownership of a token quantity |
| **Instrument ID** | `InstrumentId` | Identifies the token type (admin + text ID) |
| **Lock** | `Lock` | Freezes a holding for pending operations |
| **Transfer** | `Transfer` | Describes who sends how much of what to whom |
| **Transfer Instruction** | `interface TransferInstruction` | A pending two-step transfer |
| **Allocation** | `interface Allocation` | A funded settlement leg for DvP |
| **Transfer Factory** | `interface TransferFactory` | Creates transfers atomically |
| **Allocation Factory** | `interface AllocationFactory` | Creates allocations atomically |

### The 6 Interface DARs

The project consumes 6 pre-built DARs from Splice:

```
dars/
  splice-api-token-metadata-v1-1.0.0.dar       # Types: AnyValue, Metadata, ExtraArgs, etc.
  splice-api-token-holding-v1-1.0.0.dar         # interface Holding, InstrumentId, Lock
  splice-api-token-transfer-instruction-v1-1.0.0.dar  # interface TransferInstruction + TransferFactory
  splice-api-token-allocation-v1-1.0.0.dar      # interface Allocation, SettlementInfo, TransferLeg
  splice-api-token-allocation-instruction-v1-1.0.0.dar  # interface AllocationFactory + AllocationInstruction
  splice-api-token-allocation-request-v1-1.0.0.dar      # interface AllocationRequest (not implemented here)
```

---

## 3. Project Structure

```
canton-token-template/
├── dars/                          # Pre-built Splice API DARs (CIP-0056 interfaces)
├── simple-token/                  # Core library — our implementation
│   ├── daml.yaml
│   └── daml/SimpleToken/
│       ├── Holding.daml           # SimpleHolding + LockedSimpleHolding
│       ├── Rules.daml             # SimpleTokenRules (factory contract)
│       ├── TransferInstruction.daml  # SimpleTransferInstruction
│       ├── Allocation.daml        # SimpleAllocation
│       └── ContextUtils.daml      # Choice context helpers
└── simple-token-test/             # Test suite
    ├── daml.yaml
    └── daml/SimpleToken/
        ├── Testing/
        │   ├── SimpleRegistry.daml   # Test harness
        │   └── WalletClient.daml     # Wallet query helpers
        └── Test/
            ├── Setup.daml            # Common test env
            ├── Bond.daml             # Bond lifecycle tests (9 tests)
            ├── Transfer.daml         # Transfer flow tests (7 tests)
            ├── Allocation.daml       # DvP tests (5 tests)
            └── Negative.daml         # Edge cases (18 tests)
```

---

## 4. Foundation: Splice API Metadata

File: `Splice/Api/Token/MetadataV1.daml` (from the DAR)

This module provides **core types** that everything else depends on:

```haskell
-- A polymorphic value that can hold any primitive Daml type.
-- Used in choice contexts to pass arbitrary extra arguments.
data AnyValue =
    AV_Text Text
  | AV_Int Int
  | AV_Decimal Decimal
  | AV_Bool Bool
  | AV_Date Date
  | AV_Time Time
  | AV_RelTime RelTime
  | AV_Party Party
  | AV_ContractId AnyContractId  -- any contract ID
  | AV_List [AnyValue]
  | AV_Map (TextMap.TextMap AnyValue)

-- A key-value map passed to choices as extra context
data ChoiceContext = ChoiceContext with
    values : TextMap AnyValue

emptyChoiceContext = ChoiceContext TextMap.empty

-- Simple string-string metadata (e.g., tx-kind labels)
data Metadata = Metadata with
    values : TextMap Text

-- Extra arguments passed to every interface choice
data ExtraArgs = ExtraArgs with
    context : ChoiceContext
    meta : Metadata

-- A phantom interface for "any contract ID" type erasure
interface AnyContract where viewtype AnyContractView
data AnyContractView = AnyContractView {}
type AnyContractId = ContractId AnyContract
```

**Why this matters**: All interface choices in CIP-0056 accept `ExtraArgs`, which lets callers attach arbitrary `context` data (e.g., "this is an expire-lock operation") without changing the interface signature.

---

## 5. The Holding Interface

File: `Splice/Api/Token/HoldingV1.daml`

This defines the **core ownership primitive**:

```haskell
-- Identifies a token type (like an ERC-20 contract address)
data InstrumentId = InstrumentId with
    admin : Party     -- who controls this token type
    id : Text         -- e.g., "SimpleToken", "USDC", "BTC"

-- A lock that can freeze a holding
data Lock = Lock with
    holders : [Party]         -- who can release the lock
    expiresAt : Optional Time  -- absolute expiry
    expiresAfter : Optional RelTime  -- relative expiry
    context : Optional Text    -- why the lock exists

-- The interface that all holdings must implement
interface Holding where
    viewtype HoldingView

-- What you see when you "view" a holding
data HoldingView = HoldingView with
    owner : Party
    instrumentId : InstrumentId
    amount : Decimal
    lock : Optional Lock     -- None = unlocked, Some = locked
    meta : Metadata
```

**Key insight**: `Holding` is an **interface**, not a template. Any template can implement `interface Holding` by providing `view = HoldingView{..}`. This is the polymorphism mechanism: the CIP-0056 standard defines what operations exist, and each token project provides its own templates that implement them.

---

## 6. Our Implementation: SimpleToken.Holding

File: `simple-token/daml/SimpleToken/Holding.daml`

Now we move from interfaces to **concrete templates** that implement them.

### SimpleHolding (Unlocked)

```haskell
template SimpleHolding
  with
    admin : Party
    owner : Party
    instrumentId : InstrumentId
    amount : Decimal
    couponRate : Decimal        -- bond-specific: annual interest rate
    maturityDate : Date         -- bond-specific: when principal is due
    description : Text          -- bond-specific: e.g., "Corporate Bond Series A"
    meta : Metadata
  where
    signatory admin, owner      -- BOTH must sign to create
    ensure amount > 0.0         -- invariant: no zero holdings

    -- Direct ownership transfer (avoids two-step for simple cases)
    choice TransferOwnership : ContractId SimpleHolding
      with newOwner : Party
      controller owner, newOwner   -- both must agree
      do create this with owner = newOwner

    -- Owner can destroy (redeem) the bond
    nonconsuming choice Burn : ()
      controller owner
      do archive self

    -- Admin can force-destroy (regulatory action)
    nonconsuming choice BurnByAdmin : ()
      controller admin
      do archive self

    -- Implement the CIP-0056 Holding interface
    interface instance Holding for SimpleHolding where
      view = HoldingView with
        owner
        instrumentId
        amount
        lock = None              -- unlocked
        meta
```

**Key design decisions:**
- `signatory admin, owner` — both must agree to create the holding. This prevents minting holdings without the owner's consent.
- `ensure amount > 0.0` — the `ensure` clause is checked on create and exercise. This is defense-in-depth: even if a factory contract has a bug, you can't create zero-amount holdings directly.
- Bond-specific fields (`couponRate`, `maturityDate`, `description`) are **extra fields** on the template beyond what `HoldingView` requires. They propagate through the system.
- `TransferOwnership` is a **template choice** (not interface choice). It requires `owner, newOwner` as joint controllers — both parties must sign.

### LockedSimpleHolding

```haskell
template LockedSimpleHolding
  with
    admin : Party
    owner : Party
    instrumentId : InstrumentId
    amount : Decimal
    couponRate : Decimal
    maturityDate : Date
    description : Text
    lock : Lock               -- the lock that binds this holding
    extraObservers : [Party]  -- receiver/executor can see this contract
    meta : Metadata
  where
    signatory admin, owner, lock.holders  -- lock holders must also sign
    observer extraObservers               -- extra parties can see it
    ensure amount > 0.0

    -- After lock expires, owner can recover funds
    choice LockedSimpleHolding_Unlock : ContractId SimpleHolding
      controller owner
      do
        now <- getTime
        case lock.expiresAt of
          None -> fail "Lock has no expiry and cannot be owner-expired"
          Some t -> assertMsg "Lock has not yet expired" (now >= t)
        create SimpleHolding with admin; owner; instrumentId; amount; couponRate; maturityDate; description; meta

    interface instance Holding for LockedSimpleHolding where
      view = HoldingView with
        owner
        instrumentId
        amount
        lock = Some lock
        meta
```

**The expiring lock pattern:**
1. When a transfer is initiated, the sender's holdings are archived and a `LockedSimpleHolding` is created.
2. The lock has `expiresAt = executeBefore` — the transfer deadline.
3. If the deadline passes, the **owner** can call `LockedSimpleHolding_Unlock` to recover their funds.
4. The `LockedSimpleHolding_Unlock` choice ONLY works after expiry — preventing premature fund recovery.
5. After the owner unlocks, subsequent reject/withdraw must handle the fact that the locked holding is already gone (see `expireLockContextKey` in TransferInstruction).

---

## 7. The Transfer Factory Interface

File: `Splice/Api/Token/TransferInstructionV1.daml`

This defines **how transfers happen** — two interfaces in one module:

### The Transfer Data Structure

```haskell
-- Describes a transfer request
data Transfer = Transfer with
    sender : Party
    receiver : Party
    amount : Decimal
    instrumentId : InstrumentId
    requestedAt : Time          -- when the transfer was requested
    executeBefore : Time        -- deadline — after this, transfer fails
    inputHoldingCids : [ContractId Holding]  -- which holdings to spend
    meta : Metadata
```

### The Transfer Instruction Interface (Two-Step)

```haskell
interface TransferInstruction where
    viewtype TransferInstructionView

    -- These 4 methods MUST be implemented by any template claiming
    -- to be a TransferInstruction:
    transferInstruction_acceptImpl   : ContractId TransferInstruction -> TransferInstruction_Accept   -> Update TransferInstructionResult
    transferInstruction_rejectImpl   : ContractId TransferInstruction -> TransferInstruction_Reject   -> Update TransferInstructionResult
    transferInstruction_withdrawImpl : ContractId TransferInstruction -> TransferInstruction_Withdraw -> Update TransferInstructionResult
    transferInstruction_updateImpl   : ContractId TransferInstruction -> TransferInstruction_Update   -> Update TransferInstructionResult

    -- The standard choices (with built-in authorization):
    choice TransferInstruction_Accept : TransferInstructionResult
        controller (view this).transfer.receiver  -- only receiver can accept
        do transferInstruction_acceptImpl this self arg

    choice TransferInstruction_Reject : TransferInstructionResult
        controller (view this).transfer.receiver  -- only receiver can reject
        do transferInstruction_rejectImpl this self arg

    choice TransferInstruction_Withdraw : TransferInstructionResult
        controller (view this).transfer.sender    -- only sender can withdraw
        do transferInstruction_withdrawImpl this self arg

    choice TransferInstruction_Update : TransferInstructionResult
        controller (view this).transfer.instrumentId.admin, extraActors
        do transferInstruction_updateImpl this self arg
```

The pattern: the **interface defines the choices with authorization rules**, then delegates to **implementation methods** (`*Impl`) that templates provide. A template just needs to write `interface instance TransferInstruction for MyTemplate where ...` and implement the 4 methods.

### The Transfer Factory Interface (Atomic Creation)

```haskell
interface TransferFactory where
    viewtype TransferFactoryView

    transferFactory_transferImpl     : ContractId TransferFactory -> TransferFactory_Transfer -> Update TransferInstructionResult
    transferFactory_publicFetchImpl  : ContractId TransferFactory -> TransferFactory_PublicFetch -> Update TransferFactoryView

    nonconsuming choice TransferFactory_Transfer : TransferInstructionResult
        controller transfer.sender  -- sender authorizes
        do transferFactory_transferImpl this self arg

    nonconsuming choice TransferFactory_PublicFetch : TransferFactoryView
        controller actor  -- anyone can query
        do transferFactory_publicFetchImpl this self arg
```

The Transfer Factory is a "router" contract: it takes a `Transfer` request, validates invariants, archives inputs, and dispatches to either a **self-transfer** (sender == receiver, just merge holdings) or a **two-step transfer** (creates a locked holding + `TransferInstruction`).

### Result Types

```haskell
data TransferInstructionResult = TransferInstructionResult with
    output : TransferInstructionResult_Output
    senderChangeCids : [ContractId Holding]  -- change back to sender
    meta : Metadata

data TransferInstructionResult_Output
    = TransferInstructionResult_Pending
        with transferInstructionCid : ContractId TransferInstruction  -- awaits accept/reject
    | TransferInstructionResult_Completed
        with receiverHoldingCids : [ContractId Holding]  -- done!
    | TransferInstructionResult_Failed  -- rejected/withdrawn
```

---

## 8. The Factory Contract: SimpleTokenRules

File: `simple-token/daml/SimpleToken/Rules.daml`

This is the **heart of the system** — the factory contract that:

1. Mints new holdings (`Mint` choice)
2. Creates transfers (`TransferFactory` interface)
3. Creates allocations (`AllocationFactory` interface)
4. Dispatches between self-transfer and two-step transfer

### The Template

```haskell
template SimpleTokenRules
  with
    admin : Party
    supportedInstruments : [Text]    -- multi-instrument support
  where
    signatory admin                  -- only admin controls the factory
```

### Minting

```haskell
    nonconsuming choice Mint
      : ContractId SimpleHolding
      with
        owner : Party
        amount : Decimal
        couponRate : Decimal
        maturityDate : Date
        description : Text
        instrumentId : InstrumentId
      controller admin, owner       -- both must agree
      do
        assertMsg "Mint amount must be positive" (amount > 0.0)
        assertMsg "Instrument not supported" (instrumentId.id `elem` supportedInstruments)
        create SimpleHolding with
          admin; owner; instrumentId; amount; couponRate; maturityDate; description; meta = emptyMetadata
```

Note: the `Mint` choice requires **both** `admin` and `owner` as controllers. This means:
- The admin cannot force-mint holdings to someone.
- The owner cannot mint holdings without the admin's authorization.

This is the bond issuance ceremony.

### TransferFactory Implementation

```haskell
    interface instance TransferFactory for SimpleTokenRules where
      view = TransferFactoryView with admin; meta = emptyMetadata

      transferFactory_transferImpl _selfCid arg = do
        let transfer = arg.transfer

        -- INVARIANTS (from CIP-0056 specification):
        assertMsg "expectedAdmin matches factory admin" (arg.expectedAdmin == admin)
        assertMsg "Transfer amount must be positive"    (transfer.amount > 0.0)
        now <- getTime
        assertMsg "requestedAt must not be in the future" (transfer.requestedAt <= now)
        assertMsg "executeBefore must be in the future"   (transfer.executeBefore > now)
        assertMsg "instrumentId.admin matches factory admin" (transfer.instrumentId.admin == admin)
        assertMsg "Instrument is supported" (transfer.instrumentId.id `elem` supportedInstruments)
        assertMsg "inputHoldingCids must not be empty" (not $ null transfer.inputHoldingCids)

        -- Archive ALL input holdings, sum their amounts, validate each
        totalInput <- archiveAndSumInputs transfer.sender transfer.instrumentId transfer.inputHoldingCids

        -- Sufficient funds?
        assertMsg ("Insufficient funds: have " <> show totalInput <> " but need " <> show transfer.amount)
          (totalInput >= transfer.amount)

        -- 2-way dispatch:
        if transfer.sender == transfer.receiver
          then selfTransfer admin transfer totalInput arg.extraArgs.meta
          else twoStepTransfer admin transfer totalInput
```

**The 2-way dispatch** is a key design decision:
- **Self-transfer** (sender == receiver): merge multiple holdings into one, with optional change. No locking needed — it's the same owner.
- **Two-step transfer** (sender != receiver): create a locked holding + transfer instruction. The receiver must explicitly accept.

### Self-Transfer Logic

```haskell
selfTransfer : Party -> Transfer -> Decimal -> Metadata -> Update TransferInstructionResult
selfTransfer admin transfer totalInput meta = do
  -- Create single merged holding
  receiverCid <- create SimpleHolding with
    admin; owner = transfer.sender; instrumentId = transfer.instrumentId
    amount = transfer.amount; meta
  -- Return change (if any) to sender
  senderChangeCids <- if totalInput > transfer.amount then ...
  pure TransferInstructionResult with
    output = TransferInstructionResult_Completed with
      receiverHoldingCids = [toInterfaceContractId @Holding receiverCid]
    senderChangeCids
    meta = txKindMeta "merge-split"
```

### Two-Step Transfer Logic

```haskell
twoStepTransfer : Party -> Transfer -> Decimal -> Update TransferInstructionResult
twoStepTransfer admin transfer totalInput = do
  -- 1. Create locked holding (receiver is extra observer)
  lockedCid <- create LockedSimpleHolding with
    admin; owner = transfer.sender; instrumentId = transfer.instrumentId
    amount = transfer.amount; lock = Lock with
      holders = [admin]
      expiresAt = Some transfer.executeBefore  -- lock expires at deadline
      context = Some "pending-transfer"
    extraObservers = [transfer.receiver]  -- receiver can see it
  -- 2. Create transfer instruction
  instrCid <- create SimpleTransferInstruction with
    admin; transfer; lockedHoldingCid; originalInstructionCid = None
  -- 3. Return change
  senderChangeCids <- if totalInput > transfer.amount then ...
  pure TransferInstructionResult with
    output = TransferInstructionResult_Pending with
      transferInstructionCid = toInterfaceContractId @TransferInstruction instrCid
    senderChangeCids
    meta = txKindMeta "transfer"
```

### AllocationFactory Implementation

```haskell
    interface instance AllocationFactory for SimpleTokenRules where
      allocationFactory_allocateImpl _selfCid arg = do
        let leg = arg.allocation.transferLeg
            settlement = arg.allocation.settlement
        -- ... same invariants as transfer ...

        -- Archive inputs, get total
        totalInput <- archiveAndSumInputs leg.sender leg.instrumentId arg.inputHoldingCids
        assertMsg "Sufficient funds" (totalInput >= leg.amount)

        -- Create locked holding for allocation
        lockedCid <- create LockedSimpleHolding with
          lock = Lock with
            holders = [admin]
            expiresAt = Some settlement.settleBefore
            context = Some "allocation"
          extraObservers = [settlement.executor, leg.receiver]

        -- Create allocation (direct-to-completed — no pending state)
        allocationCid <- create SimpleAllocation with
          admin; allocation = arg.allocation; lockedHoldingCid = lockedCid

        -- Return change
        senderChangeCids <- if totalInput > leg.amount then ...

        pure AllocationInstructionResult with
          output = AllocationInstructionResult_Completed with
            allocationCid = toInterfaceContractId @Allocation allocationCid
          senderChangeCids
```

### Internal Helper: archiveAndSumInputs

```haskell
archiveAndSumInputs : Party -> InstrumentId -> [ContractId Holding] -> Update Decimal
archiveAndSumInputs sender expectedInstrumentId holdingCids = do
  amounts <- forA holdingCids $ \holdingCid -> do
    holdingI <- fetch holdingCid
    let hv = view holdingI
    -- Invariant: input owner == sender
    assertMsg "Input holding owner does not match sender" (hv.owner == sender)
    -- Invariant: input instrumentId matches transfer
    assertMsg "Input holding instrumentId does not match" (hv.instrumentId == expectedInstrumentId)
    -- Lock check: unexpired locks REJECTED, expired locks ACCEPTED
    case hv.lock of
      None -> pure ()  -- unlocked: fine
      Some lock -> do
        now <- getTime
        let lockExpired = case lock.expiresAt of
              None -> False
              Some t -> now >= t
        assertMsg "Cannot use holding with unexpired lock" lockExpired
    archive holdingCid                     -- <-- FIRST mutation: archive all inputs
    pure hv.amount
  pure (sum amounts)
```

**Critical design point**: inputs are archived **before** creating outputs. This is the "contention guarantee" — if two transfers try to use the same holding, only the first one succeeds because the second will fail when trying to archive an already-archived contract.

---

## 9. Two-Step Transfer: SimpleTransferInstruction

File: `simple-token/daml/SimpleToken/TransferInstruction.daml`

This contract represents a **pending transfer** awaiting receiver action:

```haskell
template SimpleTransferInstruction
  with
    admin : Party
    transfer : Transfer
    lockedHoldingCid : ContractId LockedSimpleHolding
    originalInstructionCid : Optional (ContractId TransferInstruction)  -- for updates
  where
    signatory admin, transfer.sender     -- sender must agree to create
    observer transfer.receiver           -- receiver can see it
```

### Accept

```haskell
transferInstruction_acceptImpl selfCid arg = do
  now <- getTime
  assertMsg "Transfer has expired" (now < transfer.executeBefore)  -- deadline check
  
  lockedHolding <- fetch lockedHoldingCid
  archive lockedHoldingCid
  
  -- Create receiver holding, preserving bond fields
  receiverCid <- create SimpleHolding with
    admin; owner = transfer.receiver; instrumentId = transfer.instrumentId
    amount = transfer.amount
    couponRate = lockedHolding.couponRate        -- bond field preserved
    maturityDate = lockedHolding.maturityDate     -- bond field preserved
    description = lockedHolding.description       -- bond field preserved
  -- Return any excess (if locked amount > transfer amount)
  senderChangeCids <- if lockedHolding.amount > transfer.amount then ...
  pure TransferInstructionResult with
    output = TransferInstructionResult_Completed with
      receiverHoldingCids = [toInterfaceContractId @Holding receiverCid]
    senderChangeCids
```

### Reject / Withdraw

```haskell
transferInstruction_rejectImpl _selfCid arg =
  returnLockedFundsToSender admin transfer lockedHoldingCid arg.extraArgs

transferInstruction_withdrawImpl _selfCid arg =
  returnLockedFundsToSender admin transfer lockedHoldingCid arg.extraArgs
```

Both delegate to a shared helper that handles the **expire-lock pattern**:

```haskell
returnLockedFundsToSender : Party -> Transfer -> ContractId LockedSimpleHolding 
                           -> ExtraArgs -> Update TransferInstructionResult
returnLockedFundsToSender admin transfer lockedHoldingCid extraArgs = do
  -- Check if the locked holding was already unlocked by owner
  let lockedHoldingActive = case lookupFromContext @Bool extraArgs.context expireLockContextKey of
        Right (Some False) -> False     -- expired, owner already unlocked
        _ -> True                        -- still active

  if lockedHoldingActive
    then do
      -- Normal case: archive locked holding, return funds to sender
      lockedHolding <- fetch lockedHoldingCid
      archive lockedHoldingCid
      returnCid <- create SimpleHolding with ...
      pure TransferInstructionResult with
        output = TransferInstructionResult_Failed
        senderChangeCids = [toInterfaceContractId @Holding returnCid]
    else do
      -- Owner already unlocked. Just verify deadline passed and return empty.
      now <- getTime
      assertMsg "Locked holding not active but transfer has not expired"
        (now >= transfer.executeBefore)
      pure TransferInstructionResult with
        output = TransferInstructionResult_Failed
        senderChangeCids = []
```

**The expire-lock pattern in detail:**

1. Normal flow: reject/withdraw → archive locked holding → create unlocked holding for sender.
2. Edge case: The lock expires → owner calls `LockedSimpleHolding_Unlock` → locked holding is archived, sender gets their funds back.
3. Now the transfer instruction still exists, but its `lockedHoldingCid` is stale (already archived).
4. When reject/withdraw is called on the instruction, `fetch lockedHoldingCid` would fail.
5. Solution: the caller passes `expireLockContextKey = False` in the choice context, signaling "the locked holding is already gone, don't try to archive it."
6. The helper skips the archival and just returns `Failed` with no change (funds already returned via Unlock).

---

## 10. The Allocation & Settlement Interfaces

File: `Splice/Api/Token/AllocationV1.daml` and `Splice/Api/Token/AllocationInstructionV1.daml`

These enable **Delivery vs Payment (DvP)** — atomic exchange of two assets:

### The Allocation Interface

```haskell
-- A leg of a settlement (one asset moving one direction)
data TransferLeg = TransferLeg with
    sender : Party
    receiver : Party
    amount : Decimal
    instrumentId : InstrumentId
    meta : Metadata

-- Timing constraints for settlement
data SettlementInfo = SettlementInfo with
    executor : Party       -- who coordinates the settlement
    settlementRef : Reference
    requestedAt : Time
    allocateBefore : Time  -- deadline to fund the allocation
    settleBefore : Time    -- deadline to execute transfer

-- Full allocation specification (one leg of a multi-leg DvP)
data AllocationSpecification = AllocationSpecification with
    settlement : SettlementInfo       -- shared timing across legs
    transferLegId : Text              -- "leg-1", "leg-2", etc.
    transferLeg : TransferLeg         -- the actual transfer

interface Allocation where
    viewtype AllocationView

    allocation_executeTransferImpl  : ContractId Allocation -> Allocation_ExecuteTransfer -> Update Allocation_ExecuteTransferResult
    allocation_cancelImpl           : ContractId Allocation -> Allocation_Cancel          -> Update Allocation_CancelResult
    allocation_withdrawImpl         : ContractId Allocation -> Allocation_Withdraw        -> Update Allocation_WithdrawResult

    choice Allocation_ExecuteTransfer : Allocation_ExecuteTransferResult
        controller allocationControllers (view this)  -- executor + sender + receiver
        do allocation_executeTransferImpl this self arg

    choice Allocation_Cancel : Allocation_CancelResult
        controller allocationControllers (view this)
        do allocation_cancelImpl this self arg

    choice Allocation_Withdraw : Allocation_WithdrawResult
        controller (view this).allocation.transferLeg.sender  -- only sender
        do allocation_withdrawImpl this self arg
```

### The Allocation Factory Interface

```haskell
interface AllocationFactory where
    viewtype AllocationFactoryView

    allocationFactory_allocateImpl     : ContractId AllocationFactory -> AllocationFactory_Allocate -> Update AllocationInstructionResult
    allocationFactory_publicFetchImpl  : ContractId AllocationFactory -> AllocationFactory_PublicFetch -> Update AllocationFactoryView

    nonconsuming choice AllocationFactory_Allocate : AllocationInstructionResult
        controller allocation.transferLeg.sender
        do allocationFactory_allocateImpl this self arg
```

The **Allocation Factory** is like the Transfer Factory but for DvP: it archives inputs, creates a locked holding, and returns a **completed allocation** (direct to `AllocationInstructionResult_Completed`). No pending state — the allocation is funded immediately.

---

## 11. DvP Settlement: SimpleAllocation

File: `simple-token/daml/SimpleToken/Allocation.daml`

### The Template

```haskell
template SimpleAllocation
  with
    admin : Party
    allocation : AllocationSpecification
    lockedHoldingCid : ContractId LockedSimpleHolding
  where
    signatory admin, allocation.transferLeg.sender
    observer allocation.settlement.executor, allocation.transferLeg.receiver
```

The allocation is "funded" — it holds a locked `LockedSimpleHolding` that will be released to the receiver upon execution.

### Execute Transfer

```haskell
allocation_executeTransferImpl _selfCid arg = do
  now <- getTime
  assertMsg "Settlement has expired" (now < allocation.settlement.settleBefore)
  
  lockedHolding <- fetch lockedHoldingCid
  archive lockedHoldingCid
  
  let leg = allocation.transferLeg
  -- Create receiver holding with bond fields
  receiverCid <- create SimpleHolding with
    admin; owner = leg.receiver; instrumentId = leg.instrumentId
    amount = leg.amount
    couponRate = lockedHolding.couponRate     -- bond fields preserved
    maturityDate = lockedHolding.maturityDate
    description = lockedHolding.description
  -- Return change
  senderChangeCids <- if totalInput > leg.amount then ...
  pure Allocation_ExecuteTransferResult with
    senderHoldingCids = senderChangeCids
    receiverHoldingCids = [toInterfaceContractId @Holding receiverCid]
```

### Cancel

Same expire-lock pattern as reject/withdraw in TransferInstruction. The `releaseAllocatedFunds` helper works identically.

### Withdraw

```haskell
allocation_withdrawImpl _selfCid _arg = do
  now <- getTime
  assertMsg "Cannot withdraw after allocateBefore deadline"
    (now < allocation.settlement.allocateBefore)  -- must be before deadline
  lockedHolding <- fetch lockedHoldingCid
  archive lockedHoldingCid
  returnCid <- create SimpleHolding with ...
  pure Allocation_WithdrawResult with
    senderHoldingCids = [toInterfaceContractId @Holding returnCid]
```

Withdraw can only happen **before** `allocateBefore`, while execute/cancel happen **before** `settleBefore`. This timeline:

```
requestedAt → allocateBefore → [withdraw allowed here] → settleBefore → [execute/cancel here]
               ↑ withdraw stops   ↑ execute/cancel stops   ↑ deadline passes
```

---

## 12. Choice Context Utilities

File: `simple-token/daml/SimpleToken/ContextUtils.daml`

This module provides **serialization helpers** for passing typed data through `ChoiceContext`:

```haskell
-- Context key constants
transferPreapprovalContextKey : Text  = "transfer-preapproval"
expireLockContextKey : Text           = "expire-lock"
txKindMetaKey : Text                  = "splice.lfdecentralizedtrust.org/tx-kind"

-- Create tx-kind metadata (Splice convention)
txKindMeta : Text -> Metadata
txKindMeta kind = Metadata with values = TextMap.fromList [(txKindMetaKey, kind)]

-- Typeclass for encoding values into AnyValue
class ToAnyValue a where
  toAnyValue : a -> AnyValue

-- Typeclass for decoding values from AnyValue
class FromAnyValue a where
  fromAnyValue : AnyValue -> Either Text a

-- Lookup and decode a value from a choice context
lookupFromContext : FromAnyValue a => ChoiceContext -> Text -> Either Text (Optional a)

-- Convenience versions that fail in Update context
lookupFromContextU : FromAnyValue a => ChoiceContext -> Text -> Update (Optional a)
getFromContextU     : FromAnyValue a => ChoiceContext -> Text -> Update a
```

Instances are provided for `Text`, `Int`, `Decimal`, `Bool`, `Date`, `Time`, `Party`, `ContractId`, lists, and `ChoiceContext` itself.

**Why this is needed**: The CIP-0056 `ExtraArgs.context` field is a `TextMap AnyValue`. To pass specific typed data through it (e.g., "this is an expire-lock operation"), we need to encode/decode between Daml types and `AnyValue`.

---

## 13. Test Harness

### SimpleRegistry (Test Harness)

File: `simple-token-test/daml/SimpleToken/Testing/SimpleRegistry.daml`

This simulates a simple off-ledger registry for tests:

```haskell
data SimpleRegistry = SimpleRegistry with
    admin : Party
    instrumentId : InstrumentId

-- Initialize: create admin, instrument ID, and the factory contract
initialize : Text -> Text -> Script SimpleRegistry
initialize adminName instrumentName = do
  admin <- allocatePartyByHint (PartyIdHint adminName)
  let instrumentId = InstrumentId with admin; id = instrumentName
  _ <- submit admin $ createCmd SimpleTokenRules with
    admin
    supportedInstruments = [instrumentName]
  pure SimpleRegistry with admin; instrumentId
```

### EnrichedChoice Pattern

```haskell
data EnrichedChoice t ch = EnrichedChoice with
    factoryCid : ContractId t
    arg : ch
    disclosures : Disclosures

-- Helper: query the factory, get its disclosure, bundle it all together
getTransferFactory : SimpleRegistry -> Transfer -> ExtraArgs 
                   -> Script (EnrichedChoice TransferFactory TransferFactory_Transfer)
getTransferFactory registry transfer extraArgs = do
  [(rulesCid, _)] <- query @SimpleTokenRules registry.admin
  let tfCid = toInterfaceContractId @TransferFactory rulesCid
  rulesDisc <- queryDisc @SimpleTokenRules registry.admin rulesCid
  pure EnrichedChoice with
    factoryCid = tfCid
    arg = TransferFactory_Transfer with expectedAdmin = registry.admin; transfer; extraArgs
    disclosures = rulesDisc
```

Usage in tests:
```haskell
enriched <- getTransferFactory registry transfer emptyExtraArgs
result <- submitWithDisc alice enriched.disclosures $
  exerciseCmd enriched.factoryCid enriched.arg
```

This pattern handles the Splice disclosure mechanism: since the factory contract is not visible to Alice (Alice is not the admin), we need to **disclose** the contract to Alice before she can exercise a choice on it.

### WalletClient (Query Helpers)

File: `simple-token-test/daml/SimpleToken/Testing/WalletClient.daml`

```haskell
-- List all holdings for a party filtered by instrument
listHoldings : Party -> InstrumentId -> Script [(ContractId Holding, HoldingView)]
listHoldings p instrumentId = do
  holdings <- queryInterface @Holding p
  let filtered = do
    (cid, Some hv) <- holdings
    guard (hv.instrumentId == instrumentId)
    guard (hv.owner == p)
    pure (cid, hv)
  pure filtered

-- Assert exact balance
checkBalance : Party -> InstrumentId -> Decimal -> Script ()
checkBalance p instrumentId expected = do
  holdings <- listHoldings p instrumentId
  let total = sum $ map (._2.amount) holdings
  unless (total == expected) $ fail $
    show p <> ": balance " <> show total <> " /= expected " <> show expected

-- List pending transfer offers for a party
listTransferOffers : Party -> InstrumentId -> Script [(ContractId TransferInstruction, TransferInstructionView)]
listTransferOffers p instrumentId = do
  instrs <- queryInterface @TransferInstruction p
  ...
```

---

## 14. Test Suite Walkthrough

### Setup (`Test/Setup.daml`)

```haskell
data TestParties = TestParties with
    admin : Party
    alice : Party
    bob : Party
    charlie : Party
    executor : Party

data TestEnv = TestEnv with
    registry : SimpleRegistry
    parties : TestParties

setupTestEnv : Script TestEnv
setupTestEnv = do
  setTime demoTime  -- freeze time at 2024-01-01 00:00:00
  registry <- initialize "admin" "SimpleToken"
  alice <- allocatePartyByHint (PartyIdHint "alice")
  bob <- allocatePartyByHint (PartyIdHint "bob")
  -- ...
  pure TestEnv with registry; parties
```

### Bond Tests (`Test/Bond.daml`) — 9 tests

| # | Test | What It Verifies |
|---|------|------------------|
| 1 | `test_mint` | Admin + owner can mint a bond holding |
| 2 | `test_mintNonAdminFails` | Non-admin cannot mint |
| 3 | `test_mintZeroAmountFails` | Zero-amount mint is rejected |
| 4 | `test_transfer` | Owner can transfer ownership |
| 5 | `test_nonOwnerCannotTransfer` | Non-owner cannot transfer |
| 6 | `test_burn` | Owner can burn (redeem) their bond |
| 7 | `test_burnByAdmin` | Admin can burn any bond |
| 8 | `test_nonOwnerCannotBurn` | Non-owner cannot burn |
| 9 | `test_mintTransferBurn` | Full lifecycle end-to-end |

Example — full lifecycle:
```haskell
test_mintTransferBurn : Script ()
test_mintTransferBurn = do
  TestEnv{..} <- setupTestEnv
  [(rulesCid, _)] <- query @SimpleTokenRules registry.admin
  
  -- Mint: admin + alice create a 5000-token bond
  bondCid <- submitMulti [admin, alice] [] $ exerciseCmd rulesCid Mint with
    owner = alice; amount = 5000.0; instrumentId = registry.instrumentId
    couponRate = 0.03; maturityDate = date 2028 Jun 15; description = "Government Bond"
  checkBalance alice registry.instrumentId 5000.0
  
  -- Transfer: alice → bob (both must sign)
  bondCid' <- submitMulti [alice, bob] [] $ exerciseCmd bondCid TransferOwnership with
    newOwner = bob
  checkBalance bob registry.instrumentId 5000.0
  checkBalance alice registry.instrumentId 0.0
  
  -- Burn: bob redeems
  submit bob $ exerciseCmd bondCid' Burn
  checkBalance bob registry.instrumentId 0.0
```

### Transfer Tests (`Test/Transfer.daml`) — 7 tests

| # | Test | What It Verifies |
|---|------|------------------|
| 1 | `test_selfTransfer` | Merge 2 holdings via self-transfer |
| 2 | `test_selfTransferExactAmount` | Self-transfer with no change |
| 3 | `test_twoStepTransferPending` | Sender → receiver creates pending instruction |
| 4 | `test_twoStepTransferAccept` | Receiver accepts, gets tokens |
| 5 | `test_twoStepTransferReject` | Receiver rejects, funds return |
| 6 | `test_twoStepTransferWithdraw` | Sender withdraws, funds return |
| 7 | `test_publicFetch` | Non-admin can query factory via disclosure |

Example — two-step accept:
```haskell
test_twoStepTransferAccept = do
  TestEnv{..} <- setupTestEnv
  [h1] <- fundParty registry alice 100.0
  
  -- Step 1: Alice initiates transfer of 70 to Bob
  let transfer = mkTransfer registry alice bob 70.0 [h1]
  enriched <- getTransferFactory registry transfer emptyExtraArgs
  result <- submitWithDisc alice enriched.disclosures $
    exerciseCmd enriched.factoryCid enriched.arg
  
  case result.output of
    TransferInstructionResult_Pending instrCid -> do
      -- Step 2: Bob accepts
      acceptResult <- submit bob $ exerciseCmd instrCid TransferInstruction_Accept with
        extraArgs = emptyExtraArgs
      case acceptResult.output of
        TransferInstructionResult_Completed receiverCids ->
          assert (length receiverCids == 1)
      checkBalance bob registry.instrumentId 70.0
```

### Allocation Tests (`Test/Allocation.daml`) — 5 tests

| # | Test | What It Verifies |
|---|------|------------------|
| 1 | `test_allocate` | Create allocation (direct to completed) |
| 2 | `test_allocationExecuteTransfer` | Execute transfer → receiver gets tokens |
| 3 | `test_allocationCancel` | Cancel → funds return to sender |
| 4 | `test_allocationWithdraw` | Sender withdraws before deadline |
| 5 | `test_dvpTwoLegs` | Atomic DvP: Alice sends 80 to Bob, Bob sends 30 to Alice |

Example — DvP with two legs:
```haskell
test_dvpTwoLegs = do
  TestEnv{..} <- setupTestEnv
  [hAlice] <- fundParty registry alice 100.0
  [hBob]   <- fundParty registry bob 50.0
  
  -- Leg 1: Alice sends 80 to Bob (funded with same settlement group)
  let args1 = mkAllocationArgs registry alice bob exec 80.0 [hAlice]
  -- Leg 2: Bob sends 30 to Alice (shared settlement)
  let args2 = ...  -- same settlement as args1
  
  result1 <- submitWithDisc alice enriched1.disclosures $ ...
  result2 <- submitWithDisc bob   enriched2.disclosures $ ...
  
  -- Execute both legs atomically
  _ <- submitMulti [exec, alice, bob] [] $ exerciseCmd alloc1 Allocation_ExecuteTransfer ...
  _ <- submitMulti [exec, alice, bob] [] $ exerciseCmd alloc2 Allocation_ExecuteTransfer ...
  
  -- Verify final balances
  checkBalance alice registry.instrumentId 50.0    -- 100 - 80 + 30
  checkBalance bob   registry.instrumentId 100.0   -- 50 - 30 + 80
```

### Negative Tests (`Test/Negative.daml`) — 18 tests

These test **error cases and invariants**:

| # | Test | What It Verifies |
|---|------|------------------|
| 1 | `test_wrongExpectedAdmin` | Wrong `expectedAdmin` is rejected |
| 2 | `test_futureRequestedAt` | Future `requestedAt` rejected |
| 3 | `test_expiredExecuteBefore` | Past `executeBefore` rejected |
| 4 | `test_nonPositiveAmount` | Zero/negative amount rejected |
| 5 | `test_wrongInstrumentId` | Wrong `instrumentId` rejected |
| 6 | `test_emptyInputHoldings` | Empty inputs rejected |
| 7 | `test_holdingContention` | Double-spend same holding fails |
| 8 | `test_unauthorizedAccept` | Charlie can't accept Bob's transfer |
| 9 | `test_expiredTransferAccept` | Bob can't accept after deadline |
| 10 | `test_unexpiredLockedHoldingInput` | Unexpired locked holding rejected as input |
| 11 | `test_expiredLockedHoldingInput` | Expired locked holding ACCEPTED as input |
| 12 | `test_zeroAmountHolding` | Zero-amount SimpleHolding can't be created (`ensure`) |
| 13 | `test_negativeAmountHolding` | Negative amount rejected |
| 14 | `test_zeroAmountLockedHolding` | Zero-amount LockedSimpleHolding rejected |
| 15 | `test_ownerUnlockExpiredLock` | Owner can unlock after expiry |
| 16 | `test_ownerUnlockUnexpiredLock` | Owner CANNOT unlock before expiry |
| 17 | `test_rejectAfterOwnerUnlock` | Reject succeeds after owner unlock (expire-lock pattern) |
| 18 | `test_withdrawAfterOwnerUnlock` | Withdraw succeeds after owner unlock |

---

## 15. Build & Run

```sh
# 1. Build the core library
cd simple-token
dpm build

# 2. Build and run all tests
cd ../simple-token-test
dpm build
dpm test
```

Expected test output:
```
All tests passed (39 tests)
```

## Key Patterns Summary

1. **Interfaces for Polymorphism**: CIP-0056 defines interfaces (`Holding`, `TransferInstruction`, `Allocation`, `TransferFactory`, `AllocationFactory`). Templates implement them via `interface instance`. This allows any token project to be interoperable.

2. **UTXO-style Accounting**: Each holding is a separate contract. Spend multiple holdings to fund a transfer, get change back. This enables parallel operations and partial transfers.

3. **First-Mutation Contention Guarantee**: Input holdings are archived BEFORE output holdings are created. If two transfers use the same input, only the first succeeds.

4. **Locking for Atomicity**: Locked holdings enable multi-step operations (two-step transfer, DvP) by freezing assets until the operation completes or expires.

5. **Expire-Lock Pattern**: If a lock expires, the owner can recover funds via `LockedSimpleHolding_Unlock`. Subsequent reject/withdraw detect this via the `expireLockContextKey` to avoid double-returning.

6. **Factory Pattern**: A single `SimpleTokenRules` contract manages all operations for one or more instruments. It validates invariants, archives inputs, and dispatches to the correct path.

7. **Disclosure Mechanism**: In Splice/Canton, contracts are only visible to signatories and observers. To let a non-signatory exercise a choice, the contract must be disclosed to them. The `EnrichedChoice` pattern bundles the factory CID, the choice arguments, and the required disclosure together.

```haskell
-- List all holdings for a party filtered by instrument
listHoldings : Party -> InstrumentId -> Script [(ContractId Holding, HoldingView)]
listHoldings p instrumentId = do
  holdings <- queryInterface @Holding p
  let filtered = do
        (cid, Some hv) <- holdings
        guard (hv.instrumentId == instrumentId)
        guard (hv.owner == p)
        pure (cid, hv)
  pure filtered

-- Assert exact balance
checkBalance : Party -> InstrumentId -> Decimal -> Script ()
checkBalance p instrumentId expected = do
  holdings <- listHoldings p instrumentId
  let total = sum $ map (._2.amount) holdings
  unless (total == expected) $ fail $
    show p <> ": balance " <> show total <> " /= expected " <> show expected

-- List pending transfer offers for a party
listTransferOffers : Party -> InstrumentId -> Script [(ContractId TransferInstruction, TransferInstructionView)]
listTransferOffers p instrumentId = do
  instrs <- queryInterface @TransferInstruction p
  ...
```

---

## 14. Test Suite Walkthrough

### Setup (`Test/Setup.daml`)

```haskell
data TestParties = TestParties with
    admin : Party
    alice : Party
    bob : Party
    charlie : Party
    executor : Party

data TestEnv = TestEnv with
    registry : SimpleRegistry
    parties : TestParties

setupTestEnv : Script TestEnv
setupTestEnv = do
  setTime demoTime  -- freeze time at 2024-01-01 00:00:00
  registry <- initialize "admin" "SimpleToken"
  alice <- allocatePartyByHint (PartyIdHint "alice")
  bob <- allocatePartyByHint (PartyIdHint "bob")
  -- ...
  pure TestEnv with registry; parties
```

### Bond Tests (`Test/Bond.daml`) — 9 tests

| # | Test | What It Verifies |
|---|------|------------------|
| 1 | `test_mint` | Admin + owner can mint a bond holding |
| 2 | `test_mintNonAdminFails` | Non-admin cannot mint |
| 3 | `test_mintZeroAmountFails` | Zero-amount mint is rejected |
| 4 | `test_transfer` | Owner can transfer ownership |
| 5 | `test_nonOwnerCannotTransfer` | Non-owner cannot transfer |
| 6 | `test_burn` | Owner can burn (redeem) their bond |
| 7 | `test_burnByAdmin` | Admin can burn any bond |
| 8 | `test_nonOwnerCannotBurn` | Non-owner cannot burn |
| 9 | `test_mintTransferBurn` | Full lifecycle end-to-end |

Example — full lifecycle:
```haskell
test_mintTransferBurn : Script ()
test_mintTransferBurn = do
  TestEnv{..} <- setupTestEnv
  [(rulesCid, _)] <- query @SimpleTokenRules registry.admin
  
  -- Mint: admin + alice create a 5000-token bond
  bondCid <- submitMulti [admin, alice] [] $ exerciseCmd rulesCid Mint with
    owner = alice; amount = 5000.0; instrumentId = registry.instrumentId
    couponRate = 0.03; maturityDate = date 2028 Jun 15; description = "Government Bond"
  checkBalance alice registry.instrumentId 5000.0
  
  -- Transfer: alice → bob (both must sign)
  bondCid' <- submitMulti [alice, bob] [] $ exerciseCmd bondCid TransferOwnership with
    newOwner = bob
  checkBalance bob registry.instrumentId 5000.0
  checkBalance alice registry.instrumentId 0.0
  
  -- Burn: bob redeems
  submit bob $ exerciseCmd bondCid' Burn
  checkBalance bob registry.instrumentId 0.0
```

### Transfer Tests (`Test/Transfer.daml`) — 7 tests

| # | Test | What It Verifies |
|---|------|------------------|
| 1 | `test_selfTransfer` | Merge 2 holdings via self-transfer |
| 2 | `test_selfTransferExactAmount` | Self-transfer with no change |
| 3 | `test_twoStepTransferPending` | Sender → receiver creates pending instruction |
| 4 | `test_twoStepTransferAccept` | Receiver accepts, gets tokens |
| 5 | `test_twoStepTransferReject` | Receiver rejects, funds return |
| 6 | `test_twoStepTransferWithdraw` | Sender withdraws, funds return |
| 7 | `test_publicFetch` | Non-admin can query factory via disclosure |

Example — two-step accept:
```haskell
test_twoStepTransferAccept = do
  TestEnv{..} <- setupTestEnv
  [h1] <- fundParty registry alice 100.0
  
  -- Step 1: Alice initiates transfer of 70 to Bob
  let transfer = mkTransfer registry alice bob 70.0 [h1]
  enriched <- getTransferFactory registry transfer emptyExtraArgs
  result <- submitWithDisc alice enriched.disclosures $
    exerciseCmd enriched.factoryCid enriched.arg
  
  case result.output of
    TransferInstructionResult_Pending instrCid -> do
      -- Step 2: Bob accepts
      acceptResult <- submit bob $ exerciseCmd instrCid TransferInstruction_Accept with
        extraArgs = emptyExtraArgs
      case acceptResult.output of
        TransferInstructionResult_Completed receiverCids ->
          assert (length receiverCids == 1)
      checkBalance bob registry.instrumentId 70.0
```

### Allocation Tests (`Test/Allocation.daml`) — 5 tests

| # | Test | What It Verifies |
|---|------|------------------|
| 1 | `test_allocate` | Create allocation (direct to completed) |
| 2 | `test_allocationExecuteTransfer` | Execute transfer → receiver gets tokens |
| 3 | `test_allocationCancel` | Cancel → funds return to sender |
| 4 | `test_allocationWithdraw` | Sender withdraws before deadline |
| 5 | `test_dvpTwoLegs` | Atomic DvP: Alice sends 80 to Bob, Bob sends 30 to Alice |

Example — DvP with two legs:
```haskell
test_dvpTwoLegs = do
  TestEnv{..} <- setupTestEnv
  [hAlice] <- fundParty registry alice 100.0
  [hBob]   <- fundParty registry bob 50.0
  
  -- Leg 1: Alice sends 80 to Bob (funded with same settlement group)
  let args1 = mkAllocationArgs registry alice bob exec 80.0 [hAlice]
  -- Leg 2: Bob sends 30 to Alice (shared settlement)
  let args2 = ...  -- same settlement as args1
  
  result1 <- submitWithDisc alice enriched1.disclosures $ ...
  result2 <- submitWithDisc bob   enriched2.disclosures $ ...
  
  -- Execute both legs atomically
  _ <- submitMulti [exec, alice, bob] [] $ exerciseCmd alloc1 Allocation_ExecuteTransfer ...
  _ <- submitMulti [exec, alice, bob] [] $ exerciseCmd alloc2 Allocation_ExecuteTransfer ...
  
  -- Verify final balances
  checkBalance alice registry.instrumentId 50.0    -- 100 - 80 + 30
  checkBalance bob   registry.instrumentId 100.0   -- 50 - 30 + 80
```

### Negative Tests (`Test/Negative.daml`) — 18 tests

These test **error cases and invariants**:

| # | Test | What It Verifies |
|---|------|------------------|
| 1 | `test_wrongExpectedAdmin` | Wrong `expectedAdmin` is rejected |
| 2 | `test_futureRequestedAt` | Future `requestedAt` rejected |
| 3 | `test_expiredExecuteBefore` | Past `executeBefore` rejected |
| 4 | `test_nonPositiveAmount` | Zero/negative amount rejected |
| 5 | `test_wrongInstrumentId` | Wrong `instrumentId` rejected |
| 6 | `test_emptyInputHoldings` | Empty inputs rejected |
| 7 | `test_holdingContention` | Double-spend same holding fails |
| 8 | `test_unauthorizedAccept` | Charlie can't accept Bob's transfer |
| 9 | `test_expiredTransferAccept` | Bob can't accept after deadline |
| 10 | `test_unexpiredLockedHoldingInput` | Unexpired locked holding rejected as input |
| 11 | `test_expiredLockedHoldingInput` | Expired locked holding ACCEPTED as input |
| 12 | `test_zeroAmountHolding` | Zero-amount SimpleHolding can't be created (`ensure`) |
| 13 | `test_negativeAmountHolding` | Negative amount rejected |
| 14 | `test_zeroAmountLockedHolding` | Zero-amount LockedSimpleHolding rejected |
| 15 | `test_ownerUnlockExpiredLock` | Owner can unlock after expiry |
| 16 | `test_ownerUnlockUnexpiredLock` | Owner CANNOT unlock before expiry |
| 17 | `test_rejectAfterOwnerUnlock` | Reject succeeds after owner unlock (expire-lock pattern) |
| 18 | `test_withdrawAfterOwnerUnlock` | Withdraw succeeds after owner unlock |

---

## 15. Build & Run

```sh
# 1. Build the core library
cd simple-token
dpm build

# 2. Build and run all tests
cd ../simple-token-test
dpm build
dpm test
```

Expected test output:
```
All tests passed (39 tests)
```

## Key Patterns Summary

1. **Interfaces for Polymorphism**: CIP-0056 defines interfaces (`Holding`, `TransferInstruction`, `Allocation`, `TransferFactory`, `AllocationFactory`). Templates implement them via `interface instance`. This allows any token project to be interoperable.

2. **UTXO-style Accounting**: Each holding is a separate contract. Spend multiple holdings to fund a transfer, get change back. This enables parallel operations and partial transfers.

3. **First-Mutation Contention Guarantee**: Input holdings are archived BEFORE output holdings are created. If two transfers use the same input, only the first succeeds.

4. **Locking for Atomicity**: Locked holdings enable multi-step operations (two-step transfer, DvP) by freezing assets until the operation completes or expires.

5. **Expire-Lock Pattern**: If a lock expires, the owner can recover funds via `LockedSimpleHolding_Unlock`. Subsequent reject/withdraw detect this via the `expireLockContextKey` to avoid double-returning.

6. **Factory Pattern**: A single `SimpleTokenRules` contract manages all operations for one or more instruments. It validates invariants, archives inputs, and dispatches to the correct path.

7. **Disclosure Mechanism**: In Splice/Canton, contracts are only visible to signatories and observers. To let a non-signatory exercise a choice, the contract must be disclosed to them. The `EnrichedChoice` pattern bundles the factory CID, the choice arguments, and the required disclosure together.
