# Backend API — Endpoint Documentation

## Architecture Overview

```
Frontend (SPA)         Backend (Go)            Canton Ledger
┌────────────────┐     ┌────────────────┐     ┌─────────────────────┐
│                │     │                │     │                     │
│  app.js        │────▶│  handlers.go   │────▶│  JSON API V2        │
│  (Vanilla JS)  │     │  (REST)        │     │  (per participant)  │
│                │     │                │     │                     │
│  Browser       │     │  Port 8080     │     │  Ports 5013/5023/5033│
│                │     │                │     │                     │
└────────────────┘     └────────────────┘     └─────────────────────┘
                                                         │
                                                         ▼
                                                ┌─────────────────┐
                                                │  Canton Domain  │
                                                │  (Sequencer +   │
                                                │   Mediator)     │
                                                └─────────────────┘
```

The backend runs as a Go HTTP server that translates REST calls into Canton Ledger API requests. It manages connections to 3 Canton participants (each hosting different parties) and provides a unified REST API consumed by the frontend SPA.

**Transport modes**: By default, the backend uses gRPC for read operations (`GetLedgerEnd`, `GetActiveContracts`) and the HTTP JSON API v2 for writes (`submit-and-wait`) and party management. Set `LEDGER_TRANSPORT=http` to fall back to full HTTP JSON API v2 mode.

### Participant Layout

| Participant | Parties | HTTP Port | gRPC Port |
|-------------|---------|-----------|-----------|
| participant1 | admin, alice, executor | 5013 | 5011 |
| participant2 | bob | 5023 | 5021 |
| participant3 | charlie | 5033 | 5031 |

### Daml Contract Templates

| Template | File | Description |
|----------|------|-------------|
| `SimpleTokenRules` | `Rules.daml` | Factory — orchestrates minting, transfers, allocations. Signatory: `admin`. Observers: all parties. |
| `SimpleHolding` | `Holding.daml` | Unlocked holding (a bond). Signatory: `admin, owner`. |
| `LockedSimpleHolding` | `Holding.daml` | Locked holding (pending transfer). Signatory: `admin, owner, lock.holders`. |
| `SimpleTransferInstruction` | `TransferInstruction.daml` | Pending transfer with locked holding. Signatory: `admin, transfer.sender`. Observer: `transfer.receiver`. |
| `SimpleAllocation` | `Allocation.daml` | DvP allocation. Signatory: `admin, allocation.transferLeg.sender`. |

---

## `GET /api/v1/health`

**Purpose**: Liveness check.

**Frontend usage**: Called once on page load to verify the backend is reachable. Sets factory to locked state.

**Request**: None

**Response**:
```json
{ "status": "ok" }
```

**Canton/Daml interaction**: None — purely backend-internal.

---

## `GET /api/v1/factory`

**Purpose**: Check if the `SimpleTokenRules` factory exists on participant1.

**Frontend usage**: Not called directly; the equivalent check is done by `POST /api/v1/factory` during startup.

**Backend logic**:
1. Calls `StateService.GetLedgerEnd` (gRPC) or `GET /v2/state/ledger-end` (HTTP) on participant1
2. Calls `StateService.GetActiveContracts` (gRPC) or `POST /v2/state/active-contracts` (HTTP) with template `SimpleTokenRules`
3. If found → returns contract ID, template ID, admin, instruments
4. If NOT found → delegates to **POST** logic and creates the factory

**Response** (factory exists):
```json
{
  "contractId": "00be2e...",
  "templateId": "#simple-token:SimpleToken.Rules:SimpleTokenRules",
  "admin": "admin::1220...",
  "instruments": "[\"BOND\"]"
}
```

---

## `POST /api/v1/factory`

**Purpose**: Create the `SimpleTokenRules` factory contract on participant1.

**Frontend usage**: Called by the "Start Factory" button (`#startFactoryBtn`) on the dashboard. The button has retry logic with exponential backoff (up to 5 attempts, 2s/4s/8s/10s/10s delays).

**Backend logic**:
1. Calls `LedgerEnd` + `ActiveContracts` (gRPC reads) for `SimpleTokenRules`
2. If not found:
   a. Retries looking up the `admin` party (up to 10 attempts × 3s) — handles bootstrap race
   b. Queries ALL participants for their local party lists
   c. Collects all party IDs as `observers` (excluding admin)
   d. Calls `POST /v2/commands/submit-and-wait` on participant1 (HTTP JSON API):
      - **Command**: `CreateCommand` with template `SimpleTokenRules`
      - **CreateArguments**: `{admin, supportedInstruments: ["BOND"], observers: [...]}`
      - **actAs**: `[adminID]`
3. If found: returns existing factory data

**Daml contract affected**:
```
template SimpleTokenRules
  with
    admin : Party
    supportedInstruments : [Text]
    observers : [Party]          -- ← Makes factory visible on all participants
  where
    signatory admin
    observer observers            -- ← Stakeholders for cross-participant visibility
```

**Response**:
```json
{
  "status": "created",
  "offset": 57,
  "admin": "admin::1220...",
  "instruments": ["BOND"]
}
```

---

## `GET /api/v1/parties`

**Purpose**: List all local parties across all participants.

**Frontend usage**: Called on dashboard load, parties page, and to populate party select dropdowns across Mint, Transfer, Burn, and Pending pages. The frontend extracts short names (`party::hash` → `party`) via `shortName()` for display.

**Backend logic**:
1. For each configured participant (1/2/3): calls `GET /v2/parties`
2. Filters to `isLocal == true` parties only
3. Deduplicates by party ID
4. Maps each party to `{identifier, displayName, participant}`

**Canton API used**: `GET /v2/parties` — returns `{partyDetails: [{party, isLocal, displayName}]}`

**Response**:
```json
[
  { "identifier": "admin::1220...", "displayName": "", "participant": "participant1" },
  { "identifier": "bob::1220...",   "displayName": "", "participant": "participant2" },
  { "identifier": "charlie::1220...", "displayName": "", "participant": "participant3" }
]
```

---

## `POST /api/v1/parties`

**Purpose**: Allocate a new party on a specific participant.

**Frontend usage**: Part of the "Create Party" form. User selects a participant (dropdown) and enters a hint (text input). On success, the parties list is refreshed.

**Backend logic**:
1. Looks up the participant client by name (`participant1`, `participant2`, `participant3`)
2. Calls `POST /v2/parties` with `{partyIdHint: hint}` on the target participant
3. Parses the response (handles both array and object `partyDetails` formats)

**Canton API used**: `POST /v2/parties` — allocates a new party with an optional hint. Returns `{partyDetails: [{party, isLocal, displayName}]}`.

**Request**:
```json
{
  "participant": "participant2",
  "hint": "eve"
}
```

**Response**:
```json
{
  "identifier": "eve::1220...",
  "displayName": "",
  "participant": "participant2"
}
```

---

## `GET /api/v1/holdings?party={party}`

**Purpose**: List holdings (bonds) for a given party.

**Frontend usage**:
- Dashboard: called for every party to aggregate all holdings across participants. Each holding is annotated with `observedViaParticipants`.
- Holdings page: filtered by the selected party dropdown.
- Burn page: filtered by the party dropdown, used to select which holding to burn (`fillBurn(cid)` fills the contract ID input).

**Backend logic**:
1. Resolves the party to its participant via `clientForParty()` (checks configured mapping first, then falls back to dynamic lookup across all participants)
2. Resolves the short party name to the full identifier via `lookupPartyIdentifier()` (queries the participant's party list and matches by short name, full ID, or display name)
3. Calls `POST /v2/state/active-contracts` with a `WildcardFilter` (returns ALL contracts visible to the participant)
4. Filters for `SimpleHolding` and `LockedSimpleHolding` templates
5. Filters by `owner == partyID || admin == partyID`
6. Extracts fields: admin, owner, amount, couponRate, maturityDate, description, lock status, instrumentId (nested `{admin, id}` → flattened to `admin:id`)

**Canton API used**: `POST /v2/state/active-contracts` with `filtersForAnyParty` WildcardFilter (no party-specific filter — returns all contracts the participant can see).

**Response**:
```json
[
  {
    "contractId": "008726...",
    "templateId": "#simple-token:SimpleToken.Holding:SimpleHolding",
    "admin": "admin::1220...",
    "owner": "bob::1220...",
    "instrumentId": "admin::1220...:BOND",
    "amount": 100,
    "couponRate": 5,
    "maturityDate": "2099-12-31",
    "description": "",
    "locked": false
  }
]
```

---

## `POST /api/v1/mint`

**Purpose**: Mint new bonds (create a `SimpleHolding` via the factory's `Mint` choice).

**Frontend usage**: Mint page form. User selects admin party (dropdown), owner party (dropdown), enters amount, coupon rate, maturity date, and description. Validates that admin and owner are on the same participant (the backend enforces this — the frontend relies on backend error for validation).

**Backend logic**:
1. Resolves admin and owner to their participants
2. **Checks both parties are on the same participant** → returns 400 if not
3. Resolves short names to full identifiers
4. Calls `GET /v2/state/ledger-end` on the participant
5. Calls `POST /v2/state/active-contracts` to find the factory (template `SimpleTokenRules`)
6. Builds `Mint` choice argument with owner, instrumentId `{admin, "BOND"}`, amount, couponRate, maturityDate, description
7. Calls `POST /v2/commands/submit-and-wait`:
   - **Command**: `ExerciseCommand` on factory CID
   - **Template**: `SimpleTokenRules`
   - **Choice**: `Mint`
   - **actAs**: `[adminID, ownerID]` (both must authorize — controller is `admin, owner`)

**Daml contract affected**:
```
nonconsuming choice Mint : ContractId SimpleHolding
  with owner, amount, couponRate, maturityDate, description, instrumentId
  controller admin, owner          -- ← Both must be in actAs
  do
    assertMsg "amount > 0" (amount > 0.0)
    assertMsg "instrument supported" (instrumentId.id `elem` supportedInstruments)
    create SimpleHolding with admin, owner, ...
```

**Response**:
```json
{
  "status": "created",
  "offset": 60,
  "admin": "admin::1220...",
  "owner": "alice::1220...",
  "amount": 500,
  "coupon": 5,
  "maturity": "2027-12-31"
}
```

---

## `POST /api/v1/transfer`

**Purpose**: Initiate a transfer (exercises `TransferFactory_Transfer` on the factory).

**Frontend usage**: Transfer page form. User selects sender and receiver from party dropdowns, enters amount. On success, shows "pending" status and refreshes dashboard.

**Backend logic**:
1. Resolves sender and receiver to their participants and full identifiers
2. Queries the factory **from participant1** (where it was created) — always uses `factoryClient`
3. Queries the sender's unlocked holdings from **the sender's participant**
4. Auto-selects enough holdings to cover the amount (or uses provided `holdingCids`)
5. Validates sufficient holdings → returns 400 if not
6. Builds `TransferFactory_Transfer` choice argument with:
   - `expectedAdmin`: factory admin
   - `transfer`: sender, receiver, amount, instrumentId, requestedAt, executeBefore (+24h), inputHoldingCids, meta
   - `extraArgs`: context, meta
7. Calls `POST /v2/commands/submit-and-wait` on **the sender's participant**:
   - **Command**: `ExerciseCommand` with interface template `TransferFactory`
   - **Choice**: `TransferFactory_Transfer`
   - **actAs**: `[senderID]` (only sender — controller is `transfer.sender`)
   - The factory CID is passed directly (it's visible on all participants via the `observers` field)

**Key design decisions**:
- Factory is queried from participant1 (the create participant) but the command is submitted from the sender's participant (factory is visible there via observers)
- Only `senderID` is in actAs because `TransferFactory_Transfer` controller is `transfer.sender`
- Admin's authorization is implicit (admin is the signatory of the factory contract)

**Daml contract flow**:
```
TransferFactory_Transfer (on SimpleTokenRules)
    ├── archiveAndSumInputs sender instrumentId inputHoldingCids
    │     └── for each holding: fetch, validate owner/instrumentId/lock, archive
    ├── if sender == receiver → selfTransfer (merge)
    └── else → twoStepTransfer
              ├── create LockedSimpleHolding (signatory: admin, sender, lock.holders=[admin])
              │     └── extraObservers: [receiver]
              ├── create SimpleTransferInstruction (signatory: admin, sender)
              │     └── observer: receiver
              └── create change holding if totalInput > amount
```

**Response**:
```json
{
  "status": "pending",
  "offset": 63,
  "sender": "bob::1220...",
  "receiver": "alice::1220...",
  "amount": 50
}
```

---

## `POST /api/v1/transfer/accept`

**Purpose**: Accept a pending transfer (receiver exercises `TransferInstruction_Accept`).

**Frontend usage**: "Accept" button in the Pending Transfers table. The frontend passes `contractId` and the receiver's short name as `party`.

**Backend logic**:
1. Resolves the party short name to the full identifier on their participant
2. Calls `POST /v2/commands/submit-and-wait`:
   - **Command**: `ExerciseCommand` with interface template `TransferInstruction`
   - **Choice**: `TransferInstruction_Accept`
   - **actAs**: `[partyID]` (receiver — controller is `transfer.receiver`)

**Daml contract flow**:
```
TransferInstruction_Accept (on SimpleTransferInstruction)
    ├── fetch + archive lockedHoldingCid
    ├── create SimpleHolding (admin, owner=receiver, amount=transfer.amount)
    └── create change holding if lockedHolding.amount > transfer.amount
       (admin, owner=sender, amount=difference)
```

---

## `POST /api/v1/transfer/reject`

**Purpose**: Reject a pending transfer (receiver exercises `TransferInstruction_Reject`).

**Frontend usage**: "Reject" button in the Pending Transfers table. The frontend passes `contractId` and the receiver's short name as `party`.

**Backend logic**:
1. Same pattern as Accept but with `TransferInstruction_Reject` choice
2. `actAs`: `[partyID]` (receiver — controller is `transfer.receiver`)

**Daml contract flow**:
```
TransferInstruction_Reject (on SimpleTransferInstruction)
    └── returnLockedFundsToSender admin transfer lockedHoldingCid
          ├── archive lockedHoldingCid
          └── create SimpleHolding (admin, owner=sender, amount=lockedHolding.amount)
```

---

## `POST /api/v1/transfer/withdraw`

**Purpose**: Withdraw a pending transfer (sender exercises `TransferInstruction_Withdraw`).

**Frontend usage**: "Withdraw" button in the Pending Transfers table. The frontend passes `contractId` and the sender's short name as `party`.

**Backend logic**:
1. Same pattern as Accept/Reject but with `TransferInstruction_Withdraw` choice
2. `actAs`: `[partyID]` (sender — controller is `transfer.sender`)

**Daml contract flow**:
```
TransferInstruction_Withdraw (on SimpleTransferInstruction)
    └── returnLockedFundsToSender admin transfer lockedHoldingCid
          ├── archive lockedHoldingCid
          └── create SimpleHolding (admin, owner=sender, amount=lockedHolding.amount)
```

---

## `GET /api/v1/transfer-instructions?party={party}`

**Purpose**: List pending transfer instructions for a given party (as sender or receiver).

**Frontend usage**:
- Dashboard: called for every party to aggregate pending transfers for the stats counter and the dashboard view.
- Pending page: called for all parties, filtered by the dropdown. Displays Accept/Reject/Withdraw buttons per row.

**Backend logic**:
1. Resolves party to participant and full identifier
2. Calls `POST /v2/state/active-contracts` with template `SimpleTransferInstruction`
3. Extracts the nested `transfer` field from each event's createArguments
4. Filters to instructions where `sender == partyID || receiver == partyID`
5. Parses sender, receiver, and amount from the nested transfer object

**Response**:
```json
[
  {
    "contractId": "00817f...",
    "sender": "alice::1220...",
    "receiver": "bob::1220...",
    "amount": 100
  }
]
```

---

## `GET /api/v1/allocations?party={party}`

**Purpose**: List active DvP allocations for a given party (as admin, sender, receiver, or executor).

**Frontend usage**: Allocations page; aggregated across parties and filtered by dropdown.

**Backend logic**:
1. Resolves party to participant and full identifier
2. Calls `POST /v2/state/active-contracts` with template `SimpleAllocation`
3. Extracts nested `allocation.transferLeg` and `allocation.settlement`
4. Filters to allocations where `party` is admin, sender, receiver, or executor

**Response**:
```json
[
  {
    "contractId": "00817f...",
    "templateId": "#simple-token:SimpleToken.Allocation:SimpleAllocation",
    "admin": "admin::1220...",
    "sender": "alice::1220...",
    "receiver": "bob::1220...",
    "executor": "executor::1220...",
    "amount": 100,
    "instrumentId": "admin::1220...:BOND",
    "allocateBefore": "2028-12-31T00:00:00Z",
    "settleBefore": "2029-01-02T00:00:00Z",
    "lockedHoldingCid": "00be2e..."
  }
]
```

---

## `POST /api/v1/allocations`

**Purpose**: Create a DvP allocation (exercise `AllocationFactory_Allocate`).

**Backend logic**:
1. Resolves sender/receiver/executor to full identifiers
2. Finds `SimpleTokenRules` factory on participant1
3. Selects sender holdings to fund the allocation
4. Calls `AllocationFactory_Allocate` via the AllocationFactory interface

**Request**:
```json
{
  "sender": "alice",
  "receiver": "bob",
  "executor": "executor",
  "amount": 100,
  "allocateBefore": "2028-12-31T00:00:00Z",
  "settleBefore": "2029-01-02T00:00:00Z",
  "settlementRef": "dvp-2028-0001",
  "transferLegId": "leg-1"
}
```

**Response**:
```json
{
  "status": "created",
  "offset": 88,
  "sender": "alice::1220...",
  "receiver": "bob::1220...",
  "executor": "executor::1220...",
  "amount": 100
}
```

---

## `POST /api/v1/allocations/execute`

**Purpose**: Execute a funded allocation (exercise `Allocation_ExecuteTransfer`).

**Request**:
```json
{ "party": "executor", "contractId": "00be2e..." }
```

---

## `POST /api/v1/allocations/cancel`

**Purpose**: Cancel an allocation and return funds (exercise `Allocation_Cancel`).

**Request**:
```json
{ "party": "alice", "contractId": "00be2e..." }
```

---

## `POST /api/v1/allocations/withdraw`

**Purpose**: Withdraw an allocation before allocateBefore (exercise `Allocation_Withdraw`).

**Request**:
```json
{ "party": "alice", "contractId": "00be2e..." }
```

---

## `POST /api/v1/burn`

**Purpose**: Destroy a holding (exercises `Burn` or `BurnByAdmin` choice on `SimpleHolding`).

**Frontend usage**: Burn page. User selects a party, sees their holdings table, clicks "Select" on a holding to fill the contract ID, optionally checks "Burn as admin" if they have admin privileges, then submits.

**Backend logic**:
1. Resolves party to participant and full identifier
2. If `asAdmin: true`, uses `BurnByAdmin` choice (controller `admin`)
3. Otherwise uses `Burn` choice (controller `owner`)
4. Calls `POST /v2/commands/submit-and-wait`:
   - **Command**: `ExerciseCommand` on the holding contract CID
   - **Template**: `SimpleHolding`
   - **Choice**: `Burn` or `BurnByAdmin`
   - **actAs**: `[partyID]`

**Daml contract**:
```
nonconsuming choice Burn : ()
  controller owner
  do archive self

nonconsuming choice BurnByAdmin : ()
  controller admin
  do archive self
```

**Response**:
```json
{
  "status": "burned",
  "offset": 72
}
```

---

## `POST /api/v1/self-transfer`

**Purpose**: Merge/defragment holdings owned by the same party. Delegates to `handleTransfer` but enforces `sender == receiver`.

**Frontend usage**: Not exposed as a separate UI page; tested via the Playwright e2e suite.

**Backend logic**:
1. Validates `sender == receiver`
2. Calls `handleTransfer` (same flow as regular transfer)
3. The Daml `twoStepTransfer` detects `sender == receiver` and calls `selfTransfer` instead — which creates a single merged `SimpleHolding` with the total amount

**Daml contract flow**:
```
transferFactory_transferImpl
  └── if sender == receiver → selfTransfer
        └── create SimpleHolding (admin, owner=sender, amount=transfer.amount)
        └── create change holding if totalInput > transfer.amount
```

---

## Canton Ledger API — Low-Level Interaction Details

All Daml interactions go through the `cantonledger` package in `pkg/cantonledger/`, which implements the `LedgerClient` interface. Two implementations exist:

### gRPC mode (`LEDGER_TRANSPORT=grpc`, default)

Uses `pkg/cantonledger/grpc_client.go`:

| Operation | Transport | Endpoint |
|-----------|-----------|----------|
| `LedgerEnd` | gRPC (native) | `StateService.GetLedgerEnd` |
| `ActiveContracts` | gRPC (native) | `StateService.GetActiveContracts` (streaming) |
| `Parties` | HTTP JSON API | `GET /v2/parties` |
| `AllocateParty` | HTTP JSON API | `POST /v2/parties` |
| `SubmitCommand` | HTTP JSON API | `POST /v2/commands/submit-and-wait` |

### HTTP mode (`LEDGER_TRANSPORT=http`, fallback)

Uses `pkg/cantonledger/client.go`:

| Operation | Transport | Endpoint |
|-----------|-----------|----------|
| `LedgerEnd` | HTTP JSON API | `GET /v2/state/ledger-end` |
| `ActiveContracts` | HTTP JSON API | `POST /v2/state/active-contracts` |
| `Parties` | HTTP JSON API | `GET /v2/parties` |
| `AllocateParty` | HTTP JSON API | `POST /v2/parties` |
| `SubmitCommand` | HTTP JSON API | `POST /v2/commands/submit-and-wait` |

### Command Structure (HTTP JSON API v2)

All write commands use `submit-and-wait` with:
```json
{
  "commands": [
    {
      "CreateCommand": { "templateId": "...", "createArguments": {...} }
      // or
      "ExerciseCommand": { "templateId": "...", "choice": "...", "contractId": "...", "choiceArgument": {...} }
    }
  ],
  "userId": "ledger-api-user",
  "commandId": "mint-abc123",
  "actAs": ["party::id"],
  "readAs": ["party::id"]
}
```

### Template ID References

The interface template IDs are package hash-prefixed (e.g., `55ba4deb...:Splice.Api.Token.TransferInstructionV1:TransferFactory`), while concrete template IDs use the short form (`#simple-token:SimpleToken.Rules:SimpleTokenRules`). The JSON API V2 requires the correct template ID for exercise commands — interface choices use the interface template ID, direct template choices use the template ID.

### Client Routing

The backend maintains one `LedgerClient` per participant (configured via `PARTICIPANT{1,2,3}_URL` and `PARTICIPANT{1,2,3}_GRPC_URL` env vars). The `clientForParty()` function resolves a party to its host participant:
1. Check the static config mapping (e.g., `bob` → `participant2`)
2. Fallback: query all participants' party lists and match by short name, full ID, or display name

### Relevant Source Files

- `backend/api.go` — HTTP handlers, choice-building, actAs logic
- `backend/config.go` — Environment-based configuration
- `pkg/cantonledger/interface.go` — `LedgerClient` interface
- `pkg/cantonledger/grpc_client.go` — gRPC client (default transport)
- `pkg/cantonledger/client.go` — HTTP JSON API v2 client (fallback)
- `pkg/cantonledger/commands.go` — Command building + `submit-and-wait`
- `pkg/cantonledger/events.go` — CreatedEvent parsing from ActiveContracts
- `pkg/cantonledger/templates.go` — Template and choice ID constants
