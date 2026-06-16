const API = (process.env.E2E_API_URL || 'http://localhost:3000') + '/api/v1';

export async function apiRequest(path, options = {}) {
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

export async function initFactory() {
  return apiRequest('/factory', { method: 'POST' });
}

export async function mintBond({ admin, owner, amount, couponRate = 5.0, maturityDate = '2028-12-31', description = 'Test Bond' }) {
  return apiRequest('/mint', {
    method: 'POST',
    body: { admin, owner, amount, couponRate, maturityDate, description },
  });
}

export async function transferBond({ sender, receiver, amount }) {
  return apiRequest('/transfer', {
    method: 'POST',
    body: { sender, receiver, amount },
  });
}

export async function acceptTransfer({ party, contractId }) {
  return apiRequest('/transfer/accept', {
    method: 'POST',
    body: { party, contractId },
  });
}

export async function rejectTransfer({ party, contractId }) {
  return apiRequest('/transfer/reject', {
    method: 'POST',
    body: { party, contractId },
  });
}

export async function withdrawTransfer({ party, contractId }) {
  return apiRequest('/transfer/withdraw', {
    method: 'POST',
    body: { party, contractId },
  });
}

export async function burnBond({ party, contractId, asAdmin = false }) {
  return apiRequest('/burn', {
    method: 'POST',
    body: { party, contractId, asAdmin },
  });
}

export async function createParty({ participant, hint }) {
  return apiRequest('/parties', {
    method: 'POST',
    body: { participant, hint },
  });
}

export async function listParties() {
  return apiRequest('/parties');
}

export async function listHoldings(party) {
  return apiRequest(`/holdings?party=${party}`);
}

export async function listTransferInstructions(party) {
  return apiRequest(`/transfer-instructions?party=${party}`);
}

export async function healthCheck() {
  return apiRequest('/health');
}

export async function getAllHoldings() {
  const parties = await listParties();
  const all = [];
  const seen = new Set();
  for (const p of parties) {
    const token = partyToken(p);
    if (!token || seen.has(token)) continue;
    seen.add(token);
    try {
      const h = await listHoldings(token);
      all.push(...h);
    } catch (_) {}
  }
  return all;
}

function partyToken(p) {
  if (!p) return '';
  if (p.displayName && p.displayName.trim() !== '') return p.displayName;
  return (p.identifier || '').split('::')[0];
}
