# WebSocket Usage in the Bond Platform Frontend

## Architecture

```
Canton Ledger (participant1:5011)
    │ gRPC UpdateService.GetUpdates stream
    ▼
Listener (Go, port 8081) — bridges gRPC events → WebSocket JSON
    │ ws://hostname:8081/ws/bonds
    ▼
Frontend (Vite, port 3000) — `app.js` consumes events
```

## Frontend: `frontend/app.js` (lines 696–773)

### Connection
- **Type**: Raw `new WebSocket(url)` — no Socket.IO or library
- **URL**: `ws://${window.location.hostname}:8081/ws/bonds`
- **Established**: Once during app startup in `init()` (line 700)
- **Reconnection**: On `onclose`, retries after 5s via `setTimeout(initWebSocket, 5000)` — no exponential backoff

### Message Handling (`onmessage`)
Messages are JSON with `templateId` and `action` fields. Based on template:

| Template | Actions | Reloads |
|---|---|---|
| `SimpleHolding` / `LockedSimpleHolding` | `created`, `archived` | dashboard, holdings, burn (if archived) |
| `SimpleTransferInstruction` | any | dashboard, pending |
| `SimpleAllocation` | any | dashboard, allocations |
| `SimpleTokenRules` | any | dashboard |

- Reloads are conditional on `currentPage` — only refreshes the view if the user is on that page
- `loadDashboard()`, `loadHoldings()`, `loadPending()`, `loadAllocations()`, `loadBurnHoldings()` are re-called

### Error Handling
- `onclose`: logs disconnect, schedules reconnect
- `onerror`: logs error, no explicit recovery (reconnect is handled by `onclose`)
- No heartbeat/ping — missed disconnects rely on TCP timeout

## Backend Listener: `listener/main.go`

- **Go standard library** + `gorilla/websocket`
- **gRPC stream** from `participant1:5011` subscribes to **all** template events (wildcard filter)
- Each received gRPC event is broadcast via `conn.WriteJSON()` to **all connected WebSocket clients**
- Client disconnect detected via read loop (`conn.ReadMessage()` returns error)
- Broadcaster holds a `map[*websocket.Conn]bool` protected by a `sync.Mutex`

## Key Observations

1. **Unidirectional**: Events flow only from Ledger → Listener → Frontend. Frontend never sends data over WebSocket.
2. **No backpressure**: Listener broadcasts to all clients unconditionally. A slow client could block the broadcaster.
3. **Port 8081 direct**: WebSocket goes directly to the listener, bypassing the Vite dev proxy and the backend API.
4. **Single subscriber**: Listener only subscribes to participant1 events. Changes involving only parties on participant2/3 that are not visible to participant1 are not streamed.
5. **Every event triggers full reload**: Rather than applying incremental updates, each WebSocket message re-fetches all data via REST API, negating some of the real-time benefit.
