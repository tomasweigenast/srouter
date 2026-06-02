// ─── Toast notifications ──────────────────────────────────────────────────────

function toast(message, type) {
  type = type || 'success';
  let container = document.getElementById('toast-container');
  if (!container) {
    container = document.createElement('div');
    container.id = 'toast-container';
    container.className = 'fixed bottom-4 right-4 z-50 flex flex-col gap-2 pointer-events-none';
    document.body.appendChild(container);
  }
  const colors = { success: 'bg-emerald-600', error: 'bg-red-600', info: 'bg-gray-800' };
  const el = document.createElement('div');
  el.className = 'px-4 py-3 rounded-lg shadow-lg text-sm text-white pointer-events-auto ' + (colors[type] || colors.info);
  el.textContent = message;
  container.appendChild(el);
  setTimeout(() => el.remove(), 3000);
}

// ─── Confirm modal ────────────────────────────────────────────────────────────

function confirmModal(message) {
  return new Promise(function(resolve) {
    const overlay = document.createElement('div');
    overlay.className = 'fixed inset-0 bg-black/50 z-50 flex items-center justify-center';
    const box = document.createElement('div');
    box.className = 'bg-white rounded-xl p-6 shadow-xl max-w-sm w-full mx-4 space-y-4';
    const msg = document.createElement('p');
    msg.className = 'text-sm text-gray-800';
    msg.textContent = message;
    const btns = document.createElement('div');
    btns.className = 'flex justify-end gap-2';
    const cancel = document.createElement('button');
    cancel.className = 'text-sm text-gray-500 hover:text-gray-900 px-4 py-2 rounded-lg transition-colors';
    cancel.textContent = 'Cancel';
    const confirm = document.createElement('button');
    confirm.className = 'text-sm bg-red-600 hover:bg-red-500 text-white px-4 py-2 rounded-lg transition-colors';
    confirm.textContent = 'Confirm';
    btns.append(cancel, confirm);
    box.append(msg, btns);
    overlay.appendChild(box);
    document.body.appendChild(overlay);
    const close = function(v) { overlay.remove(); resolve(v); };
    cancel.addEventListener('click', function() { close(false); });
    confirm.addEventListener('click', function() { close(true); });
    overlay.addEventListener('click', function(e) { if (e.target === overlay) close(false); });
  });
}

// ─── Fetch helper ─────────────────────────────────────────────────────────────

async function request(method, url, body, btn) {
  if (btn) { btn.disabled = true; btn._orig = btn.innerHTML; btn.innerHTML = '&hellip;'; }
  try {
    const opts = { method: method };
    if (body && method !== 'GET') opts.body = body;
    return await fetch(url, opts);
  } finally {
    if (btn) { btn.disabled = false; btn.innerHTML = btn._orig; }
  }
}

// ─── DOM helpers ─────────────────────────────────────────────────────────────

function swapHTML(selector, html, method) {
  const el = typeof selector === 'string' ? document.querySelector(selector) : selector;
  if (!el) return;
  if (method === 'outerHTML') { el.outerHTML = html; }
  else if (method === 'beforeend') { el.insertAdjacentHTML('beforeend', html); }
  else { el.innerHTML = html; }
}

function toFormData(body) {
  if (!body || body instanceof FormData || body instanceof URLSearchParams) return body;
  const fd = new FormData();
  for (const k of Object.keys(body)) fd.append(k, body[k]);
  return fd;
}

// ─── Action helpers ──────────────────────────────────────────────────────────

// Delete a row with a confirm dialog, then remove the <tr> on success.
async function deleteRowConfirm(btn) {
  const msg = btn.dataset.confirm || 'Delete?';
  if (!await confirmModal(msg)) return;
  const res = await request('DELETE', btn.dataset.url, null, btn);
  if (res.ok) btn.closest('tr').remove();
  else toast('Failed to delete', 'error');
}

// Fetch and replace an element (outerHTML) with the response HTML.
async function fetchReplace(btn, method, url, body, selector) {
  const res = await request(method, url, toFormData(body), btn);
  if (!res.ok) { toast('Request failed', 'error'); return; }
  const html = await res.text();
  const el = document.querySelector(selector);
  if (el) el.outerHTML = html;
}

// Fetch and inject response HTML into a target element.
async function fetchSwap(btn, method, url, selector, swap, body) {
  const res = await request(method, url, toFormData(body), btn);
  if (!res.ok) { toast('Request failed', 'error'); return; }
  swapHTML(selector, await res.text(), swap || 'innerHTML');
}

// Fetch and show a toast from the JSON response.
async function fetchToast(btn, method, url, body) {
  const res = await request(method, url, toFormData(body), btn);
  let msg = res.ok ? 'Done' : 'Failed';
  try { const d = await res.json(); if (d.message) msg = d.message; } catch {}
  toast(msg, res.ok ? 'success' : 'error');
}

// Delete a firewall custom rule with confirm, then refresh the custom-rules-list.
async function deleteCustomRule(btn) {
  if (!await confirmModal('Delete this rule?')) return;
  fetchSwap(btn, 'DELETE', '/firewall/rules/' + btn.dataset.idx, '#custom-rules-list', 'innerHTML');
}

// Confirm + reboot via POST /system/reboot, show toast.
async function rebootNow(btn) {
  if (!await confirmModal('Reboot the router now?')) return;
  fetchToast(btn, 'POST', '/system/reboot');
}

// Install update: shows a spinner with live status text, polls /ping to detect
// service-down then service-up, then reloads the page.
async function installUpdate(btn) {
  if (!await confirmModal('Install the update and restart the service?')) return;

  const area     = document.getElementById('install-area');
  const progress = document.getElementById('install-progress');
  const statusEl = document.getElementById('install-status-text');

  function setStatus(text) {
    if (statusEl) statusEl.textContent = text;
  }

  // Switch to spinner view.
  if (area)     area.classList.add('hidden');
  if (progress) progress.classList.remove('hidden');
  setStatus('Downloading update…');

  // Trigger install on the server (returns immediately; install runs in background).
  let res;
  try {
    res = await request('POST', '/system/update/install', null, btn);
  } catch (err) {
    toast('Request failed', 'error');
    if (area)     area.classList.remove('hidden');
    if (progress) progress.classList.add('hidden');
    return;
  }

  if (!res.ok) {
    let msg = 'Install failed';
    try { const d = await res.json(); if (d.message) msg = d.message; } catch {}
    toast(msg, 'error');
    if (area)     area.classList.remove('hidden');
    if (progress) progress.classList.add('hidden');
    return;
  }

  setStatus('Waiting for service to restart…');

  // Poll /ping until service goes down, then comes back up.
  let wentDown = false;
  (async function poll() {
    try {
      const r = await fetch('/ping', { signal: AbortSignal.timeout(1000) });
      if (r.status === 204) {
        if (wentDown) {
          // Service is back — reload to show new version.
          setStatus('Update complete — reloading…');
          sessionStorage.setItem('update_success', '1');
          setTimeout(() => window.location.reload(), 800);
          return;
        }
        // Still up; keep waiting.
        setTimeout(poll, 1000);
        return;
      }
    } catch {
      // fetch threw (connection refused / timeout) — service is down.
    }
    if (!wentDown) {
      wentDown = true;
      setStatus('Service restarting…');
    }
    setTimeout(poll, 1000);
  })();
}

// ─── Delegated form handler ───────────────────────────────────────────────────
// Intercepts forms with data-ajax="true".
//
//   data-target="#sel"        CSS selector for HTML injection target
//   data-swap="beforeend"     swap method: innerHTML (default), outerHTML, beforeend
//   data-reset="true"         reset the form on success
//   data-json="true"          expect JSON {ok, message}; show toast instead of injecting HTML
//   data-error-target="#sel"  where to display inline validation error text

document.addEventListener('submit', async function(e) {
  const form = e.target;
  if (!form.dataset.ajax) return;
  e.preventDefault();

  const btn = form.querySelector('[type=submit]');
  const res = await request((form.method || 'post').toUpperCase(), form.action, new URLSearchParams(new FormData(form)), btn);

  if (form.dataset.json) {
    let msg = res.ok ? 'Done' : 'Failed';
    try { const d = await res.json(); if (d.message) msg = d.message; } catch {}
    toast(msg, res.ok ? 'success' : 'error');
    return;
  }

  if (!res.ok) {
    let msg = 'Request failed';
    const ct = res.headers.get('content-type') || '';
    if (ct.includes('json')) { try { const d = await res.json(); if (d.message) msg = d.message; } catch {} }
    const errSel = form.dataset.errorTarget;
    if (errSel) { const el = document.querySelector(errSel); if (el) el.textContent = msg; }
    else toast(msg, 'error');
    return;
  }

  const html = await res.text();
  if (form.dataset.target) swapHTML(form.dataset.target, html, form.dataset.swap || 'innerHTML');
  if (form.dataset.reset === 'true') form.reset();
  const errSel = form.dataset.errorTarget;
  if (errSel) { const el = document.querySelector(errSel); if (el) el.textContent = ''; }
});

// ─── Post-update success toast ────────────────────────────────────────────────

if (sessionStorage.getItem('update_success')) {
  sessionStorage.removeItem('update_success');
  toast('Update installed successfully!', 'success');
}
