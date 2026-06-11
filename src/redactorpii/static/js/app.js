let currentStep = 1;
let selectedDocId = '';
let currentReqId = '';
let currentScenario = null;
let currentFramework = '';
const API = '';

// ═══════════ SIDEBAR TOGGLE (mobile) ═══════════
function toggleSidebar() {
  const sidebar = document.getElementById('app-sidebar');
  const overlay = document.getElementById('sidebar-overlay');
  const btn = document.getElementById('hamburger-btn');
  const isOpen = sidebar.classList.contains('open');
  if (isOpen) {
    closeSidebar();
  } else {
    sidebar.classList.add('open');
    overlay.classList.add('show');
    btn.setAttribute('aria-expanded', 'true');
    sidebar.querySelector('.nav-item')?.focus();
  }
}

function closeSidebar() {
  const sidebar = document.getElementById('app-sidebar');
  const overlay = document.getElementById('sidebar-overlay');
  const btn = document.getElementById('hamburger-btn');
  sidebar.classList.remove('open');
  overlay.classList.remove('show');
  btn.setAttribute('aria-expanded', 'false');
  btn.focus();
}

// Close sidebar on Escape
document.addEventListener('keydown', function(e) {
  if (e.key === 'Escape') {
    const sidebar = document.getElementById('app-sidebar');
    if (sidebar && sidebar.classList.contains('open')) {
      closeSidebar();
    }
  }
});

// ═══════════ FOCUS TRAP FOR MODALS ═══════════
function trapFocus(modalEl) {
  const focusable = modalEl.querySelectorAll(
    'button:not([disabled]):not([style*="display:none"]), ' +
    'input:not([disabled]):not([type="hidden"]), ' +
    'select:not([disabled]), ' +
    'textarea:not([disabled]), ' +
    '[tabindex]:not([tabindex="-1"])'
  );
  if (!focusable.length) return;
  const first = focusable[0];
  const last = focusable[focusable.length - 1];

  function handler(e) {
    if (e.key !== 'Tab') return;
    if (e.shiftKey) {
      if (document.activeElement === first) {
        e.preventDefault();
        last.focus();
      }
    } else {
      if (document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    }
  }
  modalEl._focusTrapHandler = handler;
  modalEl.addEventListener('keydown', handler);
  first.focus();
}

function releaseFocus(modalEl) {
  if (modalEl._focusTrapHandler) {
    modalEl.removeEventListener('keydown', modalEl._focusTrapHandler);
    modalEl._focusTrapHandler = null;
  }
}

// ═══════════ SETUP WIZARD ═══════════
function hideAllViews() {
  document.getElementById('landing-page').classList.add('hidden');
  document.getElementById('dms-dashboard').classList.add('hidden');
}

function backToSelector() {
  hideAllViews();
  document.getElementById('landing-page').classList.remove('hidden');
  document.getElementById('app-header').classList.add('hidden');
  currentStep = 1;
  selectedDocId = '';
  currentReqId = '';
  currentScenario = null;
}

function backToOverview() {
  showDmsDashboard();
}

function navTo(view) {
  if (view === 'dashboard') showDmsDashboard();
}

function toggleTheme() {
  const h = document.documentElement;
  h.dataset.theme = h.dataset.theme === 'light' ? 'dark' : 'light';
  localStorage.setItem('theme', h.dataset.theme);
}
(function() {
  const s = localStorage.getItem('theme');
  if (s) document.documentElement.dataset.theme = s;
})();


function closeModal() {
  releaseFocus(document.getElementById('pii-modal'));
  document.getElementById('pii-modal').style.display = 'none';
  if (typeof patternLabReset === 'function') patternLabReset();
}

async function addCustomPattern() {
  const pattern = document.getElementById('custom-pattern-input').value.trim();
  if (!pattern) return;

  if (window._pasteLabText) {
    pasteApplyPattern('literal', pattern);
    document.getElementById('pattern-result').innerHTML =
      '<div class="pattern-lab-success">&#10003; Added literal rule: &quot;' + esc(pattern) + '&quot;</div>';
    document.getElementById('custom-pattern-input').value = '';
    return;
  }

  const activeDocId = selectedDocId || dmsSelectedDocId;
  if (!activeDocId) return;
  const r = await fetch(API + '/api/documents/add-pattern', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ pattern: pattern, docId: activeDocId })
  });
  const data = await r.json();
  const result = (data && data.result) ? data.result : data;

  if (dmsSelectedDocId && !selectedDocId) dmsProcessDoc(activeDocId);

  document.getElementById('pattern-result').innerHTML =
    '<div class="pattern-lab-success">&#10003; Added literal rule: &quot;' + esc(pattern) + '&quot;</div>';
  document.getElementById('custom-pattern-input').value = '';
  if (typeof openPatternLab === 'function') {
    await openPatternLab(activeDocId);
  }
}

function esc(s) { return s ? s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;') : ''; }
function escFmt(s) {
  if (!s) return '';
  s = esc(s);
  s = s.replace(/(^\|.+$\n?)+/gm, function(block) {
    var rows = block.trim().split('\n').filter(function(r) { return r.trim(); });
    if (rows.length < 2) return block;
    var html = '<table class="report-table">';
    rows.forEach(function(row, i) {
      if (/^\|[\s-:|]+\|$/.test(row.trim())) return;
      var cells = row.split('|').filter(function(c) { return c.trim() !== ''; });
      var tag = i === 0 ? 'th' : 'td';
      html += '<tr>' + cells.map(function(c) { return '<' + tag + '>' + c.trim() + '</' + tag + '>'; }).join('') + '</tr>';
    });
    html += '</table>';
    return html;
  });
  s = s.replace(/^### (.+)$/gm, '<h3>$1</h3>');
  s = s.replace(/^## (.+)$/gm, '<h2>$1</h2>');
  s = s.replace(/^# (.+)$/gm, '<h1>$1</h1>');
  s = s.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');
  s = s.replace(/✅/g, '<span style="color:var(--success)">&#10003;</span>');
  s = s.replace(/❌/g, '<span style="color:var(--danger)">&#10007;</span>');
  s = s.replace(/⚠️/g, '<span style="color:var(--warning)">&#9888;</span>');
  s = s.replace(/^[-*] (.+)$/gm, '<li>$1</li>');
  s = s.replace(/\n\n/g, '<br><br>');
  s = s.replace(/\n/g, '<br>');
  return s;
}

async function refreshStatus() {
  try {
    const r = await fetch(API + '/api/status');
    const s = await r.json();
    const p = [];
    if (s.documentsProcessed > 0) p.push(s.documentsApproved + '/' + s.documentsProcessed + ' docs approved');
    if (s.gapsAgreed > 0) p.push(s.gapsAgreed + ' gaps agreed');
    if (s.reviewsCompleted > 0) p.push(s.reviewsCompleted + ' reviews');
    const el = document.getElementById('status-bar');
    if (el) el.textContent = p.join(' \u00b7 ');
  } catch(e) {}
}

// ═══════════ SCENARIO OVERVIEW NAVIGATION ═══════════

let compSelectedDocId = '';
let dataSelectedDocId = '';
