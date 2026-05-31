const API = '/api/v1';
let parties = [];
let holdings = [];
let pendingTransfers = [];
let selectedParty = null;

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

// Navigation
document.querySelectorAll('nav a').forEach(a => {
    a.addEventListener('click', e => {
        e.preventDefault();
        document.querySelectorAll('nav a').forEach(x => x.classList.remove('active'));
        a.classList.add('active');
        document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
        const page = $('page-' + a.dataset.page);
        if (page) page.classList.add('active');
        loadPageData(a.dataset.page);
    });
});

async function loadPageData(page) {
    switch (page) {
        case 'dashboard': loadDashboard(); break;
        case 'holdings': loadHoldings(); break;
        case 'pending': loadPending(); break;
        case 'burn': loadBurnHoldings(); break;
        case 'mint': populatePartySelects(); break;
        case 'transfer': populatePartySelects(); break;
        case 'parties': loadParties(); break;
    }
}

async function loadDashboard() {
    try {
        const [h, pts, pends] = await Promise.all([
            api('/holdings?party=admin'),
            api('/parties'),
            loadPendingData(),
        ]);
        parties = pts;
        pendingTransfers = pends;

        let allHoldings = h;
        // Also get other parties' holdings
        for (const p of parties) {
            const token = partyToken(p);
            if (token !== 'admin') {
                try {
                    const more = await api(`/holdings?party=${token}`);
                    allHoldings = allHoldings.concat(more);
                } catch (_) {}
            }
        }
        holdings = allHoldings;

        const totalValue = holdings.reduce((sum, h) => sum + (h.locked ? 0 : h.amount), 0);
        $('statTotalBonds').textContent = holdings.filter(h => !h.locked).length;
        $('statTotalValue').textContent = totalValue.toLocaleString();
        $('statParties').textContent = parties.length;
        $('statPending').textContent = pendingTransfers.length;

        renderHoldingsTable('dashboardHoldings', holdings);
        updateStatus('connected');
    } catch (err) {
        $('dashboardHoldings').innerHTML = `<p class="error">Failed: ${err.message}</p>`;
        updateStatus('error', err.message);
    }
}

async function loadHoldings() {
    try {
        const filter = $('holdingsFilter').value;
        let all = [];
        for (const p of parties) {
            const token = partyToken(p);
            if (filter && token !== filter) continue;
            try {
                const h = await api(`/holdings?party=${token}`);
                all = all.concat(h);
            } catch (_) {}
        }
        holdings = all;
        renderHoldingsTable('holdingsList', holdings);
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

async function loadPendingData() {
    let all = [];
    for (const p of parties) {
        const token = partyToken(p);
        if (!token) continue;
        try {
            const t = await api(`/transfer-instructions?party=${token}`);
            all = all.concat(t);
        } catch (err) {
            console.warn(`Failed to load pending transfers for ${token}:`, err);
        }
    }
    return all;
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

function renderHoldingsTable(containerId, data, clickToFill = false) {
    const el = $(containerId);
    if (!data.length) {
        el.innerHTML = '<p>No holdings found.</p>';
        return;
    }
    let html = `<table><thead><tr>
        <th>Owner</th><th>Amount</th><th>Coupon</th><th>Maturity</th><th>Description</th><th>Status</th>
        ${clickToFill ? '<th>Action</th>' : ''}
    </tr></thead><tbody>`;
    for (const h of data) {
        html += `<tr>
            <td>${shortName(h.owner)}</td>
            <td>${h.amount}</td>
            <td>${h.couponRate}%</td>
            <td>${h.maturityDate}</td>
            <td>${h.description || '-'}</td>
            <td>${h.locked ? '<span class="badge badge-locked">Locked</span>' : '<span class="badge badge-active">Active</span>'}</td>
            ${clickToFill ? `<td><button class="btn-small" onclick="fillBurn('${h.contractId}')">Select</button></td>` : ''}
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
                <button class="btn-small btn-success" onclick="acceptTransfer('${t.contractId}', '${shortName(t.receiver)}')">Accept</button>
                <button class="btn-small btn-danger" onclick="rejectTransfer('${t.contractId}', '${shortName(t.sender)}')">Reject</button>
                <button class="btn-small" onclick="withdrawTransfer('${t.contractId}', '${shortName(t.sender)}')">Withdraw</button>
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

function populatePartySelects() {
    const selects = ['mintAdmin', 'mintOwner', 'transferSender', 'transferReceiver', 'burnParty', 'holdingsFilter', 'pendingFilter'];
    for (const id of selects) {
        const sel = $(id);
        if (!sel) continue;
        const current = sel.value;
        sel.innerHTML = id === 'holdingsFilter' || id === 'pendingFilter' ? '<option value="">All</option>' : '';
        for (const p of parties) {
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
    if (state === 'connected') {
        badge.classList.add('connected');
        badge.textContent = '✅ Connected';
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

// ---- Event Handlers ----

// Mint
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

// Transfer
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

// Accept transfer
async function acceptTransfer(cid, party) {
    if (!confirm('Accept this transfer?')) return;
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

// Reject transfer
async function rejectTransfer(cid, party) {
    if (!confirm('Reject this transfer?')) return;
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

// Withdraw transfer
async function withdrawTransfer(cid, party) {
    if (!confirm('Withdraw this transfer?')) return;
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

// Burn
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

// Party form
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

// Holdings filter
$('holdingsFilter').addEventListener('change', loadHoldings);
$('pendingFilter').addEventListener('change', loadPending);
$('burnParty').addEventListener('change', loadBurnHoldings);

// Init
(async function init() {
    try {
        await api('/health');
        // Ensure the SimpleTokenRules factory exists before user actions.
        await api('/factory');
        await loadParties();
        await loadDashboard();
    } catch (err) {
        updateStatus('error', err.message);
        // Retry after 3 seconds
        setTimeout(() => {
            $('statusBadge').textContent = '⏳ Retrying...';
            init();
        }, 3000);
    }
})();
