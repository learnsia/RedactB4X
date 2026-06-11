// Saved pattern library — persist rules and apply selected ones per document

var patternLibraryCache = [];
var patternPickerDocId = '';
var patternPickerDocTitle = '';

var PATTERN_SELECTION_KEY = 'dmsSelectedPatternIds';

function parseAddPatternResponse(data) {
  return (data && data.result) ? data.result : data;
}

function getStoredPatternSelection() {
  try {
    var raw = localStorage.getItem(PATTERN_SELECTION_KEY);
    if (!raw) return [];
    var arr = JSON.parse(raw);
    return Array.isArray(arr) ? arr : [];
  } catch (e) {
    return [];
  }
}

function setStoredPatternSelection(ids) {
  localStorage.setItem(PATTERN_SELECTION_KEY, JSON.stringify(ids || []));
}

async function fetchPatternLibrary() {
  var r = await fetch(API + '/api/patterns/library');
  var data = await r.json();
  if (!r.ok) throw new Error(data.error || 'Failed to load pattern library');
  patternLibraryCache = data.patterns || [];
  return patternLibraryCache;
}

function formatPatternSummary(p) {
  if (p.kind === 'regex') {
    var expr = p.pattern || '';
    if (expr.length > 56) expr = expr.slice(0, 55) + '…';
    return expr;
  }
  var lit = p.pattern || '';
  if (lit.length > 40) lit = lit.slice(0, 39) + '…';
  return lit;
}

async function openPatternLibraryModal() {
  document.getElementById('pattern-library-modal').style.display = 'flex';
  trapFocus(document.getElementById('pattern-library-modal'));
  var picker = document.getElementById('pattern-picker-modal');
  if (picker && picker.style.display !== 'none') {
    document.getElementById('pattern-library-modal').dataset.fromPicker = '1';
  }
  document.getElementById('pattern-library-preview-result').textContent = '';
  await patternLibraryRenderList();
}

async function openPatternLibraryModalSidebar() {
  document.getElementById('pattern-library-modal').style.display = 'flex';
  trapFocus(document.getElementById('pattern-library-modal'));
  delete document.getElementById('pattern-library-modal').dataset.fromPicker;
  document.getElementById('pattern-library-preview-result').textContent = '';
  await patternLibraryRenderList();
}

function patternLibraryPreview() {
  var bodyEl = document.getElementById('pattern-library-new-body');
  if (!bodyEl) return;
  var body = bodyEl.value.trim();
  var kind = document.getElementById('pattern-library-new-kind').value;
  var resultEl = document.getElementById('pattern-library-preview-result');
  if (!body) {
    resultEl.textContent = '';
    return;
  }
  var sample = typeof patternLabSample !== 'undefined' ? patternLabSample : '';
  if (!sample) {
    resultEl.innerHTML = '<em>No document loaded.</em> <button type="button" class="btn btn-ghost btn-sm" style="padding:0.1rem 0.4rem;font-size:0.75rem" onclick="patternLibraryPromptSampleText()">Enter sample text</button>';
    return;
  }
  patternLibraryRunTest(body, kind, sample, resultEl);
}

function patternLibraryRunTest(body, kind, sample, resultEl) {
  if (kind === 'literal') {
    var count = 0;
    var idx = 0;
    while ((idx = sample.indexOf(body, idx)) !== -1) { count++; idx += body.length; }
    resultEl.textContent = count + ' match' + (count === 1 ? '' : 'es') + ' found in text.';
    return;
  }
  try {
    var re = new RegExp(body, 'gi');
    var matches = sample.match(re);
    var count = matches ? matches.length : 0;
    resultEl.textContent = count + ' match' + (count === 1 ? '' : 'es') + ' found in text.';
    if (count > 0 && count <= 10) {
      resultEl.textContent += ' Found: ' + matches.slice(0, 5).join(', ') + (count > 5 ? '...' : '');
    }
  } catch(e) {
    resultEl.textContent = 'Invalid regex: ' + e.message;
  }
}

function patternLibraryPromptSampleText() {
  var text = window.prompt('Paste or type some sample text to test against:', '');
  if (text === null || !text.trim()) return;
  patternLabSample = text;
  var bodyEl = document.getElementById('pattern-library-new-body');
  if (bodyEl && bodyEl.value.trim()) {
    patternLibraryPreview();
  }
}

function closePatternLibraryModal() {
  var modal = document.getElementById('pattern-library-modal');
  releaseFocus(modal);
  modal.style.display = 'none';
  document.getElementById('pattern-library-preview-result').textContent = '';
  var fromPicker = modal.dataset.fromPicker;
  delete modal.dataset.fromPicker;
  if (fromPicker) {
    patternPickerRefreshList();
  }
}

async function patternLibraryRenderList() {
  var el = document.getElementById('pattern-library-list');
  el.innerHTML = '<div class="pattern-lab-loading">Loading…</div>';
  try {
    var patterns = await fetchPatternLibrary();
    if (!patterns.length) {
      el.innerHTML = '<div class="pattern-lab-muted">No saved rules yet. Create one from the document view or add below.</div>';
      return;
    }
    el.innerHTML = patterns.map(function(p) {
      return '<div class="pattern-library-row">' +
        '<div class="pattern-library-row-main">' +
        '<div class="pattern-library-row-label">' + esc(p.label) + '</div>' +
        '<div class="pattern-library-row-meta">' +
        '<span class="badge badge-partial">' + esc(p.kind) + '</span> ' +
        '<code>' + esc(formatPatternSummary(p)) + '</code>' +
        '</div></div>' +
        '<button type="button" class="btn btn-ghost btn-sm" data-id="' + patternLabAttr(p.id) +
        '" onclick="patternLibraryDelete(this.getAttribute(\'data-id\'))">Delete</button></div>';
    }).join('');
  } catch (e) {
    el.innerHTML = '<div class="pattern-lab-error">' + esc(e.message) + '</div>';
  }
}

async function patternLibraryDelete(id) {
  if (!confirm('Delete this saved pattern?')) return;
  var r = await fetch(API + '/api/patterns/library?id=' + encodeURIComponent(id), { method: 'DELETE' });
  var data = await r.json();
  if (!r.ok) {
    alert(data.error || 'Delete failed');
    return;
  }
  await patternLibraryRenderList();
  if (typeof dmsShowToast === 'function') {
    dmsShowToast('Pattern deleted.', 'info');
  }
}

async function patternLibrarySubmitNew() {
  var label = document.getElementById('pattern-library-new-label').value.trim();
  var kind = document.getElementById('pattern-library-new-kind').value;
  var body = document.getElementById('pattern-library-new-body').value.trim();
  if (!body) {
    alert('Enter the text or regex to save.');
    return;
  }
  var payload = { label: label, kind: kind };
  if (kind === 'regex') payload.expression = body;
  else payload.pattern = body;
  var r = await fetch(API + '/api/patterns/library', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload)
  });
  var data = await r.json();
  if (!r.ok) {
    alert(data.error || 'Save failed');
    return;
  }
  document.getElementById('pattern-library-new-label').value = '';
  document.getElementById('pattern-library-new-body').value = '';
  document.getElementById('pattern-library-preview-result').textContent = '';
  await patternLibraryRenderList();
}

async function openPatternPickerModal(docId, docTitle) {
  patternPickerDocId = docId;
  patternPickerDocTitle = docTitle || docId;
  document.getElementById('pattern-picker-title').textContent = docTitle || docId;
  document.getElementById('pattern-picker-modal').style.display = 'flex';
  trapFocus(document.getElementById('pattern-picker-modal'));
  await patternPickerRefreshList();
}

async function patternPickerRefreshList() {
  var list = document.getElementById('pattern-picker-list');
  list.innerHTML = '<div class="pattern-lab-loading">Loading saved patterns…</div>';

  try {
    var patterns = await fetchPatternLibrary();
    var stored = getStoredPatternSelection();
    if (!patterns.length) {
      list.innerHTML = '<div class="pattern-lab-muted">No saved rules yet. Continue with automatic detection.</div>';
      document.getElementById('pattern-picker-run').textContent = 'Process';
      return;
    }
    document.getElementById('pattern-picker-run').textContent = 'Process document';
    list.innerHTML = patterns.map(function(p) {
      var checked = stored.indexOf(p.id) >= 0 ? ' checked' : '';
      return '<label class="pattern-picker-item">' +
        '<input type="checkbox" class="pattern-picker-cb" value="' + esc(p.id) + '"' + checked + ' />' +
        '<span class="pattern-picker-item-body">' +
        '<span class="pattern-picker-item-label">' + esc(p.label) + '</span>' +
        '<code class="pattern-picker-item-code">' + esc(formatPatternSummary(p)) + '</code>' +
        '<span class="badge badge-partial" style="margin-left:0.35rem">' + esc(p.kind) + '</span>' +
        '</span></label>';
    }).join('');
  } catch (e) {
    list.innerHTML = '<div class="pattern-lab-error">' + esc(e.message) + '</div>';
  }
}

function closePatternPickerModal() {
  releaseFocus(document.getElementById('pattern-picker-modal'));
  releaseFocus(document.getElementById('pattern-library-modal'));
  document.getElementById('pattern-picker-modal').style.display = 'none';
  document.getElementById('pattern-library-modal').style.display = 'none';
  patternPickerDocId = '';
}

function patternPickerSelectedIds() {
  var ids = [];
  document.querySelectorAll('.pattern-picker-cb:checked').forEach(function(cb) {
    ids.push(cb.value);
  });
  return ids;
}

async function patternPickerRunProcess() {
  var docId = patternPickerDocId;
  if (!docId) return;
  var ids = patternPickerSelectedIds();
  setStoredPatternSelection(ids);
  closePatternPickerModal();
  await dmsProcessDocWithPatterns(docId, patternPickerDocTitle, ids);
}

async function dmsProcessDocWithPatterns(docId, docTitle, libraryPatternIds) {
  dmsSelectedDocId = docId;
  dmsSelectedDocTitle = docTitle || docId;
  resetDocProcessingFooter();
  var approveBtn = document.getElementById('modal-btn-approve');
  if (approveBtn) {
    approveBtn.setAttribute('data-doc-id', docId);
    approveBtn.disabled = false;
  }
  document.getElementById('doc-processing-modal').style.display = 'flex';
  trapFocus(document.getElementById('doc-processing-modal'));
  document.body.style.overflow = 'hidden';
  setDocModalActionsEnabled(false);
  document.getElementById('modal-detail-status').textContent = 'Processing...';
  document.getElementById('modal-detail-status').className = 'badge badge-partial';
  var title = dmsSelectedDocTitle;
  if (libraryPatternIds && libraryPatternIds.length) {
    title += ' (' + libraryPatternIds.length + ' custom rule' + (libraryPatternIds.length === 1 ? '' : 's') + ')';
  }
  document.getElementById('modal-detail-title').textContent = title;

  try {
    var r = await fetch(API + '/api/documents/process', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ docId: docId, libraryPatternIds: libraryPatternIds || [] })
    });
    var data = await r.json();
    if (!r.ok) {
      document.getElementById('modal-detail-status').textContent = (data && data.error) ? data.error : 'Process failed';
      document.getElementById('modal-detail-status').className = 'badge badge-critical';
      setDocModalActionsEnabled(true);
      return;
    }
    dmsApplyProcessResult(data);
  } catch (e) {
    document.getElementById('modal-detail-status').textContent = 'Error';
    document.getElementById('modal-detail-status').className = 'badge badge-critical';
    setDocModalActionsEnabled(true);
  }
}

async function savePatternToLibrary(kind, expression, label) {
  var r = await fetch(API + '/api/patterns/library', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ kind: kind, expression: expression, label: label })
  });
  var data = await r.json();
  if (!r.ok) throw new Error(data.error || 'Save failed');
  await fetchPatternLibrary();
  return data.pattern;
}

async function promptSavePatternToLibrary(kind, expression, defaultLabel) {
  var label = window.prompt('Name this rule for the library:', defaultLabel || '');
  if (label === null) return null;
  try {
    return await savePatternToLibrary(kind, expression, label.trim());
  } catch (e) {
    if (typeof patternLabToast === 'function') patternLabToast(e.message);
    else alert(e.message);
    return null;
  }
}
