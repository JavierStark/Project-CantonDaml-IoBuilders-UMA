const API = '/api/v1';
let parties = [];
let holdings = [];
let pendingTransfers = [];
let selectedParty = null;
let factoryReady = false;
let currentPage = 'dashboard';

function $(id) { return document.getElementById(id); }

async function api(path, options = {}) {
    const url = `${API}${path}`;
    const config = {
        headers: { 'Content-Type': 'application/json' },
        ...options,
    };
    if (config.body && typeof config.body === 'object') {
        config.body = JSON.stringify(config.body);
    }
    const res = await fetch(url, config);
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
    return data;
}

function showResult(el, msg, isError = false) {
    el.className = 'result ' + (isError ? 'error' : 'success');
    el.textContent = isError ? '❌ ' + msg : '✅ ' + msg;
    el.style.display = 'block';
}

function hideResult(el) {
    el.style.display = 'none';
    el.className = 'result';
}

function requireFactoryReady(resultEl) {
    if (!factoryReady) {
        showResult(resultEl, "Factory is not ready yet.", true);
        return false;
    }
    return true;
}

function toISOStringFromLocal(localDateTime) {
    if (!localDateTime) return '';
    const date = new Date(localDateTime);
    return date.toISOString();
}

function setFactoryLocked(locked, message) {
    factoryReady = !locked;
    document.body.classList.toggle('factory-locked', locked);
    updateStatus(locked ? 'locked' : 'ready', message);
}

// Navigation
document.querySelectorAll('nav a').forEach(a => {
    a.addEventListener('click', e => {
        e.preventDefault();
        if (!factoryReady) return;
        document.querySelectorAll('nav a').forEach(x => x.classList.remove('active'));
        a.classList.add('active');
        document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
        const page = $('page-' + a.dataset.page);
        if (page) page.classList.add('active');
        currentPage = a.dataset.page;
        loadPageData(a.dataset.page);
    });
});

async function loadPageData(page) {
    if (!factoryReady) return;
    switch (page) {
        case 'dashboard': loadDashboard(); break;
        case 'holdings': loadHoldings(); break;
        case 'pending': loadPending(); break;
        case 'allocations':
            if (!parties.length) {
                await loadParties();
            } else {
                populatePartySelects();
            }
            await loadAllocations();
            break;
        case 'burn': loadBurnHoldings(); break;
        case 'mint': populatePartySelects(); break;
        case 'transfer': populatePartySelects(); break;
        case 'parties': loadParties(); break;
    }
}

async function loadDashboard() {
    try {
        const [h, pts] = await Promise.all([
            api('/holdings?party=admin'),
            api('/parties'),
        ]);
        parties = pts;
        const pends = await loadPendingData(parties);
        pendingTransfers = pends;

        const byContract = new Map();
        for (const row of h) {
            byContract.set(row.contractId, {
                ...row,
                observedViaParticipants: ['participant1'],
            });
        }

        for (const p of uniqueParties(parties)) {
            const token = partyToken(p);
            if (token !== 'admin') {
                try {
                    const more = await api(`/holdings?party=${token}`);
                    for (const row of more) {
                        const existing = byContract.get(row.contractId);
                        if (!existing) {
                            byContract.set(row.contractId, {
                                ...row,
                                observedViaParticipants: [p.participant],
                            });
                            continue;
                        }
                        if (!existing.observedViaParticipants.includes(p.participant)) {
                            existing.observedViaParticipants.push(p.participant);
                        }
                    }
                } catch (_) { }
            }
        }
        holdings = Array.from(byContract.values());

        const totalValue = holdings.reduce((sum, h) => sum + (h.locked ? 0 : h.amount), 0);
        $('statTotalBonds').textContent = holdings.filter(h => !h.locked).length;
        $('statTotalValue').textContent = totalValue.toLocaleString();
        $('statParties').textContent = parties.length;
        $('statPending').textContent = pendingTransfers.length;

        renderHoldingsTable('dashboardHoldings', holdings, false, true);
        updateStatus('connected');
    } catch (err) {
        $('dashboardHoldings').innerHTML = `<p class="error">Failed: ${err.message}</p>`;
        updateStatus('error', err.message);
    }
}

async function loadHoldings() {
    try {
        const filter = $('holdingsFilter').value;
        const byContract = new Map();
        for (const p of uniqueParties(parties)) {
            const token = partyToken(p);
            if (filter && token !== filter) continue;
            try {
                const h = await api(`/holdings?party=${token}`);
                for (const row of h) {
                    const existing = byContract.get(row.contractId);
                    if (!existing) {
                        byContract.set(row.contractId, {
                            ...row,
                            observedViaParticipants: [p.participant],
                        });
                        continue;
                    }
                    if (!existing.observedViaParticipants.includes(p.participant)) {
                        existing.observedViaParticipants.push(p.participant);
                    }
                }
            } catch (_) { }
        }
        holdings = Array.from(byContract.values());
        renderHoldingsTable('holdingsList', holdings, false, true);
    } catch (err) {
        $('holdingsList').innerHTML = `<p class="error">${err.message}</p>`;
    }
}

async function loadPending() {
    try {
        const data = await loadPendingData();
        pendingTransfers = data;
        renderPendingTable(data);
    } catch (err) {
        $('pendingList').innerHTML = `<p class="error">${err.message}</p>`;
    }
}

async function loadAllocations() {
    try {
        const filter = $('allocationsFilter').value;
        const byContract = new Map();
        for (const p of uniqueParties(parties)) {
            const token = partyToken(p);
            if (!token) continue;
            if (filter && token !== filter) continue;
            try {
                const list = await api(`/allocations?party=${token}`);
                for (const row of list) {
                    if (!byContract.has(row.contractId)) {
                        byContract.set(row.contractId, row);
                    }
                }
            } catch (err) {
                console.warn(`Failed to load allocations for ${token}:`, err);
            }
        }
        const values = Array.from(byContract.values());
        values.sort((a, b) => (a.allocateBefore || '').localeCompare(b.allocateBefore || ''));
        renderAllocationsTable(values);
    } catch (err) {
        $('allocationsList').innerHTML = `<p class="error">${err.message}</p>`;
    }
}

async function loadPendingData(partyList = parties) {
    const byContract = new Map();
    for (const p of uniqueParties(partyList)) {
        const token = partyToken(p);
        if (!token) continue;
        try {
            const t = await api(`/transfer-instructions?party=${token}`);
            for (const row of t) {
                if (!byContract.has(row.contractId)) {
                    byContract.set(row.contractId, row);
                }
            }
        } catch (err) {
            console.warn(`Failed to load pending transfers for ${token}:`, err);
        }
    }
    return Array.from(byContract.values());
}

async function loadBurnHoldings() {
    try {
        const filter = $('burnParty').value;
        const party = filter || 'admin';
        const h = await api(`/holdings?party=${party}`);
        renderHoldingsTable('burnHoldingsList', h.filter(x => !x.locked), true);
    } catch (err) {
        $('burnHoldingsList').innerHTML = `<p class="error">${err.message}</p>`;
    }
}

async function loadParties() {
    try {
        parties = await api('/parties');
        renderPartiesTable(parties);
        populatePartySelects();
    } catch (err) {
        $('partyList').innerHTML = `<p class="error">${err.message}</p>`;
    }
}

function renderHoldingsTable(containerId, data, clickToFill = false, showObservedVia = false) {
    const el = $(containerId);
    if (!data.length) {
        el.innerHTML = '<p>No holdings found.</p>';
        return;
    }
    let html = `<table><thead><tr>
        <th>Owner</th><th>Amount</th><th>Coupon</th><th>Maturity</th><th>Description</th><th>Status</th>
        ${showObservedVia ? '<th>Observed Via</th>' : ''}
        ${clickToFill ? '<th>Action</th>' : ''}
    </tr></thead><tbody>`;
    for (const h of data) {
        const observedVia = (h.observedViaParticipants || []).join(', ') || '-';
        html += `<tr>
            <td>${shortName(h.owner)}</td>
            <td>${h.amount}</td>
            <td>${h.couponRate}%</td>
            <td>${h.maturityDate}</td>
            <td>${h.description || '-'}</td>
            <td>${h.locked ? '<span class="badge badge-locked">Locked</span>' : '<span class="badge badge-active">Active</span>'}</td>
            ${showObservedVia ? `<td>${observedVia}</td>` : ''}
            ${clickToFill ? `<td><button class="btn-small" data-action="fill-burn" data-cid="${h.contractId}">Select</button></td>` : ''}
        </tr>`;
    }
    html += '</tbody></table>';
    el.innerHTML = html;
}

function renderPendingTable(data) {
    const el = $('pendingList');
    const partyFilter = $('pendingFilter').value;
    const filtered = data.filter(t => {
        if (!partyFilter) return true;
        return shortName(t.sender) === partyFilter || shortName(t.receiver) === partyFilter;
    });

    if (!filtered.length) {
        el.innerHTML = '<p>No pending transfers.</p>';
        return;
    }
    let html = `<table><thead><tr>
        <th>Sender</th><th>Receiver</th><th>Amount</th><th>Contract ID</th><th>Actions</th>
    </tr></thead><tbody>`;
    for (const t of filtered) {
        html += `<tr>
            <td>${shortName(t.sender)}</td>
            <td>${shortName(t.receiver)}</td>
            <td>${t.amount}</td>
            <td style="font-size:0.75rem;max-width:200px;overflow:hidden;text-overflow:ellipsis">${t.contractId}</td>
            <td>
                <button class="btn-small btn-success" data-action="accept-transfer" data-cid="${t.contractId}" data-party="${shortName(t.receiver)}">Accept</button>
                <button class="btn-small btn-danger" data-action="reject-transfer" data-cid="${t.contractId}" data-party="${shortName(t.receiver)}">Reject</button>
                <button class="btn-small" data-action="withdraw-transfer" data-cid="${t.contractId}" data-party="${shortName(t.sender)}">Withdraw</button>
            </td>
        </tr>`;
    }
    html += '</tbody></table>';
    el.innerHTML = html;
}

function renderAllocationsTable(data) {
    const el = $('allocationsList');
    if (!data.length) {
        el.innerHTML = '<p>No allocations found.</p>';
        return;
    }

    let html = `<table><thead><tr>
    <th>Sender</th>
    <th>Receiver</th>
    <th>Executor</th>
    <th>Amount</th>
    <th>Instrument</th>
    <th>Allocate Before</th>
    <th>Settle Before</th>
    <th>Contract ID</th>
    <th>Actions</th>
    </tr></thead><tbody>`;

    for (const a of data) {
        const sender = shortName(a.sender);
        const receiver = shortName(a.receiver);
        const executor = shortName(a.executor);

        html += `<tr>
        <td>${sender}</td>
        <td>${receiver}</td>
        <td>${executor}</td>
        <td>${a.amount}</td>
        <td title="${a.instrumentId || '-'}">${(a.instrumentId || '-').split(':').pop()}</td>
        <td>${a.allocateBefore ? a.allocateBefore.replace('T', ' ').substring(0, 16) : '-'}</td>
        <td>${a.settleBefore ? a.settleBefore.replace('T', ' ').substring(0, 16) : '-'}</td>
        <td style="font-size:0.75rem;max-width:120px;overflow:hidden;text-overflow:ellipsis" title="${a.contractId}">${a.contractId.substring(0, 12)}…</td>
        <td>
        <button class="btn-small btn-success" data-action="execute-allocation" data-cid="${a.contractId}" data-party="${executor}">Execute</button>
        <button class="btn-small btn-danger" data-action="cancel-allocation" data-cid="${a.contractId}" data-party="${sender}">Cancel</button>
        <button class="btn-small" data-action="withdraw-allocation" data-cid="${a.contractId}" data-party="${sender}">Withdraw</button>
        </td>
        </tr>`;
    }

    html += '</tbody></table>';
    el.innerHTML = html;
}

function renderPartiesTable(data) {
    const el = $('partyList');
    if (!data.length) {
        el.innerHTML = '<p>No parties found.</p>';
        return;
    }
    let html = `<table><thead><tr><th>Identifier</th><th>Display Name</th><th>Participant</th></tr></thead><tbody>`;
    for (const p of data) {
        html += `<tr><td style="font-size:0.8rem">${p.identifier}</td><td>${p.displayName || '-'}</td><td>${p.participant}</td></tr>`;
    }
    html += '</tbody></table>';
    el.innerHTML = html;
}

function shortName(id) {
    if (!id) return '-';
    return id.split('::')[0];
}

function partyToken(p) {
    if (!p) return '';
    if (p.displayName && p.displayName.trim() !== '') return p.displayName;
    return shortName(p.identifier);
}

function uniqueParties(list) {
    const unique = [];
    const seen = new Set();
    for (const p of list || []) {
        const token = partyToken(p);
        if (!token || seen.has(token)) continue;
        seen.add(token);
        unique.push(p);
    }
    return unique;
}

function populatePartySelects() {
    const selects = [
        'mintAdmin',
        'mintOwner',
        'transferSender',
        'transferReceiver',
        'burnParty',
        'holdingsFilter',
        'pendingFilter',
        'allocationSender',
        'allocationReceiver',
        'allocationExecutor',
        'allocationsFilter'
    ];
    const unique = uniqueParties(parties);
    for (const id of selects) {
        const sel = $(id);
        if (!sel) continue;
        const current = sel.value;
        sel.innerHTML = id === 'holdingsFilter' || id === 'pendingFilter' || id === 'allocationsFilter' ? '<option value="">All</option>' : '';
        for (const p of unique) {
            const token = partyToken(p);
            const opt = document.createElement('option');
            opt.value = token;
            opt.textContent = token;
            sel.appendChild(opt);
        }
        if (current) sel.value = current;
    }
}

function updateStatus(state, msg) {
    const badge = $('statusBadge');
    badge.className = 'status-badge';
    if (state === 'ready') {
        badge.classList.add('connected');
        badge.textContent = '✅ ' + (msg || 'Factory ready');
    } else if (state === 'connected') {
        badge.classList.add('connected');
        badge.textContent = '✅ ' + (msg || 'Connected');
    } else if (state === 'locked') {
        badge.classList.add('locked');
        badge.textContent = msg || '⏳ Initializing...';
    } else if (state === 'error') {
        badge.classList.add('error');
        badge.textContent = '❌ ' + (msg || 'Error');
    } else {
        badge.textContent = '⏳ ' + (msg || 'Connecting...');
    }
}

function fillBurn(cid) {
    $('burnContractId').value = cid;
    $('burnContractId').scrollIntoView({ behavior: 'smooth' });
}

// ---- Event Delegation (for dynamic buttons in ES module mode) ----
document.addEventListener('click', e => {
    const btn = e.target.closest('[data-action]');
    if (!btn) return;
    const action = btn.dataset.action;
    const cid = btn.dataset.cid;
    const party = btn.dataset.party;
    switch (action) {
        case 'fill-burn':
            fillBurn(cid);
            break;
        case 'accept-transfer':
            if (confirm('Accept this transfer?')) {
                acceptTransfer(cid, party);
            }
            break;
        case 'reject-transfer':
            if (confirm('Reject this transfer?')) {
                rejectTransfer(cid, party);
            }
            break;
        case 'withdraw-transfer':
            if (confirm('Withdraw this transfer?')) {
                withdrawTransfer(cid, party);
            }
            break;
        case 'execute-allocation':
            if (confirm('Execute this allocation? This will settle the transfer.')) {
                executeAllocation(cid, party);
            }
            break;
        case 'cancel-allocation':
            if (confirm('Cancel this allocation?')) {
                cancelAllocation(cid, party);
            }
            break;
        case 'withdraw-allocation':
            if (confirm('Withdraw this allocation? Your holdings will be returned.')) {
                withdrawAllocation(cid, party);
            }
            break;
    }
});

// ---- Event Handlers ----

$('mintForm').addEventListener('submit', async e => {
    e.preventDefault();
    const btn = e.target.querySelector('button');
    const result = $('mintResult');
    hideResult(result);
    btn.disabled = true;
    try {
        const data = await api('/mint', {
            method: 'POST',
            body: {
                admin: $('mintAdmin').value,
                owner: $('mintOwner').value,
                amount: parseFloat($('mintAmount').value),
                couponRate: parseFloat($('mintCoupon').value),
                maturityDate: $('mintMaturity').value,
                description: $('mintDescription').value,
                instrumentId: $('mintInstrument').value,
            },
        });
        showResult(result, `Bond minted! Offset: ${data.offset}`);
        setTimeout(() => loadDashboard(), 1000);
    } catch (err) {
        showResult(result, err.message, true);
    } finally {
        btn.disabled = false;
    }
});

$('transferForm').addEventListener('submit', async e => {
    e.preventDefault();
    const btn = e.target.querySelector('button');
    const result = $('transferResult');
    hideResult(result);
    btn.disabled = true;
    try {
        const data = await api('/transfer', {
            method: 'POST',
            body: {
                sender: $('transferSender').value,
                receiver: $('transferReceiver').value,
                amount: parseFloat($('transferAmount').value),
                instrumentId: $('transferInstrument').value,
            },
        });
        showResult(result, `Transfer initiated! Status: ${data.status}`);
        setTimeout(() => loadDashboard(), 1000);
    } catch (err) {
        showResult(result, err.message, true);
    } finally {
        btn.disabled = false;
    }
});

async function acceptTransfer(cid, party) {
    try {
        const data = await api('/transfer/accept', {
            method: 'POST',
            body: { party, contractId: cid },
        });
        alert('Transfer accepted!');
        loadPending();
        loadDashboard();
    } catch (err) {
        alert('Error: ' + err.message);
    }
}

async function rejectTransfer(cid, party) {
    try {
        await api('/transfer/reject', {
            method: 'POST',
            body: { party, contractId: cid },
        });
        alert('Transfer rejected!');
        loadPending();
        loadDashboard();
    } catch (err) {
        alert('Error: ' + err.message);
    }
}

async function withdrawTransfer(cid, party) {
    try {
        await api('/transfer/withdraw', {
            method: 'POST',
            body: { party, contractId: cid },
        });
        alert('Transfer withdrawn!');
        loadPending();
        loadDashboard();
    } catch (err) {
        alert('Error: ' + err.message);
    }
}

$('burnForm').addEventListener('submit', async e => {
    e.preventDefault();
    const btn = e.target.querySelector('button');
    const result = $('burnResult');
    hideResult(result);
    btn.disabled = true;
    try {
        const data = await api('/burn', {
            method: 'POST',
            body: {
                party: $('burnParty').value,
                contractId: $('burnContractId').value,
                asAdmin: $('burnAsAdmin').checked,
            },
        });
        showResult(result, `Bond burned! Offset: ${data.offset}`);
        setTimeout(() => { loadBurnHoldings(); loadDashboard(); }, 1000);
    } catch (err) {
        showResult(result, err.message, true);
    } finally {
        btn.disabled = false;
    }
});

// Allocation create
$('allocationForm').addEventListener('submit', async e => {
    e.preventDefault();
    const btn = e.target.querySelector('button');
    const result = $('allocationResult');
    hideResult(result);
    if (!requireFactoryReady(result)) return;
    btn.disabled = true;
    try {
        const allocateBeforeStr = $('allocationAllocateBefore').value;
        const settleBeforeStr = $('allocationSettleBefore').value;

        if (new Date(allocateBeforeStr) <= new Date()) {
            showResult(result, "Allocate Before date must be in the future.", true);
            btn.disabled = false;
            return;
        }

        if (new Date(allocateBeforeStr) > new Date(settleBeforeStr)) {
            showResult(result, "Allocate Before date must be <= Settle Before date.", true);
            btn.disabled = false;
            return;
        }

        const data = await api('/allocations', {
            method: 'POST',
            body: {
                sender: $('allocationSender').value,
                receiver: $('allocationReceiver').value,
                executor: $('allocationExecutor').value,
                amount: parseFloat($('allocationAmount').value),
                allocateBefore: toISOStringFromLocal(allocateBeforeStr),
                settleBefore: toISOStringFromLocal(settleBeforeStr),
                settlementRef: $('allocationSettlementRef').value.trim(),
                transferLegId: $('allocationTransferLegId').value.trim(),
                instrumentId: $('allocationInstrument').value,
            },
        });
        showResult(result, `Allocation created! Offset: ${data.offset}`);
        setTimeout(() => { loadAllocations(); loadDashboard(); }, 1000);
    } catch (err) {
        showResult(result, err.message, true);
    } finally {
        btn.disabled = false;
    }
});

// Allocation actions
async function executeAllocation(cid, party) {
    try {
        await api('/allocations/execute', {
            method: 'POST',
            body: { party, contractId: cid },
        });
        alert('Allocation executed! Bonds transferred to receiver.');
        loadAllocations();
        loadDashboard();
    } catch (err) {
        alert('Error: ' + err.message);
    }
}

async function cancelAllocation(cid, party) {
    try {
        await api('/allocations/cancel', {
            method: 'POST',
            body: { party, contractId: cid },
        });
        alert('Allocation canceled! Holdings returned to sender.');
        loadAllocations();
        loadDashboard();
    } catch (err) {
        alert('Error: ' + err.message);
    }
}

async function withdrawAllocation(cid, party) {
    try {
        await api('/allocations/withdraw', {
            method: 'POST',
            body: { party, contractId: cid },
        });
        alert('Allocation withdrawn! Holdings returned to sender.');
        loadAllocations();
        loadDashboard();
    } catch (err) {
        alert('Error: ' + err.message);
    }
}

$('partyForm').addEventListener('submit', async e => {
    e.preventDefault();
    const btn = e.target.querySelector('button');
    const result = $('partyResult');
    hideResult(result);
    btn.disabled = true;
    try {
        const data = await api('/parties', {
            method: 'POST',
            body: {
                participant: $('partyParticipant').value,
                hint: $('partyHint').value,
            },
        });
        showResult(result, `Party created: ${data.identifier}`);
        setTimeout(() => loadParties(), 1000);
    } catch (err) {
        showResult(result, err.message, true);
    } finally {
        btn.disabled = false;
    }
});

$('holdingsFilter').addEventListener('change', loadHoldings);
$('pendingFilter').addEventListener('change', loadPending);
$('allocationsFilter').addEventListener('change', loadAllocations);
$('burnParty').addEventListener('change', loadBurnHoldings);

// Init - auto-create factory
(async function init() {
    try {
        const health = await api('/health');
        const listenerUrl = health.listenerUrl || null;
        initWebSocket(listenerUrl);
        setFactoryLocked(true, 'Initializing...');
        $('dashboardHoldings').innerHTML = '<p class="loading">Press Start Factory to initialize the ledger.</p>';

        const maxRetries = 10;
        for (let attempt = 1; attempt <= maxRetries; attempt++) {
            try {
                const data = await api('/factory', { method: 'POST' });
                setFactoryLocked(false, data.status === 'created' ? 'Factory ready' : 'Factory ready');
                await loadParties();
                await loadDashboard();
                if (currentPage !== 'dashboard') {
                    loadPageData(currentPage);
                }
                return;
            } catch (err) {
                if (attempt < maxRetries) {
                    const delay = Math.min(3000 * attempt, 15000);
                    updateStatus('locked', `Retrying (${attempt}/${maxRetries})...`);
                    await new Promise(r => setTimeout(r, delay));
                } else {
                    setFactoryLocked(true, 'Initialization failed');
                    updateStatus('error', err.message);
                }
            }
        }
    } catch (err) {
        updateStatus('error', err.message);
        setFactoryLocked(true, 'Factory not started - press Start Factory');
    }
})();

// ---- Conexión WebSocket en Tiempo Real ----
function initWebSocket(listenerUrl) {
    const wsHost = window.location.hostname;
    const url = listenerUrl || `ws://${wsHost}:8081/ws/bonds`;
    const ws = new WebSocket(url);

    ws.onopen = () => {
        console.log('✅ Conectado al Listener en vivo (WebSocket)');
    };

    ws.onmessage = (event) => {
        const data = JSON.parse(event.data);
        console.log('📬 Evento instantáneo del Ledger:', data);

        const tpl = data.templateId;
        const action = data.action;

        if (tpl === 'SimpleHolding' || tpl === 'LockedSimpleHolding') {
            if (action === 'created' || action === 'archived') {
                if (currentPage === 'dashboard') loadDashboard();
                if (currentPage === 'holdings') loadHoldings();
                if (action === 'archived' && currentPage === 'burn') loadBurnHoldings();
            }
        } else if (tpl === 'SimpleTransferInstruction') {
            if (currentPage === 'dashboard') loadDashboard();
            if (currentPage === 'pending') loadPending();
        } else if (tpl === 'SimpleAllocation') {
            if (currentPage === 'dashboard') loadDashboard();
            if (currentPage === 'allocations') loadAllocations();
        } else if (tpl === 'SimpleTokenRules') {
            loadDashboard();
        }
    };

    ws.onclose = () => {
        console.log('❌ WebSocket desconectado. Reintentando en 5s...');
        setTimeout(() => initWebSocket(listenerUrl), 5000);
    };

    ws.onerror = (err) => {
        console.error('Error en WebSocket:', err);
    };
}