// QRDrop sender page: pick or drop files, upload them with live progress.
// Vanilla JS + XMLHttpRequest (for upload progress), no framework.
(function () {
  'use strict';

  var I18N = window.QRDropI18n;

  var EXT_GROUPS = [
    { label: 'PPT', exts: ['ppt', 'pptx'] },
    { label: 'Word', exts: ['doc', 'docx'] },
    { label: 'Excel', exts: ['xls', 'xlsx'] },
    { label: 'PDF', exts: ['pdf'] },
    { zh: '图片', en: 'images', exts: ['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'svg'] },
    { zh: '压缩包', en: 'archives', exts: ['zip', 'rar', '7z'] },
    { zh: '文本', en: 'text', exts: ['txt', 'md', 'csv'] },
    { zh: '视频', en: 'video', exts: ['mp4', 'mov'] },
    { zh: '音频', en: 'audio', exts: ['mp3'] }
  ];

  var el = {};
  var state = {
    token: '',
    allowed: [],
    maxMB: 1024,
    expiresAt: 0,
    status: 'loading', // loading | ready | ended | invalid
    queue: [],
    uploading: false,
    seq: 0,
    tickTimer: null
  };

  function $(id) {
    return document.getElementById(id);
  }

  function api(path) {
    return fetch(path).then(function (res) {
      return res.text().then(function (body) {
        var data = {};
        try {
          data = body ? JSON.parse(body) : {};
        } catch (e) {
          data = {};
        }
        if (!res.ok) {
          var err = new Error(data.error || res.statusText);
          err.status = res.status;
          throw err;
        }
        return data;
      });
    });
  }

  // ---------- boot ----------

  function init() {
    el.subtitle = $('subtitle');
    el.dropZone = $('dropZone');
    el.fileInput = $('fileInput');
    el.uploadBtn = $('uploadBtn');
    el.queue = $('queue');
    el.foot = $('footNote');
    el.notice = $('notice');
    el.overall = $('overall');
    el.overallText = $('overallText');
    el.overallPct = $('overallPct');
    el.overallFill = $('overallFill');

    state.token = new URLSearchParams(window.location.search).get('session') || '';

    el.dropZone.addEventListener('click', function () {
      if (state.status === 'ready') {
        el.fileInput.click();
      }
    });
    el.dropZone.addEventListener('keydown', function (event) {
      if (event.key === 'Enter' || event.key === ' ') {
        event.preventDefault();
        el.dropZone.click();
      }
    });
    el.fileInput.addEventListener('change', function () {
      addFiles(el.fileInput.files);
      el.fileInput.value = '';
    });
    el.uploadBtn.addEventListener('click', onUploadClick);
    document.addEventListener('qrdrop:langchange', renderDynamic);
    bindDragAndDrop();

    if (!state.token) {
      markInvalid();
      return;
    }

    api('/api/files?session=' + encodeURIComponent(state.token))
      .then(function (data) {
        state.allowed = data.allowed_exts || [];
        state.maxMB = data.max_upload_mb || 1024;
        state.expiresAt = data.expires_at || 0;
        state.status = 'ready';
        renderDynamic();
        startTick();
      })
      .catch(function (err) {
        if (err && (err.status === 404 || err.status === 410)) {
          markInvalid(I18N.t('sessionInvalid'));
          return;
        }
        if (err && err.status === undefined) {
          // Reached the server but the network call itself failed — usually the
          // phone is on a different WiFi / cellular, so the LAN IP is unreachable.
          markInvalid(I18N.t('uploadNetError'));
          return;
        }
        markInvalid(I18N.t('loadFailed'));
      });
  }

  function bindDragAndDrop() {
    ['dragenter', 'dragover'].forEach(function (type) {
      el.dropZone.addEventListener(type, function (event) {
        event.preventDefault();
        el.dropZone.classList.add('is-over');
      });
    });
    ['dragleave', 'dragend'].forEach(function (type) {
      el.dropZone.addEventListener(type, function () {
        el.dropZone.classList.remove('is-over');
      });
    });
    el.dropZone.addEventListener('drop', function (event) {
      event.preventDefault();
      el.dropZone.classList.remove('is-over');
      if (state.status !== 'ready') {
        return;
      }
      if (event.dataTransfer && event.dataTransfer.files) {
        addFiles(event.dataTransfer.files);
      }
    });
  }

  // ---------- queue ----------

  function addFiles(fileList) {
    var files = Array.prototype.slice.call(fileList || []);
    if (!files.length || state.status !== 'ready') {
      return;
    }

    files.forEach(function (file) {
      var problem = validate(file);
      state.queue.push({
        id: (state.seq += 1),
        file: file,
        name: file.name,
        size: file.size,
        status: problem ? 'error' : 'ready',
        progress: 0,
        message: problem
      });
    });

    renderQueue();
    updateButton();
    renderOverall();
    if (files.length > 1) {
      tip(I18N.t('addedFiles', { n: files.length }));
    }
    runQueue();
  }

  function validate(file) {
    var ext = file.name.indexOf('.') >= 0 ? file.name.split('.').pop().toLowerCase() : '';
    if (state.allowed.length && state.allowed.indexOf(ext) === -1) {
      return I18N.t('typeNotAllowed');
    }
    if (file.size === 0) {
      return I18N.t('emptyFile');
    }
    if (file.size > state.maxMB * 1024 * 1024) {
      return I18N.t('tooLarge', { size: humanLimit(state.maxMB) });
    }
    return '';
  }

  function renderQueue() {
    el.queue.textContent = '';
    state.queue.forEach(function (item) {
      el.queue.appendChild(buildRow(item));
      updateItem(item);
    });
    renderOverall();
  }

  function buildRow(item) {
    var row = document.createElement('li');
    row.className = 'q-row';

    var top = document.createElement('div');
    top.className = 'q-top';

    var name = document.createElement('span');
    name.className = 'q-name';
    name.textContent = item.name;
    name.title = item.name;

    var action = document.createElement('button');
    action.className = 'q-action';
    action.type = 'button';
    action.addEventListener('click', function () {
      if (item.status === 'uploading') {
        return;
      }
      if (item.status === 'error') {
        item.status = 'ready';
        item.message = '';
        updateItem(item);
        updateButton();
        runQueue();
        return;
      }
      removeItem(item);
    });

    top.appendChild(name);
    top.appendChild(action);

    var track = document.createElement('div');
    track.className = 'q-track';
    var fill = document.createElement('span');
    fill.className = 'q-fill';
    track.appendChild(fill);

    var meta = document.createElement('div');
    meta.className = 'q-meta';
    var stateText = document.createElement('span');

    meta.appendChild(stateText);

    row.appendChild(top);
    row.appendChild(track);
    row.appendChild(meta);

    item.el = row;
    item.elFill = fill;
    item.elAction = action;
    item.elState = stateText;
    return row;
  }

  function updateItem(item) {
    if (!item.el) {
      return;
    }

    item.el.classList.toggle('is-done', item.status === 'done');
    item.el.classList.toggle('is-error', item.status === 'error');
    item.el.classList.toggle('is-uploading', item.status === 'uploading');

    var percent = item.status === 'done' ? 100 : item.progress;
    item.elFill.style.width = percent + '%';

    if (item.status === 'ready') {
      item.elState.textContent = humanSize(item.size);
      item.elAction.textContent = I18N.t('remove');
    } else if (item.status === 'uploading') {
      item.elState.textContent = I18N.t('uploading') + ' ' + item.progress + '%';
      item.elAction.textContent = I18N.t('remove');
    } else if (item.status === 'done') {
      item.elState.textContent = I18N.t('uploaded');
      item.elAction.textContent = I18N.t('remove');
    } else {
      item.elState.textContent = item.message || I18N.t('uploadFailed');
      item.elAction.textContent = I18N.t('retry');
    }
  }

  function removeItem(item) {
    if (item.status === 'uploading') {
      return;
    }
    state.queue = state.queue.filter(function (candidate) {
      return candidate.id !== item.id;
    });
    if (item.el && item.el.parentNode) {
      item.el.parentNode.removeChild(item.el);
    }
    updateButton();
  }

  function updateButton() {
    if (state.status !== 'ready') {
      el.uploadBtn.disabled = true;
      el.uploadBtn.textContent = I18N.t('upload');
      return;
    }
    var hasReady = state.queue.some(function (item) {
      return item.status === 'ready';
    });
    var hasError = state.queue.some(function (item) {
      return item.status === 'error';
    });
    if (state.uploading) {
      el.uploadBtn.disabled = true;
      el.uploadBtn.textContent = I18N.t('uploadingBtn');
    } else if (hasReady) {
      el.uploadBtn.disabled = false;
      el.uploadBtn.textContent = I18N.t('upload');
    } else if (hasError) {
      el.uploadBtn.disabled = false;
      el.uploadBtn.textContent = I18N.t('retryFailed');
    } else {
      el.uploadBtn.disabled = true;
      el.uploadBtn.textContent = I18N.t('upload');
    }
  }

  // Byte-weighted overall progress: done files count full size, in-flight files
  // count their current percentage. Hidden unless the inbox is live.
  function renderOverall() {
    if (state.status !== 'ready') {
      el.overall.hidden = true;
      return;
    }
    var total = state.queue.length;
    if (!total) {
      el.overall.hidden = true;
      return;
    }
    el.overall.hidden = false;

    var done = 0;
    var totalBytes = 0;
    var doneBytes = 0;
    state.queue.forEach(function (it) {
      if (it.status === 'done') {
        done += 1;
      }
      var size = it.size || 0;
      totalBytes += size;
      if (it.status === 'done') {
        doneBytes += size;
      } else if (it.status === 'uploading') {
        doneBytes += Math.round(size * (it.progress / 100));
      }
    });

    var pct = totalBytes > 0
      ? Math.min(100, Math.round((doneBytes / totalBytes) * 100))
      : (done === total ? 100 : 0);

    el.overallText.textContent = I18N.t('overallProgress', { done: done, total: total });
    el.overallPct.textContent = pct + '%';
    el.overallFill.style.width = pct + '%';
  }

  // Lightweight transient hint shown in the footer area (e.g. "N files added").
  var tipTimer = null;
  function tip(message) {
    el.foot.textContent = message;
    if (tipTimer) {
      clearTimeout(tipTimer);
    }
    tipTimer = window.setTimeout(renderFoot, 2600);
  }

  // ---------- upload ----------

  // Clicking the button either starts a fresh upload or, when only failed items
  // remain, flips every error back to ready and retries them all.
  function onUploadClick() {
    if (state.status !== 'ready') {
      return;
    }
    if (!state.queue.some(function (it) { return it.status === 'ready'; })) {
      state.queue.forEach(function (it) {
        if (it.status === 'error') {
          it.status = 'ready';
          it.message = '';
          updateItem(it);
        }
      });
    }
    runQueue();
  }

  // Concurrent pool: up to POOL uploads in flight at once. Workers pull the next
  // ready item dynamically, so files added mid-flight are picked up automatically.
  function runQueue() {
    if (state.status !== 'ready' || state.uploading) {
      return;
    }
    if (!state.queue.some(function (it) { return it.status === 'ready'; })) {
      return;
    }

    state.uploading = true;
    updateButton();

    var POOL = 3;
    var active = 0;

    function pickNext() {
      for (var i = 0; i < state.queue.length; i++) {
        if (state.queue[i].status === 'ready') {
          return state.queue[i];
        }
      }
      return null;
    }

    function pump() {
      while (active < POOL) {
        var item = pickNext();
        if (!item) {
          break;
        }
        active += 1;
        item.status = 'uploading';
        item.progress = 0;
        updateItem(item);
        renderOverall();
        uploadOne(item).then(onDone, onDone);
      }
    }

    function onDone() {
      active -= 1;
      renderOverall();
      if (pickNext()) {
        pump();
      } else if (active === 0) {
        state.uploading = false;
        updateItemDoneStates();
        updateButton();
        renderFoot();
        renderOverall();
      }
    }

    pump();
  }

  function uploadOne(item) {
    return new Promise(function (resolve) {
      var form = new FormData();
      form.append('file', item.file, item.name);

      var xhr = new XMLHttpRequest();
      xhr.open('POST', '/api/upload?session=' + encodeURIComponent(state.token), true);

      xhr.upload.onprogress = function (event) {
        if (event.lengthComputable && event.total > 0) {
          item.progress = Math.round((event.loaded / event.total) * 100);
          updateItem(item);
          renderOverall();
        }
      };

      xhr.onload = function () {
        if (xhr.status >= 200 && xhr.status < 300) {
          item.status = 'done';
          item.progress = 100;
          item.file = null; // release the blob once it is safely on the server
          updateItem(item);
          resolve(true);
          return;
        }
        if (xhr.status === 404 || xhr.status === 410) {
          item.status = 'error';
          item.message = I18N.t('sessionEnded');
          updateItem(item);
          markEnded();
          resolve(false);
          return;
        }
        item.status = 'error';
        item.message = parseError(xhr);
        updateItem(item);
        resolve(false);
      };

      xhr.onerror = function () {
        item.status = 'error';
        item.message = I18N.t('netError');
        updateItem(item);
        resolve(false);
      };

      item.xhr = xhr;
      xhr.send(form);
    });
  }

  function updateItemDoneStates() {
    state.queue.forEach(updateItem);
  }

  function parseError(xhr) {
    try {
      var data = JSON.parse(xhr.responseText);
      if (data && data.error) {
        return data.error;
      }
    } catch (e) {
      // fall through to the status text
    }
    return xhr.statusText || I18N.t('uploadFailed');
  }

  // ---------- countdown + states ----------

  function startTick() {
    stopTick();
    tick();
    state.tickTimer = window.setInterval(tick, 1000);
  }

  function stopTick() {
    if (state.tickTimer) {
      clearInterval(state.tickTimer);
      state.tickTimer = null;
    }
  }

  function tick() {
    if (!state.expiresAt) {
      return;
    }
    var remaining = Math.max(0, state.expiresAt - Date.now());
    renderFoot(remaining);
    if (remaining <= 0) {
      markEnded();
    }
  }

  function markEnded() {
    if (state.status === 'ended' || state.status === 'invalid') {
      return;
    }
    stopTick();
    state.status = 'ended';
    el.dropZone.hidden = true;
    el.uploadBtn.hidden = true;
    el.subtitle.hidden = true;
    showNotice(I18N.t('sessionEnded'));
    renderFoot(0);
    el.overall.hidden = true;
  }

  function markInvalid(message) {
    state.status = 'invalid';
    el.dropZone.hidden = true;
    el.uploadBtn.hidden = true;
    el.subtitle.hidden = true;
    showNotice(message || I18N.t('sessionInvalid'));
    renderFoot(0);
    el.overall.hidden = true;
  }

  function showNotice(message) {
    el.notice.textContent = message;
    el.notice.hidden = false;
  }

  function renderDynamic() {
    if (state.status === 'loading') {
      return;
    }
    if (state.status === 'ready') {
      el.subtitle.textContent = I18N.t('uploadSubtitle', {
        types: typeList(state.allowed)
      });
    }
    renderFoot();
    updateItemDoneStates();
    renderOverall();
  }

  function renderFoot(remaining) {
    if (state.status === 'ended' || state.status === 'invalid') {
      el.foot.textContent = I18N.t('footEnded');
      return;
    }
    if (!state.expiresAt) {
      el.foot.textContent = I18N.t('footIdle');
      return;
    }
    var ms = remaining === undefined ? Math.max(0, state.expiresAt - Date.now()) : remaining;
    el.foot.textContent = I18N.t('footCountdown', { time: formatClock(ms) });
  }

  function formatClock(ms) {
    var total = Math.floor(ms / 1000);
    return pad(Math.floor(total / 60)) + ':' + pad(total % 60);
  }

  function pad(n) {
    return n < 10 ? '0' + n : String(n);
  }

  // ---------- formatting ----------

  function typeList(allowed) {
    var set = {};
    allowed.forEach(function (ext) {
      set[ext] = true;
    });

    var known = {};
    var labels = [];
    EXT_GROUPS.forEach(function (group) {
      group.exts.forEach(function (ext) {
        known[ext] = true;
      });
      var hit = group.exts.some(function (ext) {
        return set[ext];
      });
      if (hit) {
        labels.push(I18N.lang === 'en' ? group.en || group.label : group.zh || group.label);
      }
    });

    // Custom whitelists may include extensions outside the groups above.
    allowed.forEach(function (ext) {
      if (!known[ext]) {
        labels.push(ext.toUpperCase());
      }
    });

    return labels.join(' / ');
  }

  function humanLimit(mb) {
    if (mb >= 1024 && mb % 1024 === 0) {
      return mb / 1024 + 'GB';
    }
    return mb + 'MB';
  }

  function humanSize(bytes) {
    if (bytes < 1024) {
      return bytes + ' B';
    }
    var units = ['KB', 'MB', 'GB'];
    var value = bytes / 1024;
    var i = 0;
    while (value >= 1024 && i < units.length - 1) {
      value /= 1024;
      i += 1;
    }
    return (value >= 10 ? value.toFixed(0) : value.toFixed(1)) + ' ' + units[i];
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
