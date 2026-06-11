

// ═══════════ DOCUMENT UPLOAD ═══════════
async function uploadFiles(files) {
  const cat = document.getElementById('upload-category').value.trim() || 'general';
  const statusEl = document.getElementById('upload-status');
  let uploaded = 0;
  for (const file of files) {
    statusEl.textContent = 'Uploading ' + file.name + '...';
    const formData = new FormData();
    formData.append('file', file);
    formData.append('category', cat);
    try {
      const r = await fetch(API + '/api/documents/upload', { method: 'POST', body: formData });
      if (r.ok) uploaded++;
    } catch(e) {}
  }
  statusEl.textContent = uploaded + ' file(s) uploaded';
  if (typeof loadDocuments === 'function') loadDocuments();
  if (typeof refreshUploadedDocs === 'function') refreshUploadedDocs();
}

// ═══════════ DMS DASHBOARD ═══════════
let dmsSelectedDocId = '';
let dmsSelectedDocTitle = '';
let dmsSelectedDocIds = new Set();
let dmsLibPage = 1;
let dmsLibPageSize = 10;
let dmsLibSearchTimer = null;
let dmsLibLastTotal = 0;
let dmsLoadDocsTimer = null;
let dmsLoadDocsSeq = 0;

async function showDmsDashboard() {
  hideAllViews();
  document.getElementById('app-header').classList.remove('hidden');
  document.getElementById('dms-dashboard').classList.remove('hidden');
  document.getElementById('btn-back-dashboard').style.display = 'none';
  document.getElementById('btn-switch-demo').style.display = 'none';

  // Load config
  try {
    const r = await fetch(API + '/api/setup');
    const cfg = await r.json();
    if (cfg.company) {
      document.getElementById('dms-company-name').textContent = '';
      document.getElementById('header-company-name').textContent = '';
      document.title = cfg.company + ' \u2014 Document Management System';
      const fw = (cfg.complianceFramework || 'Custom') + ' ' + (cfg.complianceVersion || '');
      document.getElementById('dms-framework-badge').textContent = '';
      document.getElementById('dms-framework-name').textContent = cfg.complianceFramework || 'your framework';
      document.getElementById('header-subtitle').textContent = 'Document Management';
      currentFramework = cfg.complianceFramework || 'Custom';
      currentScenario = { id: 'custom', company: { name: cfg.company, industry: cfg.industry, description: cfg.description }, complianceFramework: cfg.complianceFramework, complianceVersion: cfg.complianceVersion };
    }
  } catch(e) {}

  await dmsResetRateLimit();
  dmsLoadDocs(true);

  // Set up drag and drop
  setTimeout(function() {
    const zone = document.getElementById('dms-drop-zone');
    if (zone && !zone._bound) {
      zone._bound = true;
      zone.addEventListener('dragover', function(e) { e.preventDefault(); zone.style.borderColor = 'var(--primary)'; zone.style.background = 'var(--primary-light)'; });
      zone.addEventListener('dragleave', function(e) { e.preventDefault(); zone.style.borderColor = 'var(--border)'; zone.style.background = ''; });
      zone.addEventListener('drop', function(e) {
        e.preventDefault();
        zone.style.borderColor = 'var(--border)';
        zone.style.background = '';
        if (e.dataTransfer.files.length > 0) dmsUploadFiles(e.dataTransfer.files);
      });
    }
  }, 100);
}

async function dmsUploadFiles(files) {
  const cat = document.getElementById('dms-upload-category').value.trim() || 'general';
  const folderEl = document.getElementById('dms-lib-folder');
  const folder = folderEl ? folderEl.value.trim() : '';
  const statusEl = document.getElementById('dms-upload-status');
  let uploaded = 0;
  const errors = [];
  for (const file of files) {
    statusEl.textContent = 'Uploading ' + file.name + '...';
    const formData = new FormData();
    formData.append('file', file);
    formData.append('category', cat);
    if (folder) formData.append('folder', folder);
    try {
      const r = await fetch(API + '/api/documents/upload', { method: 'POST', body: formData });
      let data = {};
      try { data = await r.json(); } catch(e) {}
      if (r.ok) {
        uploaded++;
      } else {
        errors.push(file.name + ': ' + ((data && data.error) ? data.error : ('HTTP ' + r.status)));
      }
    } catch(e) {
      errors.push(file.name + ': ' + e.message);
    }
  }
  if (errors.length) {
    statusEl.textContent = uploaded + ' uploaded, ' + errors.length + ' failed — ' + errors.slice(0, 2).join('; ');
    statusEl.title = errors.join('\n');
  } else {
    statusEl.textContent = uploaded + ' file(s) uploaded successfully';
    statusEl.removeAttribute('title');
  }
  dmsLibPage = 1;
  await dmsLoadDocs(true);
}

function dmsFormatScanStatus(data) {
  var parts = [];
  parts.push('Found ' + (data.found || 0) + ' file(s), added ' + (data.added || 0) + ' new');
  if (data.skipped) parts.push(data.skipped + ' duplicate(s) skipped');
  if (data.warnings && data.warnings.length) {
    parts.push(data.warnings.length + ' permission/read warning(s)');
  }
  return parts.join(' · ');
}

function dmsShowScanWarnings(data, statusEl) {
  if (!statusEl) return;
  statusEl.textContent = dmsFormatScanStatus(data);
  if (data.warnings && data.warnings.length) {
    var preview = data.warnings.slice(0, 3).join('; ');
    if (data.warnings.length > 3) preview += ' …';
    statusEl.title = data.warnings.join('\n');
    statusEl.textContent += ' — ' + preview;
  } else {
    statusEl.removeAttribute('title');
  }
}

async function dmsOpenScanModal() {
  var modal = document.getElementById('dms-scan-modal');
  var inp = document.getElementById('dms-scan-dir-input');
  var errEl = document.getElementById('dms-scan-error');
  if (!modal || !inp) return;
  if (errEl) errEl.style.display = 'none';
  inp.value = '';
  try {
    var r = await fetch(API + '/api/documents/scan');
    if (r.ok) {
      var cfg = await r.json();
      if (cfg.defaultDir) inp.value = cfg.defaultDir;
    }
  } catch(e) {}
  modal.style.display = 'flex';
  trapFocus(modal);
  inp.focus();
}

function dmsCloseScanModal() {
  var modal = document.getElementById('dms-scan-modal');
  if (modal) {
    releaseFocus(modal);
    modal.style.display = 'none';
  }
}

function dmsChooseScanFolder() {
  var inp = document.getElementById('dms-scan-folder-input');
  if (inp) {
    inp.value = '';
    inp.click();
  }
}

async function dmsSubmitScanDir() {
  var dirInp = document.getElementById('dms-scan-dir-input');
  var errEl = document.getElementById('dms-scan-error');
  var statusEl = document.getElementById('dms-upload-status');
  var dir = (dirInp && dirInp.value || '').trim();
  if (!dir) {
    if (errEl) {
      errEl.textContent = 'Enter a directory path on the server';
      errEl.style.display = 'block';
    }
    return;
  }
  if (errEl) errEl.style.display = 'none';
  dmsCloseScanModal();
  if (statusEl) statusEl.textContent = 'Scanning ' + dir + '...';
  try {
    var r = await fetch(API + '/api/documents/scan', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ dir: dir })
    });
    var data = await r.json();
    if (!r.ok) {
      if (statusEl) statusEl.textContent = (data && data.error) ? data.error : 'Scan failed';
      return;
    }
    dmsShowScanWarnings(data, statusEl);
    dmsLibPage = 1;
    await dmsLoadDocs(true);
  } catch(e) {
    if (statusEl) statusEl.textContent = 'Scan failed: ' + e.message;
  }
}

async function dmsImportSelectedFolder(fileList) {
  var statusEl = document.getElementById('dms-upload-status');
  if (!fileList || !fileList.length) return;
  dmsCloseScanModal();
  if (statusEl) statusEl.textContent = 'Reading ' + fileList.length + ' file(s) from selected folder...';
  var allowed = ['.txt', '.md', '.csv', '.json'];
  var docs = [];
  var readErrors = [];
  for (var i = 0; i < fileList.length; i++) {
    var file = fileList[i];
    var name = (file.webkitRelativePath || file.name || '').toLowerCase();
    var ext = name.lastIndexOf('.') >= 0 ? name.slice(name.lastIndexOf('.')) : '';
    if (allowed.indexOf(ext) === -1) continue;
    try {
      var content = await file.text();
      docs.push({
        sourceRelPath: file.webkitRelativePath || file.name,
        content: content
      });
    } catch(e) {
      readErrors.push((file.webkitRelativePath || file.name) + ': ' + e.message);
    }
  }
  if (!docs.length) {
    if (statusEl) statusEl.textContent = readErrors.length
      ? 'Could not read selected files: ' + readErrors.slice(0, 2).join('; ')
      : 'No supported files (.txt, .md, .csv, .json) in selected folder';
    return;
  }
  if (statusEl) statusEl.textContent = 'Importing ' + docs.length + ' file(s)...';
  try {
    var r = await fetch(API + '/api/documents/scan/import', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ documents: docs })
    });
    var data = await r.json();
    if (!r.ok) {
      if (statusEl) statusEl.textContent = (data && data.error) ? data.error : 'Import failed';
      return;
    }
    if (readErrors.length) {
      data.warnings = (data.warnings || []).concat(readErrors.map(function(m) { return 'browser read: ' + m; }));
    }
    dmsShowScanWarnings(data, statusEl);
    dmsLibPage = 1;
    await dmsLoadDocs(true);
  } catch(e) {
    if (statusEl) statusEl.textContent = 'Import failed: ' + e.message;
  }
}

async function dmsScanDir() {
  dmsOpenScanModal();
}

function dmsLibScheduleSearch() {
  if (dmsLibSearchTimer) clearTimeout(dmsLibSearchTimer);
  dmsLibSearchTimer = setTimeout(function() {
    dmsLibPage = 1;
    dmsLoadDocs(false);
  }, 350);
}

function dmsLibFolderChanged() {
  dmsLibPage = 1;
  dmsLoadDocs(false);
}

function dmsLibPageDelta(delta) {
  var maxPage = Math.max(1, Math.ceil(dmsLibLastTotal / dmsLibPageSize) || 1);
  var np = dmsLibPage + delta;
  if (np < 1 || np > maxPage) return;
  dmsLibPage = np;
  dmsLoadDocs(false);
}

async function dmsRefreshLibraryFolderSelect() {
  var sel = document.getElementById('dms-lib-folder');
  if (!sel) return;
  var prev = sel.value;
  try {
    var r = await fetch(API + '/api/library/folders');
    var data = await r.json();
    var folders = data.folders || [];
    sel.innerHTML = '<option value="">All folders</option>';
    folders.forEach(function(f) {
      var o = document.createElement('option');
      o.value = f;
      o.textContent = f;
      sel.appendChild(o);
    });
    if (prev && Array.prototype.some.call(sel.options, function(opt) { return opt.value === prev; })) {
      sel.value = prev;
    }
  } catch(e) {}
}

function dmsOpenNewFolderModal() {
  document.getElementById('dms-new-folder-error').style.display = 'none';
  document.getElementById('dms-new-folder-input').value = '';
  document.getElementById('dms-folder-modal').style.display = 'flex';
  trapFocus(document.getElementById('dms-folder-modal'));
}

function dmsCloseFolderModal() {
  releaseFocus(document.getElementById('dms-folder-modal'));
  document.getElementById('dms-folder-modal').style.display = 'none';
}

async function dmsSubmitNewFolder() {
  var inp = document.getElementById('dms-new-folder-input');
  var errEl = document.getElementById('dms-new-folder-error');
  var path = (inp.value || '').trim();
  if (!path) {
    errEl.textContent = 'Enter a folder path';
    errEl.style.display = 'block';
    return;
  }
  try {
    var r = await fetch(API + '/api/library/folders', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: path })
    });
    var data = await r.json();
    if (!r.ok) {
      errEl.textContent = (data && data.error) ? data.error : 'Request failed';
      errEl.style.display = 'block';
      return;
    }
    dmsCloseFolderModal();
    await dmsRefreshLibraryFolderSelect();
    var sel = document.getElementById('dms-lib-folder');
    if (sel && data.path) sel.value = data.path;
    dmsLibPage = 1;
    dmsLoadDocs(true);
  } catch(e) {
    errEl.textContent = e.message || 'Network error';
    errEl.style.display = 'block';
  }
}

async function dmsResetRateLimit() {
  try {
    await fetch(API + '/api/rate-limit/reset', { method: 'POST' });
  } catch(e) {}
}

async function dmsLoadDocs(refreshFolders) {
  if (refreshFolders === undefined) refreshFolders = false;
  if (dmsLoadDocsTimer) clearTimeout(dmsLoadDocsTimer);
  return new Promise(function(resolve) {
    dmsLoadDocsTimer = setTimeout(async function() {
      dmsLoadDocsTimer = null;
      await dmsLoadDocsNow(refreshFolders);
      resolve();
    }, refreshFolders ? 0 : 100);
  });
}

function bindDmsDocListActions() {
  var listEl = document.getElementById('dms-doc-list');
  if (!listEl || listEl._actionsBound) return;
  listEl._actionsBound = true;
  listEl.addEventListener('click', function(e) {
    var actionEl = e.target.closest('[data-dms-action]');
    if (!actionEl) return;
    e.preventDefault();
    e.stopPropagation();
    var action = actionEl.getAttribute('data-dms-action');
    var docId = actionEl.getAttribute('data-doc-id');
    if (action === 'toggle-dd') {
      dmsToggleDropdown(actionEl.getAttribute('data-dd-id'), e);
      return;
    }
    if (!docId) return;
    if (action === 'process') dmsProcessDoc(docId, actionEl.getAttribute('data-doc-title') || docId);
    else if (action === 'delete') dmsDeleteDoc(docId);
    else if (action === 'download-md') dmsDownloadDoc(docId, 'markdown');
    else if (action === 'download-orig') dmsDownloadDoc(docId, 'original');
    else if (action === 'download-redacted') dmsDownloadDoc(docId, 'redacted');
  });
}

function dmsCloseDetail() {
  closeDocProcessingModal();
}

async function dmsLoadDocsNow(refreshFolders) {
  var seq = ++dmsLoadDocsSeq;

  const listEl = document.getElementById('dms-doc-list');
  const countEl = document.getElementById('dms-doc-count');
  const batchEl = document.getElementById('dms-batch-actions');
  const pageInfo = document.getElementById('dms-lib-page-info');
  const prevBtn = document.getElementById('dms-lib-prev');
  const nextBtn = document.getElementById('dms-lib-next');
  if (!listEl) return;

  try {
    if (refreshFolders) {
      await dmsRefreshLibraryFolderSelect();
    }
    if (seq !== dmsLoadDocsSeq) return;

    var q = (document.getElementById('dms-lib-search') && document.getElementById('dms-lib-search').value) || '';
    var folder = (document.getElementById('dms-lib-folder') && document.getElementById('dms-lib-folder').value) || '';
    var qs = 'page=' + encodeURIComponent(String(dmsLibPage)) +
      '&pageSize=' + encodeURIComponent(String(dmsLibPageSize));
    if (q.trim()) qs += '&q=' + encodeURIComponent(q.trim());
    if (folder) qs += '&folder=' + encodeURIComponent(folder);

    const r = await fetch(API + '/api/library/documents?' + qs);
    const data = await r.json();
    if (r.status === 429) {
      await dmsResetRateLimit();
      listEl.innerHTML = '<div style="text-align:center;color:var(--danger);padding:1rem;font-size:0.85rem">Rate limit exceeded — refreshed automatically. Reload the page if the list is empty.</div>';
      return;
    }
    if (!r.ok) {
      listEl.innerHTML = '<div style="text-align:center;color:var(--danger);padding:1rem;font-size:0.85rem">' + esc(data.error || 'Failed to load') + '</div>';
      return;
    }
    if (seq !== dmsLoadDocsSeq) return;
    const docs = data.items || [];
    dmsLibLastTotal = typeof data.total === 'number' ? data.total : 0;
    if (countEl) countEl.textContent = dmsLibLastTotal + ' document' + (dmsLibLastTotal !== 1 ? 's' : '');

    if (dmsLibLastTotal === 0) {
      listEl.innerHTML = '<div style="text-align:center;color:var(--text-muted);padding:2rem;font-size:0.85rem">No documents match. Upload files, scan a directory, or adjust search/folder.</div>';
      if (batchEl) batchEl.style.display = 'none';
      if (pageInfo) pageInfo.textContent = '';
      if (prevBtn) prevBtn.disabled = true;
      if (nextBtn) nextBtn.disabled = true;
      return;
    }

    if (batchEl) batchEl.style.display = 'block';

    var maxPage = Math.max(1, Math.ceil(dmsLibLastTotal / dmsLibPageSize));
    if (pageInfo) {
      pageInfo.textContent = 'Page ' + data.page + ' of ' + maxPage + ' \u2014 showing ' + docs.length + ' of ' + dmsLibLastTotal;
    }
    if (prevBtn) prevBtn.disabled = data.page <= 1;
    if (nextBtn) nextBtn.disabled = data.page >= maxPage;

    var dropdownId = 0;
    listEl.innerHTML =
      '<table class="dms-doc-table">' +
        '<thead><tr>' +
          '<th style="width:2rem;text-align:center"><input type="checkbox" id="dms-select-all" onchange="dmsToggleSelectAll(this.checked)"></th>' +
          '<th>Name</th>' +
          '<th>Folder</th>' +
          '<th>Category</th>' +
          '<th>Status</th>' +
          '<th style="text-align:center">Review</th>' +
          '<th style="text-align:center">Actions</th>' +
          '<th style="text-align:center">Download</th>' +
        '</tr></thead>' +
        '<tbody>' +
    docs.map(function(d) {
      dropdownId++;

      var statusBadge = d.approved
        ? '<span class="dms-status-badge approved">Approved</span>'
        : d.processed
        ? '<span class="dms-status-badge processed">Processed</span>'
        : '<span class="dms-status-badge pending">Pending</span>';

      var reviewCell = (!d.processed && !d.approved)
        ? ''
        : (!d.approved && d.processed)
        ? '<span class="dms-review-badge">&#128270; Review</span>'
        : '<span style="color:var(--success);font-size:0.75rem">&#10003;</span>';

      var processLabel = !d.processed ? 'Process' : 'Re-Process';
      var processClass = !d.processed ? 'btn btn-primary btn-sm dms-action-btn' : 'btn btn-outline btn-sm dms-action-btn';
      var processBtn = '<button type="button" class="' + processClass + '" data-dms-action="process" data-doc-id="' + d.id + '" data-doc-title="' + escAttr(d.title) + '">' + processLabel + '</button>';
      var deleteBtn = '<button type="button" class="btn btn-ghost btn-sm dms-action-btn dms-delete-btn" data-dms-action="delete" data-doc-id="' + d.id + '" title="Delete">&#128465;</button>';

      var ddId = 'dms-dd-' + dropdownId;
      var downloadMenu =
        '<div class="dms-dropdown" id="' + ddId + '">' +
          '<button type="button" class="btn btn-ghost btn-sm dms-action-btn" style="color:var(--primary)" data-dms-action="toggle-dd" data-dd-id="' + ddId + '" title="Download">&#128229; &#9662;</button>' +
          '<div class="dms-dropdown-menu">' +
            '<button type="button" class="dms-dropdown-item" data-dms-action="download-md" data-doc-id="' + d.id + '">&#128196; Markdown</button>' +
            '<button type="button" class="dms-dropdown-item" data-dms-action="download-orig" data-doc-id="' + d.id + '">&#128228; Original</button>' +
            (d.processed ? '<button type="button" class="dms-dropdown-item" data-dms-action="download-redacted" data-doc-id="' + d.id + '">&#128274; Redacted</button>' : '') +
          '</div>' +
        '</div>';

      var folderText = d.folder || '—';
      var checked = dmsSelectedDocIds.has(d.id) ? ' checked' : '';

      return '<tr>' +
        '<td style="text-align:center"><input type="checkbox" class="dms-doc-checkbox" data-doc-id="' + d.id + '"' + checked + ' onchange="dmsToggleDocSelect(\'' + d.id + '\', this.checked)"></td>' +
        '<td class="dms-cell-name" title="' + esc(d.title) + '">' + esc(d.title) + '</td>' +
        '<td class="dms-cell-category" style="font-size:0.75rem;color:var(--text-muted)">' + esc(folderText) + '</td>' +
        '<td class="dms-cell-category"><span class="badge" style="font-size:0.6rem">' + esc(d.category) + '</span></td>' +
        '<td class="dms-cell-status">' + statusBadge + '</td>' +
        '<td class="dms-cell-status">' + reviewCell + '</td>' +
        '<td class="dms-cell-actions"><div class="dms-action-cell">' + processBtn + deleteBtn + '</div></td>' +
        '<td class="dms-cell-actions">' + downloadMenu + '</td>' +
      '</tr>';
    }).join('') +
        '</tbody></table>';

    var selAll = document.getElementById('dms-select-all');
    if (selAll) {
      var allIds = docs.map(function(d) { return d.id; });
      selAll.checked = allIds.length > 0 && allIds.every(function(id) { return dmsSelectedDocIds.has(id); });
    }
    dmsUpdateSelectionToolbar();
  } catch(e) {
    if (seq === dmsLoadDocsSeq) {
      listEl.innerHTML = '<div style="text-align:center;color:var(--danger);padding:1rem;font-size:0.85rem">Load error</div>';
    }
  }
}

function escAttr(s) {
  return String(s || '').replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/'/g, '&#39;').replace(/</g, '&lt;');
}

function dmsShowToast(message, type) {
  var host = document.getElementById('dms-toast-host');
  if (!host) return;
  var el = document.createElement('div');
  el.className = 'dms-toast' + (type ? ' ' + type : '');
  el.innerHTML = message;
  host.appendChild(el);
  setTimeout(function() {
    el.style.opacity = '0';
    el.style.transform = 'translateY(8px)';
    el.style.transition = 'opacity 0.25s ease, transform 0.25s ease';
    setTimeout(function() { el.remove(); }, 280);
  }, 5000);
}

function resetDocProcessingFooter() {
  var prompt = document.getElementById('modal-review-prompt');
  var actions = document.getElementById('modal-approve-actions');
  var success = document.getElementById('modal-approve-success');
  var approveBtn = document.getElementById('modal-btn-approve');
  if (prompt) prompt.style.display = '';
  if (actions) actions.style.display = 'flex';
  if (success) success.style.display = 'none';
  if (approveBtn) {
    approveBtn.disabled = false;
    approveBtn.innerHTML = '&#10003; Looks Correct &mdash; Approve &amp; Save';
  }
}

function showDocModalApprovedState(title) {
  var prompt = document.getElementById('modal-review-prompt');
  var actions = document.getElementById('modal-approve-actions');
  var success = document.getElementById('modal-approve-success');
  var detail = document.getElementById('modal-approve-success-detail');
  var status = document.getElementById('modal-detail-status');
  var modalTitle = document.getElementById('modal-detail-title');

  if (prompt) prompt.style.display = 'none';
  if (actions) actions.style.display = 'none';
  if (success) success.style.display = 'flex';
  if (detail) {
    detail.textContent = (title ? ('"' + title + '" ') : 'This document ') +
      'is saved in your library with Approved status. You can use it in Compliance Analysis and Data Analysis.';
  }
  if (status) {
    status.textContent = 'Approved & saved';
    status.className = 'badge badge-compliant';
  }
  if (modalTitle && title) {
    modalTitle.textContent = title;
  }
}

function setDocModalActionsEnabled(enabled) {
  var approve = document.getElementById('modal-btn-approve');
  var reject = document.getElementById('modal-btn-reject');
  if (approve) approve.disabled = false;
  if (reject) reject.disabled = !enabled;
}

function dmsApplyProcessResult(result) {
  if (!result || !document.getElementById('modal-redacted-preview')) return;
  document.getElementById('modal-original-preview').innerHTML = escFmt(result.originalText || '');
  document.getElementById('modal-redacted-preview').innerHTML = escFmt(result.redactedText || '');
  document.getElementById('modal-pii-summary').textContent = 'Found ' + (result.totalFound || 0) + ' PII items';
  document.getElementById('modal-pii-list').innerHTML = (result.items || []).map(function(item) {
    return '<li><span class="pii-type">' + esc(item.typeLabel) + '</span> ' +
      '<code>' + esc(item.original) + '</code> &rarr; <code>' + esc(item.redacted) + '</code></li>';
  }).join('');
  document.getElementById('modal-detail-status').textContent = (result.totalFound || 0) + ' PII Found';
  document.getElementById('modal-detail-status').className = 'badge badge-partial';
  setDocModalActionsEnabled(true);
}

async function dmsProcessDoc(docId, docTitle) {
  if (typeof openPatternPickerModal === 'function') {
    await openPatternPickerModal(docId, docTitle);
    return;
  }
  await dmsProcessDocWithPatterns(docId, docTitle, []);
}



function closeDocProcessingModal() {
  var copyBtn = document.getElementById('modal-btn-copy');
  var approveBtn = document.getElementById('modal-btn-approve');
  var rejectBtn = document.getElementById('modal-btn-reject');
  if (copyBtn) copyBtn.style.display = 'none';
  if (approveBtn) approveBtn.style.display = '';
  if (rejectBtn) rejectBtn.style.display = '';
  var modalTitle = document.getElementById('modal-detail-title');
  if (modalTitle) modalTitle.textContent = 'Document Processing';
  window._pasteRedactedText = '';

  releaseFocus(document.getElementById('doc-processing-modal'));
  document.getElementById('doc-processing-modal').style.display = 'none';
  document.body.style.overflow = '';
  resetDocProcessingFooter();
}

async function modalApproveDoc(ev) {
  if (ev && ev.stopPropagation) ev.stopPropagation();
  var docId = dmsSelectedDocId;
  if (!docId) {
    var btn = document.getElementById('modal-btn-approve');
    docId = btn ? btn.getAttribute('data-doc-id') : '';
  }
  if (!docId) {
    alert('No document selected. Close this dialog, click Process on a document, and try again.');
    return;
  }
  var approveBtn = document.getElementById('modal-btn-approve');
  if (approveBtn) {
    approveBtn.disabled = true;
    approveBtn.textContent = 'Saving approval…';
  }
  try {
    const r = await fetch(API + '/api/documents/approve', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ docId: docId })
    });
    if (r.status === 429) {
      await dmsResetRateLimit();
      alert('Rate limit was exceeded. Limits have been reset — click Approve again.');
      if (approveBtn) {
        approveBtn.disabled = false;
        approveBtn.innerHTML = '&#10003; Looks Correct &mdash; Approve &amp; Save';
      }
      return;
    }
    if (!r.ok) {
      const data = await r.json().catch(function() { return {}; });
      alert('Approve failed: ' + ((data && data.error) ? data.error : ('HTTP ' + r.status)));
      if (approveBtn) {
        approveBtn.disabled = false;
        approveBtn.innerHTML = '&#10003; Looks Correct &mdash; Approve &amp; Save';
      }
      return;
    }
    dmsSelectedDocId = docId;
    showDocModalApprovedState(dmsSelectedDocTitle);
    dmsShowToast('<strong>Document approved</strong><br>' + esc(dmsSelectedDocTitle) + ' is saved and ready for analysis.', 'success');
    await dmsLoadDocs(false);
  } catch(e) {
    alert('Approve failed: ' + e.message);
    if (approveBtn) {
      approveBtn.disabled = false;
      approveBtn.innerHTML = '&#10003; Looks Correct &mdash; Approve &amp; Save';
    }
  }
}

async function modalRejectDoc(ev) {
  if (ev && ev.stopPropagation) ev.stopPropagation();
  if (!dmsSelectedDocId) {
    var btn = document.getElementById('modal-btn-approve');
    dmsSelectedDocId = btn ? btn.getAttribute('data-doc-id') : '';
  }
  if (!dmsSelectedDocId) {
    if (window._pasteRedactedText) {
      var textarea = document.getElementById('paste-text-input');
      var originalText = textarea ? textarea.value.trim() : '';
      if (originalText) {
        closeModal();
        openPatternLab(null, originalText);
      }
      return;
    }
    alert('No document selected.');
    return;
  }
  try {
    document.getElementById('custom-pattern-input').value = '';
    document.getElementById('pattern-result').innerHTML = '';
    await openPatternLab(dmsSelectedDocId);
  } catch(e) { console.error('modalRejectDoc error:', e); }
}

async function dmsProcessAll() {
  const statusEl = document.getElementById('dms-batch-status');
  statusEl.textContent = 'Processing all documents...';
  try {
    // First do batch sample
    const sr = await fetch(API + '/api/batch/sample', { method: 'POST' });
    const sample = await sr.json();

    // Approve and run full batch
    await fetch(API + '/api/batch/approve', { method: 'POST' });
    const br = await fetch(API + '/api/batch/run', { method: 'POST' });
    const batch = await br.json();

    statusEl.textContent = 'Processed ' + (batch.totalDocs || 0) + ' documents, found ' + (batch.totalPII || 0) + ' PII items';
    await dmsLoadDocs(false);
  } catch(e) {
    statusEl.textContent = 'Error: ' + e.message;
  }
}

async function dmsDeleteDoc(docId) {
  if (!confirm('Delete this document?')) return;
  await fetch(API + '/api/documents/delete', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ docId: docId })
  });
  if (dmsSelectedDocId === docId) dmsCloseDetail();
  await dmsLoadDocs(true);
}

function dmsToggleDropdown(id, event) {
  if (event) {
    event.preventDefault();
    event.stopPropagation();
  }
  var dd = document.getElementById(id);
  if (!dd) return;
  var menu = dd.querySelector('.dms-dropdown-menu');
  var btn = dd.querySelector('button[data-dms-action="toggle-dd"]');
  if (!menu || !btn) return;
  var isOpen = menu.classList.contains('show');
  dmsCloseAllDropdowns();
  if (!isOpen) {
    var rect = btn.getBoundingClientRect();
    menu.style.top = (rect.bottom + 4) + 'px';
    menu.style.left = Math.max(8, rect.right - 168) + 'px';
    menu.classList.add('show');
  }
}

function dmsCloseAllDropdowns() {
  document.querySelectorAll('.dms-dropdown-menu.show').forEach(function(menu) {
    menu.classList.remove('show');
    menu.style.top = '';
    menu.style.left = '';
  });
}

document.addEventListener('click', function(e) {
  if (!e.target.closest('.dms-dropdown')) {
    dmsCloseAllDropdowns();
  }
});

function dmsDownloadDoc(docId, format) {
  dmsCloseAllDropdowns();
  var url = API + '/api/documents/download?id=' + encodeURIComponent(docId) + '&format=' + encodeURIComponent(format);
  var a = document.createElement('a');
  a.href = url;
  a.download = '';
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
}

function dmsToggleDocSelect(docId, checked) {
  if (checked) {
    dmsSelectedDocIds.add(docId);
  } else {
    dmsSelectedDocIds.delete(docId);
  }
  dmsUpdateSelectionToolbar();
}

function dmsToggleSelectAll(checked) {
  var checkboxes = document.querySelectorAll('.dms-doc-checkbox');
  checkboxes.forEach(function(cb) {
    var docId = cb.getAttribute('data-doc-id');
    cb.checked = checked;
    if (checked) {
      dmsSelectedDocIds.add(docId);
    } else {
      dmsSelectedDocIds.delete(docId);
    }
  });
  dmsUpdateSelectionToolbar();
}

function dmsClearSelection() {
  dmsSelectedDocIds.clear();
  var checkboxes = document.querySelectorAll('.dms-doc-checkbox');
  checkboxes.forEach(function(cb) { cb.checked = false; });
  var selectAll = document.getElementById('dms-select-all');
  if (selectAll) selectAll.checked = false;
  dmsUpdateSelectionToolbar();
}

function dmsUpdateSelectionToolbar() {
  var toolbar = document.getElementById('dms-selection-toolbar');
  var countEl = document.getElementById('dms-sel-count');
  if (!toolbar || !countEl) return;
  var n = dmsSelectedDocIds.size;
  if (n === 0) {
    toolbar.style.display = 'none';
  } else {
    toolbar.style.display = 'flex';
    countEl.textContent = n + ' document' + (n !== 1 ? 's' : '') + ' selected';
  }
}

async function dmsDownloadSelected(format) {
  var ids = Array.from(dmsSelectedDocIds);
  if (ids.length === 0) return;
  if (ids.length === 1) {
    dmsDownloadDoc(ids[0], format);
    return;
  }
  try {
    var r = await fetch(API + '/api/documents/download-batch', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ docIds: ids, format: format })
    });
    if (!r.ok) {
      var err = await r.json().catch(function() { return { error: 'Download failed' }; });
      dmsShowToast(err.error || 'Download failed', 'error');
      return;
    }
    var blob = await r.blob();
    var url = URL.createObjectURL(blob);
    var a = document.createElement('a');
    a.href = url;
    a.download = 'documents.zip';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  } catch(e) {
    dmsShowToast('Download error: ' + e.message, 'error');
  }
}

async function dmsUnprocessDoc(docId) {
  if (!confirm('Reset processing for this document? You will need to process it again.')) return;
  await fetch(API + '/api/documents/unprocess', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ docId: docId })
  });
  if (dmsSelectedDocId === docId) dmsCloseDetail();
  await dmsLoadDocs(false);
}

// --- Compliance walkthrough (DMS) ---
var compWalkStep = 1;


async function uploadFilesDetail(files, prefix) {
  const statusEl = document.getElementById(prefix + '-upload-status');
  let uploaded = 0;
  for (const file of files) {
    if (statusEl) statusEl.textContent = 'Uploading ' + file.name + '...';
    const formData = new FormData();
    formData.append('file', file);
    formData.append('category', 'general');
    try {
      const r = await fetch(API + '/api/documents/upload', { method: 'POST', body: formData });
      if (r.ok) uploaded++;
    } catch(e) {}
  }
  if (statusEl) statusEl.textContent = uploaded + ' file(s) uploaded';
  if (typeof refreshCompWalkDocSelect === 'function') refreshCompWalkDocSelect();
  else if (typeof refreshDataDocSelect === 'function') refreshDataDocSelect();
}

async function scanDocsDir() {
  if (typeof dmsOpenScanModal === 'function') {
    dmsOpenScanModal();
    return;
  }
  const statusEl = document.getElementById('upload-status') || document.getElementById('comp-upload-status');
  if (statusEl) statusEl.textContent = 'Scanning directory...';
  try {
    const r = await fetch(API + '/api/documents/scan', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({})
    });
    const data = await r.json();
    if (statusEl) {
      if (typeof dmsFormatScanStatus === 'function') {
        dmsShowScanWarnings(data, statusEl);
      } else {
        statusEl.textContent = 'Found ' + data.found + ' files, added ' + data.added + ' new';
      }
    }
    if (typeof refreshCompWalkDocSelect === 'function') refreshCompWalkDocSelect();
    else if (typeof refreshDataDocSelect === 'function') refreshDataDocSelect();
  } catch(e) {
    if (statusEl) statusEl.textContent = 'Scan failed: ' + e.message;
  }
}

async function refreshUploadedDocs() {
  try {
    const r = await fetch(API + '/api/documents');
    const docs = await r.json();
    const el = document.getElementById('uploaded-docs-list');
    if (!el) return;
    if (docs.length === 0) {
      el.innerHTML = '<span style="color:var(--text-muted)">No documents uploaded yet</span>';
    } else {
      el.innerHTML = docs.map(d =>
        '<div style="display:flex;justify-content:space-between;align-items:center;padding:0.25rem 0;border-bottom:1px solid var(--border-light)">' +
          '<span>' + esc(d.title) + ' <span class="badge" style="font-size:0.65rem">' + esc(d.category) + '</span></span>' +
          (d.approved ? '<span class="badge badge-compliant" style="font-size:0.65rem">Approved</span>' :
           d.processed ? '<span class="badge badge-partial" style="font-size:0.65rem">Processed</span>' :
           '<span class="badge" style="font-size:0.65rem">Pending</span>') +
        '</div>'
      ).join('');
    }
  } catch(e) {}
}

// Drag and drop
document.addEventListener('DOMContentLoaded', function() {
  const zone = document.getElementById('upload-drop-zone');
  if (zone) {
    zone.addEventListener('dragover', function(e) { e.preventDefault(); zone.style.borderColor = 'var(--primary)'; zone.style.background = 'var(--primary-light)'; });
    zone.addEventListener('dragleave', function(e) { e.preventDefault(); zone.style.borderColor = 'var(--border)'; zone.style.background = ''; });
    zone.addEventListener('drop', function(e) {
      e.preventDefault();
      zone.style.borderColor = 'var(--border)';
      zone.style.background = '';
      if (e.dataTransfer.files.length > 0) uploadFiles(e.dataTransfer.files);
    });
  }
  if (typeof wireComplianceWalkthroughOnce === 'function') wireComplianceWalkthroughOnce();
});

async function showBatchPanel() {
  document.getElementById('batch-panel').style.display = 'block';
  document.getElementById('batch-step-sample').style.display = 'none';
  document.getElementById('batch-step-progress').style.display = 'none';
  document.getElementById('batch-step-results').style.display = 'none';
  document.getElementById('batch-start-area').style.display = 'block';
}

async function runBatchSample() {
  document.getElementById('batch-start-area').style.display = 'none';
  document.getElementById('batch-step-sample').style.display = 'none';
  document.getElementById('batch-step-progress').style.display = 'block';
  document.getElementById('batch-step-results').style.display = 'none';
  document.getElementById('batch-progress-text').textContent = 'Processing sample documents...';
  document.getElementById('batch-progress-bar').style.width = '50%';
  document.getElementById('batch-progress-status').textContent = 'Applying redaction rules to all documents';

  const r = await fetch(API + '/api/batch/sample', { method: 'POST' });
  const data = await r.json();

  document.getElementById('batch-progress-bar').style.width = '100%';
  document.getElementById('batch-progress-text').textContent = 'Sample complete';

  setTimeout(() => {
    document.getElementById('batch-step-progress').style.display = 'none';
    document.getElementById('batch-step-sample').style.display = 'block';
    renderBatchSample(data);
  }, 500);
}

function renderBatchSample(data) {
  const tbody = document.getElementById('batch-sample-body');
  tbody.innerHTML = data.results.map(r =>
    '<tr>' +
      '<td><strong>' + r.title + '</strong></td>' +
      '<td>' + r.category + '</td>' +
      '<td>' + r.sampleChars.toLocaleString() + '</td>' +
      '<td><strong style="color:var(--danger)">' + r.piiFound + '</strong></td>' +
      '<td><div class="pii-chip-group">' + Object.entries(r.piiBreakdown).map(([k,v]) => '<span class="pii-chip">' + v + ' ' + k + '</span>').join('') + '</div></td>' +
    '</tr>'
  ).join('');

  // Stats
  const totalPII = data.results.reduce((sum, r) => sum + r.piiFound, 0);
  const totalChars = data.results.reduce((sum, r) => sum + r.sampleChars, 0);
  const allTypes = {};
  data.results.forEach(r => { Object.entries(r.piiBreakdown).forEach(([k,v]) => { allTypes[k] = (allTypes[k]||0) + v; }); });

  document.getElementById('batch-sample-stats').innerHTML =
    '<div class="stat-card"><div class="stat-value">' + data.totalDocs + '</div><div class="stat-label">Documents</div></div>' +
    '<div class="stat-card"><div class="stat-value">' + totalPII + '</div><div class="stat-label">Total PII Items</div></div>' +
    '<div class="stat-card"><div class="stat-value">' + totalChars.toLocaleString() + '</div><div class="stat-label">Total Characters</div></div>' +
    '<div class="stat-card"><div class="stat-value">' + Object.keys(allTypes).length + '</div><div class="stat-label">PII Categories</div></div>';

  // Show current patterns
  if (data.customPatterns && data.customPatterns.length > 0) {
    document.getElementById('batch-sample-stats').innerHTML +=
      '<div class="stat-card" style="grid-column:1/-1;text-align:left"><div class="stat-label" style="margin-bottom:0.5rem">Custom Redaction Rules Active</div>' +
      data.customPatterns.map(p => '<span class="pii-chip" style="background:var(--primary-light);color:var(--primary)">' + p + '</span>').join(' ') +
      '</div>';
  }
}

async function approveBatchSample() {
  await fetch(API + '/api/batch/approve', { method: 'POST' });

  document.getElementById('batch-step-sample').style.display = 'none';
  document.getElementById('batch-step-progress').style.display = 'block';
  document.getElementById('batch-progress-text').textContent = 'Running full batch...';
  document.getElementById('batch-progress-bar').style.width = '30%';
  document.getElementById('batch-progress-status').textContent = 'Processing all documents with confirmed rules';

  const r = await fetch(API + '/api/batch/run', { method: 'POST' });
  const data = await r.json();

  document.getElementById('batch-progress-bar').style.width = '100%';
  document.getElementById('batch-progress-text').textContent = 'Batch complete!';

  setTimeout(() => {
    document.getElementById('batch-step-progress').style.display = 'none';
    document.getElementById('batch-step-results').style.display = 'block';
    renderBatchResults(data);
  }, 500);
}

function renderBatchResults(data) {
  const tbody = document.getElementById('batch-results-body');
  tbody.innerHTML = data.results.map(r =>
    '<tr>' +
      '<td><strong>' + r.title + '</strong></td>' +
      '<td>' + r.category + '</td>' +
      '<td><strong style="color:var(--danger)">' + r.piiFound + '</strong></td>' +
      '<td><div class="pii-chip-group">' + Object.entries(r.piiBreakdown).map(([k,v]) => '<span class="pii-chip">' + v + ' ' + k + '</span>').join('') + '</div></td>' +
      '<td><span style="color:var(--success);font-weight:600">&#10003; Processed</span></td>' +
    '</tr>'
  ).join('');

  const allTypes = {};
  data.results.forEach(r => { Object.entries(r.piiBreakdown).forEach(([k,v]) => { allTypes[k] = (allTypes[k]||0) + v; }); });

  document.getElementById('batch-results-stats').innerHTML =
    '<div class="stat-card"><div class="stat-value">' + data.totalDocs + '</div><div class="stat-label">Documents Processed</div></div>' +
    '<div class="stat-card"><div class="stat-value" style="color:var(--danger)">' + data.totalPII + '</div><div class="stat-label">Total PII Redacted</div></div>' +
    Object.entries(allTypes).map(([k,v]) =>
      '<div class="stat-card"><div class="stat-value">' + v + '</div><div class="stat-label">' + k + '</div></div>'
    ).join('');

  loadDocuments();
  refreshStatus();
}

bindDmsDocListActions();

// ═══════════ PASTE & REDACT ═══════════

async function pasteRedactText() {
  var textarea = document.getElementById('paste-text-input');
  var statusEl = document.getElementById('paste-status');
  var text = textarea.value.trim();
  if (!text) {
    statusEl.textContent = 'Paste some text first.';
    statusEl.style.color = 'var(--danger)';
    return;
  }
  statusEl.textContent = 'Redacting...';
  statusEl.style.color = 'var(--text-muted)';
  try {
    var r = await fetch(API + '/api/redact/text', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ text: text })
    });
    if (!r.ok) {
      var err = await r.json().catch(function() { return { error: 'Redaction failed' }; });
      statusEl.textContent = err.error || 'Redaction failed';
      statusEl.style.color = 'var(--danger)';
      return;
    }
    var result = await r.json();
    statusEl.textContent = 'Found ' + (result.totalFound || 0) + ' PII items';
    statusEl.style.color = (result.totalFound || 0) > 0 ? 'var(--warning)' : 'var(--success)';
    showPasteResultModal(result);
  } catch (e) {
    statusEl.textContent = 'Error: ' + e.message;
    statusEl.style.color = 'var(--danger)';
  }
}

function showPasteResultModal(result) {
  dmsApplyProcessResult(result);
  var modalTitle = document.getElementById('modal-detail-title');
  if (modalTitle) modalTitle.textContent = 'Paste & Redact Results';

  var approveBtn = document.getElementById('modal-btn-approve');
  var rejectBtn = document.getElementById('modal-btn-reject');
  var copyBtn = document.getElementById('modal-btn-copy');
  if (approveBtn) approveBtn.style.display = 'none';
  if (rejectBtn) { rejectBtn.style.display = ''; rejectBtn.disabled = false; }
  if (copyBtn) { copyBtn.style.display = ''; copyBtn.innerHTML = '&#128203; Copy Redacted Text'; copyBtn.disabled = false; }

  window._pasteRedactedText = result.redactedText || '';

  document.getElementById('doc-processing-modal').style.display = 'flex';
  trapFocus(document.getElementById('doc-processing-modal'));
  document.body.style.overflow = 'hidden';
}

async function pasteCopyRedacted() {
  var text = window._pasteRedactedText || '';
  if (!text) return;
  var btn = document.getElementById('modal-btn-copy');
  try {
    await navigator.clipboard.writeText(text);
    if (btn) { btn.innerHTML = '&#10003; Copied!'; btn.disabled = true; }
    setTimeout(function() {
      if (btn) { btn.innerHTML = '&#128203; Copy Redacted Text'; btn.disabled = false; }
    }, 2000);
  } catch (e) {
    var ta = document.createElement('textarea');
    ta.value = text;
    ta.style.position = 'fixed';
    ta.style.opacity = '0';
    document.body.appendChild(ta);
    ta.select();
    document.execCommand('copy');
    document.body.removeChild(ta);
    if (btn) { btn.innerHTML = '&#10003; Copied!'; btn.disabled = true; }
    setTimeout(function() {
      if (btn) { btn.innerHTML = '&#128203; Copy Redacted Text'; btn.disabled = false; }
    }, 2000);
  }
}

function pasteApplyPattern(kind, value) {
  var textarea = document.getElementById('paste-text-input');
  var originalText = textarea ? textarea.value.trim() : '';
  if (!originalText) return;

  var statusEl = document.getElementById('paste-status');
  if (statusEl) {
    statusEl.textContent = 'Applying pattern...';
    statusEl.style.color = 'var(--text-muted)';
  }

  fetch(API + '/api/redact/text', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ text: originalText })
  }).then(function(r) { return r.json(); }).then(function(result) {
    if (kind === 'regex') {
      try {
        var re = new RegExp(value, 'gi');
        var m;
        while ((m = re.exec(originalText)) !== null) {
          result.items.push({ id: '[CUSTOM]', type: 'CUSTOM', typeLabel: 'Custom regex', original: m[0], redacted: '[REDACTED]', start: m.index, end: m.index + m[0].length, context: originalText.substring(Math.max(0, m.index - 20), Math.min(originalText.length, m.index + m[0].length + 20)) });
        }
        result.redactedText = result.items.filter(function(i) { return i.type === 'CUSTOM'; }).reduce(function(text, item) { return text.split(item.original).join(item.redacted); }, result.redactedText);
        result.totalFound = result.items.length;
      } catch(e) {
        alert('Invalid regex: ' + e.message);
        if (statusEl) statusEl.textContent = '';
        return;
      }
    } else {
      var count = 0;
      var idx = originalText.indexOf(value);
      while (idx !== -1) {
        result.items.push({ id: '[CUSTOM]', type: 'CUSTOM', typeLabel: 'Custom text', original: value, redacted: '[REDACTED]', start: idx, end: idx + value.length, context: originalText.substring(Math.max(0, idx - 20), Math.min(originalText.length, idx + value.length + 20)) });
        count++;
        idx = originalText.indexOf(value, idx + 1);
      }
      if (count > 0) {
        result.redactedText = result.redactedText.split(value).join('[REDACTED]');
        result.totalFound = result.items.length;
      }
    }

    window._pasteRedactedText = result.redactedText;
    showPasteResultModal(result);
    closeModal();
    if (statusEl) {
      statusEl.textContent = 'Found ' + (result.totalFound || 0) + ' PII items';
      statusEl.style.color = (result.totalFound || 0) > 0 ? 'var(--warning)' : 'var(--success)';
    }
  }).catch(function(e) {
    if (statusEl) {
      statusEl.textContent = 'Error: ' + e.message;
      statusEl.style.color = 'var(--danger)';
    }
  });
}
