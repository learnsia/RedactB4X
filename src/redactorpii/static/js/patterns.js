// Simple "missed PII" flow: highlight text → redact exact or similar (uses /api/patterns/*)

var patternLabDocId = '';
var patternLabSample = '';
var patternLabCandidates = [];
var patternLabSelection = null; // { start, end, text }
var patternLabSimilarReady = false;
var patternLabSimilarCount = 0;
var patternLabExpression = '';
var patternLabPreviewTimer = null;

var patternLabRecognizerPriority = [
  'Person name (3+ words)', 'SSN', 'Date YYYY/MM/DD',
  'US phone number', 'Email', 'Number', 'Digit', 'Date', 'UUID',
  'IPv4 address', 'US ZIP code', 'Alphanumeric characters', 'Decimal number',
  'Multiple characters', 'Exact number', 'Character'
];

function patternLabActiveDocId() {
  return patternLabDocId || selectedDocId || compSelectedDocId || dataSelectedDocId || dmsSelectedDocId || '';
}

function patternLabAttr(s) {
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
    .replace(/</g, '&lt;');
}

async function openPatternLab(docId, pasteText) {
  if (!docId && !pasteText) docId = patternLabActiveDocId();
  if (!docId && !pasteText) {
    alert('No document selected.');
    return;
  }
  patternLabDocId = docId || '';
  window._pasteLabText = pasteText || '';
  patternLabSelection = null;
  patternLabSimilarReady = false;
  patternLabSimilarCount = 0;
  patternLabExpression = '';

  document.getElementById('pii-modal').style.display = 'flex';
  trapFocus(document.getElementById('pii-modal'));
  document.getElementById('pattern-result').innerHTML = '';
  patternLabHideSelectionPanel();
  patternLabSetIdleHint(true);

  var libLabel = document.getElementById('pii-modal-lib-label');
  var libBody = document.getElementById('pii-modal-lib-body');
  var libPreview = document.getElementById('pii-modal-lib-preview');
  if (libLabel) libLabel.value = '';
  if (libBody) libBody.value = '';
  if (libPreview) libPreview.textContent = '';

  var ta = document.getElementById('pattern-lab-sample');
  ta.disabled = true;

  if (pasteText) {
    patternLabSample = pasteText;
    ta.value = pasteText;
    ta.disabled = false;
    patternLabRenderMissedNames([]);
    patternLabSetIdleHint(true);
    return;
  }

  ta.value = 'Loading…';

  try {
    const [suggestRes, missedRes] = await Promise.all([
      fetch(API + '/api/patterns/suggest', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ docId: docId })
      }),
      fetch(API + '/api/documents/missed-names', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ docId: docId })
      })
    ]);
    const suggest = await suggestRes.json();
    const missed = await missedRes.json();
    if (!suggestRes.ok) throw new Error(suggest.error || 'Failed to load document');

    patternLabSample = suggest.sampleText || '';
    patternLabCandidates = suggest.candidates || [];
    ta.value = patternLabSample;
    ta.disabled = false;
    patternLabRenderMissedNames(missed.missedNames || []);
    patternLabSetIdleHint(true);
  } catch (e) {
    ta.value = '';
    ta.disabled = true;
    patternLabSetIdleHint(false);
    document.getElementById('pattern-result').innerHTML =
      '<div class="pattern-lab-error">' + esc(e.message) + '</div>';
  }
}

function piiModalLibAutoFill() {
  var bodyEl = document.getElementById('pii-modal-lib-body');
  var kindEl = document.getElementById('pii-modal-lib-kind');
  var labelEl = document.getElementById('pii-modal-lib-label');
  if (patternLabExpression) {
    bodyEl.value = patternLabExpression;
    kindEl.value = 'regex';
    if (!labelEl.value.trim()) labelEl.value = 'Custom regex pattern';
  } else if (patternLabSelection && patternLabSelection.text) {
    bodyEl.value = patternLabSelection.text;
    kindEl.value = 'literal';
    if (!labelEl.value.trim()) labelEl.value = 'Exact: ' + patternLabSelection.text.slice(0, 30);
  }
  piiModalLibPreview();
}

function piiModalLibPreview() {
  var body = document.getElementById('pii-modal-lib-body').value.trim();
  var kind = document.getElementById('pii-modal-lib-kind').value;
  var resultEl = document.getElementById('pii-modal-lib-preview');
  if (!body) {
    resultEl.textContent = '';
    return;
  }
  if (!patternLabSample) {
    resultEl.textContent = 'No document loaded.';
    return;
  }
  if (typeof patternLibraryRunTest === 'function') {
    patternLibraryRunTest(body, kind, patternLabSample, resultEl);
    return;
  }
  if (kind === 'literal') {
    var count = 0;
    var idx = 0;
    while ((idx = patternLabSample.indexOf(body, idx)) !== -1) { count++; idx += body.length; }
    resultEl.textContent = count + ' match' + (count === 1 ? '' : 'es') + ' in document.';
    return;
  }
  try {
    var re = new RegExp(body, 'gi');
    var matches = patternLabSample.match(re);
    var count = matches ? matches.length : 0;
    resultEl.textContent = count + ' match' + (count === 1 ? '' : 'es') + ' in document.';
    if (count > 0 && count <= 10) {
      resultEl.textContent += ' Found: ' + matches.slice(0, 5).join(', ') + (count > 5 ? '...' : '');
    }
  } catch(e) {
    resultEl.textContent = 'Invalid regex: ' + e.message;
  }
}

async function piiModalLibSave() {
  var label = document.getElementById('pii-modal-lib-label').value.trim();
  var kind = document.getElementById('pii-modal-lib-kind').value;
  var body = document.getElementById('pii-modal-lib-body').value.trim();
  if (!body) {
    alert('Enter the text or regex to save.');
    return;
  }
  if (!label) label = (kind === 'literal' ? 'Exact: ' : 'Regex: ') + body.slice(0, 30);
  try {
    await savePatternToLibrary(kind, body, label);
    document.getElementById('pii-modal-lib-label').value = '';
    document.getElementById('pii-modal-lib-body').value = '';
    document.getElementById('pii-modal-lib-preview').textContent = 'Saved!';
  } catch(e) {
    document.getElementById('pii-modal-lib-preview').textContent = 'Error: ' + e.message;
  }
}

function patternLabSetIdleHint(show) {
  var hint = document.getElementById('pattern-lab-idle-hint');
  if (hint) hint.style.display = show ? 'block' : 'none';
}

function patternLabHideSelectionPanel() {
  var panel = document.getElementById('pattern-lab-selection-panel');
  if (panel) panel.style.display = 'none';
}

function patternLabShowSelectionPanel(text) {
  patternLabSetIdleHint(false);
  var panel = document.getElementById('pattern-lab-selection-panel');
  panel.style.display = 'block';
  document.getElementById('pattern-lab-selected-text').textContent = text;
  var btnSimilar = document.getElementById('pattern-lab-btn-similar');
  btnSimilar.disabled = true;
  document.getElementById('pattern-lab-preview-summary').textContent = 'Checking how many matches…';
}

function patternLabOnTextSelect() {
  var ta = document.getElementById('pattern-lab-sample');
  if (!ta || ta.disabled) return;
  var start = ta.selectionStart;
  var end = ta.selectionEnd;
  if (start === end) {
    patternLabSelection = null;
    patternLabHideSelectionPanel();
    patternLabSetIdleHint(true);
    return;
  }
  var text = patternLabSample.slice(start, end);
  if (!text.trim()) {
    patternLabSelection = null;
    patternLabHideSelectionPanel();
    patternLabSetIdleHint(true);
    return;
  }
  patternLabSelection = { start: start, end: end, text: text };
  piiModalLibAutoFill();
  patternLabShowSelectionPanel(text);
  patternLabScheduleSimilarPreview();
}

function patternLabScheduleSimilarPreview() {
  if (patternLabPreviewTimer) clearTimeout(patternLabPreviewTimer);
  patternLabPreviewTimer = setTimeout(patternLabUpdateSimilarPreview, 200);
}

function patternLabPickByPriority(list) {
  if (!list.length) return null;
  for (var i = 0; i < patternLabRecognizerPriority.length; i++) {
    var name = patternLabRecognizerPriority[i];
    var hit = list.find(function(c) { return c.recognizer === name || c.title.indexOf(name) === 0; });
    if (hit) return hit;
  }
  return list.slice().sort(function(a, b) {
    return (b.end - b.start) - (a.end - a.start);
  })[0];
}

function patternLabBestCandidate(start, end, text) {
  var exact = patternLabCandidates.filter(function(c) {
    return c.start === start && c.end === end;
  });
  if (exact.length) return patternLabPickByPriority(exact);

  var byText = patternLabCandidates.filter(function(c) { return c.text === text; });
  if (byText.length) return patternLabPickByPriority(byText);

  // Recognizer span covers the whole highlight (e.g. multi-word person name)
  var covering = patternLabCandidates.filter(function(c) {
    return c.start <= start && c.end >= end;
  });
  if (covering.length) {
    covering.sort(function(a, b) { return (a.end - a.start) - (b.end - b.start); });
    return patternLabPickByPriority(covering);
  }

  // Return null for multi-word selections to avoid partial matching.
  if (/\s/.test(text)) return null;

  var inside = patternLabCandidates.filter(function(c) {
    return c.start >= start && c.end <= end && (c.end - c.start) >= Math.min(text.length, 2);
  });
  if (inside.length) return patternLabPickByPriority(inside);

  return null;
}

async function patternLabUpdateSimilarPreview() {
  if (!patternLabSelection || !patternLabDocId) return;
  var summary = document.getElementById('pattern-lab-preview-summary');
  var btnSimilar = document.getElementById('pattern-lab-btn-similar');
  var cand = patternLabBestCandidate(
    patternLabSelection.start,
    patternLabSelection.end,
    patternLabSelection.text
  );

  if (!cand) {
    patternLabSimilarReady = false;
    btnSimilar.disabled = true;
    summary.textContent = 'No general pattern detected for this text — use “Only this exact text”.';
    return;
  }

  try {
    var r = await fetch(API + '/api/patterns/preview', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        docId: patternLabDocId,
        sampleText: patternLabSample,
        selectedIds: [cand.id],
        options: { onlyPatterns: true, matchWholeLine: false, caseInsensitive: false }
      })
    });
    var data = await r.json();
    if (!r.ok) throw new Error(data.error || 'Preview failed');

    patternLabSimilarReady = true;
    patternLabSimilarCount = data.matchCount || 0;
    patternLabExpression = data.expression || '';
    btnSimilar.disabled = false;
    summary.textContent =
      '“Redact all similar” would match ' + patternLabSimilarCount +
      ' place' + (patternLabSimilarCount === 1 ? '' : 's') + ' in this document' +
      ' (as ' + cand.title.toLowerCase() + ').';
    var advExpr = document.getElementById('pattern-lab-expression');
    if (advExpr) advExpr.value = patternLabExpression;
  } catch (e) {
    patternLabSimilarReady = false;
    btnSimilar.disabled = true;
    summary.textContent = e.message;
  }
}

async function patternLabRedactExact() {
  if (!patternLabSelection) {
    patternLabToast('Highlight text in the document first.');
    return;
  }
  var docId = patternLabActiveDocId();
  var text = patternLabSelection.text;

  if (window._pasteLabText) {
    pasteApplyPattern('literal', text);
    return;
  }

  if (!docId) return;
  var r = await fetch(API + '/api/documents/add-pattern', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ docId: docId, pattern: text })
  });
  var data = await r.json();
  if (!r.ok) {
    patternLabToast(data.error || 'Save failed');
    return;
  }
  patternLabAfterReprocess(docId, parseAddPatternResponse(data));
  document.getElementById('pattern-result').innerHTML =
    '<div class="pattern-lab-success">&#10003; Exact text rule saved.</div>';
  await openPatternLab(docId);
}

async function patternLabRedactSimilar() {
  if (!patternLabSimilarReady || !patternLabExpression) {
    patternLabToast('Similar redaction is not available for this selection.');
    return;
  }
  var docId = patternLabActiveDocId();

  if (window._pasteLabText) {
    pasteApplyPattern('regex', patternLabExpression);
    return;
  }

  if (!docId) return;
  if (typeof promptSavePatternToLibrary === 'function') {
    var saved = await promptSavePatternToLibrary('regex', patternLabExpression, 'Custom pattern');
    if (!saved) return;
    var ids = getStoredPatternSelection().slice();
    if (ids.indexOf(saved.id) < 0) ids.push(saved.id);
    setStoredPatternSelection(ids);
    await dmsProcessDocWithPatterns(docId, dmsSelectedDocTitle || docId, ids);
    document.getElementById('pattern-result').innerHTML =
      '<div class="pattern-lab-success">&#10003; Saved to library and applied (' + patternLabSimilarCount + ' matches in preview).</div>';
    closeModal();
    return;
  }
  var r = await fetch(API + '/api/documents/add-pattern', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ docId: docId, kind: 'regex', expression: patternLabExpression })
  });
  var data = await r.json();
  if (!r.ok) {
    patternLabToast(data.error || 'Save failed');
    return;
  }
  patternLabAfterReprocess(docId, parseAddPatternResponse(data));
  document.getElementById('pattern-result').innerHTML =
    '<div class="pattern-lab-success">&#10003; Similar-text rule saved (' + patternLabSimilarCount + ' matches in preview).</div>';
  await openPatternLab(docId);
}

function patternLabReset() {
  patternLabDocId = '';
  patternLabSample = '';
  patternLabCandidates = [];
  patternLabSelection = null;
  patternLabSimilarReady = false;
  patternLabSimilarCount = 0;
  patternLabExpression = '';
  if (patternLabPreviewTimer) { clearTimeout(patternLabPreviewTimer); patternLabPreviewTimer = null; }
}

async function patternLabRedactLiteral(text) {
  if (!text) return;
  var docId = patternLabActiveDocId();
  if (!docId) return;
  var r = await fetch(API + '/api/documents/add-pattern', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ docId: docId, pattern: text })
  });
  var data = await r.json();
  if (!r.ok) {
    patternLabToast(data.error || 'Save failed');
    return;
  }
  patternLabAfterReprocess(docId, parseAddPatternResponse(data));
  document.getElementById('pattern-result').innerHTML =
    '<div class="pattern-lab-success">&#10003; Rule saved for “' + esc(text) + '”.</div>';
  await openPatternLab(docId);
}

function patternLabRenderMissedNames(names) {
  var el = document.getElementById('missed-names-list');
  if (!el) return;
  if (!names.length) {
    el.innerHTML = '<span class="pattern-lab-muted">None</span>';
    return;
  }
  el.innerHTML = names.map(function(n) {
    return '<button type="button" class="pattern-lab-name-chip" data-literal="' + patternLabAttr(n) +
      '" onclick="patternLabRedactLiteral(this.getAttribute(\'data-literal\'))">' + esc(n) + '</button>';
  }).join('');
}

async function patternLabSaveAdvancedRegex() {
  var expr = document.getElementById('pattern-lab-expression').value.trim();
  if (!expr) {
    patternLabToast('Enter a regex in advanced options.');
    return;
  }
  var docId = patternLabActiveDocId();
  if (!docId) return;
  var r = await fetch(API + '/api/documents/add-pattern', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ docId: docId, kind: 'regex', expression: expr })
  });
  var data = await r.json();
  if (!r.ok) {
    patternLabToast(data.error || 'Save failed');
    return;
  }
  patternLabAfterReprocess(docId, parseAddPatternResponse(data));
  document.getElementById('pattern-result').innerHTML =
    '<div class="pattern-lab-success">&#10003; Custom regex saved.</div>';
  await openPatternLab(docId);
}

async function patternLabAfterReprocess(docId, result) {
  if (dmsSelectedDocId && typeof dmsApplyProcessResult === 'function' && result) {
    dmsApplyProcessResult(result);
  } else if (compSelectedDocId && typeof showDetailResults === 'function') {
    showDetailResults('comp', result);
  } else if (dataSelectedDocId && typeof showDetailResults === 'function') {
    showDetailResults('data', result);
  }
}

async function patternLabRedactLiteral(text) {
  if (!text) return;
  var docId = patternLabActiveDocId();
  if (!docId) return;
  var r = await fetch(API + '/api/documents/add-pattern', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ docId: docId, pattern: text })
  });
  var data = await r.json();
  if (!r.ok) {
    patternLabToast(data.error || 'Save failed');
    return;
  }
  patternLabAfterReprocess(docId, parseAddPatternResponse(data));
  document.getElementById('pattern-result').innerHTML =
    '<div class="pattern-lab-success">&#10003; Rule saved for "' + esc(text) + '".</div>';
  await openPatternLab(docId);
}

function patternLabToast(msg) {
  if (typeof dmsShowToast === 'function') {
    dmsShowToast(esc(msg), 'info');
    return;
  }
  alert(msg);
}

// No-op stubs kept for backward compatibility.
function patternLabShowTab() {}
function patternLabClearSelection() {
  var ta = document.getElementById('pattern-lab-sample');
  if (ta) ta.setSelectionRange(0, 0);
  patternLabOnTextSelect();
}
function patternLabPreview() { patternLabUpdateSimilarPreview(); }
function patternLabSaveRegex() { patternLabSaveAdvancedRegex(); }
function patternLabUseLiteral(text) { patternLabRedactLiteral(text); }
