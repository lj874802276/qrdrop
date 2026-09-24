// QRDrop host page: create an inbox, show its QR code, and stream received
// files in over WebSocket (with a polling fallback). Vanilla JS, no framework.
(function () {
  'use strict';

  var I18N = window.QRDropI18n;
  var SESSION_KEY = 'qrdrop.session';
  var POLL_MS = 3000;
  var WS_MAX_RETRY = 3;

  var CHECK_ICON =
    '<svg width="14" height="14" viewBox="0 0 16 16" fill="none" aria-hidden="true">' +
    '<path d="M3.5 8.5l3 3 6-6.5" stroke="currentColor" stroke-width="1.8" ' +
    'stroke-linecap="round" stroke-linejoin="round"/></svg>';

  var FOLDER_ICON =
    '<svg width="15" height="15" viewBox="0 0 16 16" fill="none" aria-hidden="true">' +
    '<path d="M1.5 4.5a1 1 0 0 1 1-1h3L6.7 4.9h5.8a1 1 0 0 1 1 1V11a1 1 0 0 1-1 1H2.5a1 1 0 0 1-1-1V4.5z" ' +
    'stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/></svg>';

  var el = {};
  var toastTimer = null;

  var state = {
    token: '',
    uploadURL: '',
    expiresAt: 0,
    status: 'idle', // idle | open | closed | expired
    files: {},
    order: [],
    ws: null,
    wsRetry: 0,
    reconnectTimer: null,
    pollTimer: null,
    tickTimer: null,
    closing: false
  };

  function $(id) {
    return document.getElementById(id);
  }

  // ---------- transport ----------

  function api(path, options) {
    return fetch(path, options).then(function (res) {
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

  // ---------- lifecycle ----------

  function init() {
    el.newInbox = $('newInbox');
    el.closeInbox = $('closeInbox');
    el.statusChip = $('statusChip');
    el.statusText = $('statusText');
    el.qrImage = $('qrImage');
    el.qrPlaceholder = $('qrPlaceholder');
    el.qrCaption = $('qrCaption');
    el.qrMeta = $('qrMeta');
    el.countdown = $('countdown');
    el.copyLink = $('copyLink');
    el.saveDir = $('saveDir');
    el.fileList = $('fileList');
    el.fileEmpty = $('fileEmpty');
    el.toast = $('toast');

    // settings + history overlays
    el.settingsBtn = $('settingsBtn');
    el.historyBtn = $('historyBtn');
    el.settingsOverlay = $('settingsOverlay');
    el.settingsClose = $('settingsClose');
    el.historyOverlay = $('historyOverlay');
    el.historyClose = $('historyClose');
    el.customPath = $('customPath');
    el.resolvedDir = $('resolvedDir');
    el.pathStatus = $('pathStatus');
    el.saveSettingsBtn = $('saveSettingsBtn');
    el.historyList = $('historyList');
    el.historyEmpty = $('historyEmpty');

    el.newInbox.addEventListener('click', createInbox);
    el.closeInbox.addEventListener('click', closeInbox);
    el.copyLink.addEventListener('click', copyLink);
    el.settingsBtn.addEventListener('click', openSettings);
    el.settingsClose.addEventListener('click', closeSettings);
    el.historyBtn.addEventListener('click', openHistory);
    el.historyClose.addEventListener('click', closeHistory);
    el.saveSettingsBtn.addEventListener('click', saveSettings);
    el.customPath.addEventListener('input', onPrefChange);
    Array.prototype.forEach.call(document.getElementsByName('savePref'), function (r) {
      r.addEventListener('change', onPrefChange);
    });
    document.addEventListener('qrdrop:langchange', renderStatus);

    restoreSession();
  }

  function restoreSession() {
    var token = localStorage.getItem(SESSION_KEY);
    if (!token) {
      renderStatus();
      return;
    }

    api('/api/files?session=' + encodeURIComponent(token))
      .then(adopt)
      .catch(function (err) {
        // 404/410 mean the inbox is gone; anything else just falls back to idle.
        localStorage.removeItem(SESSION_KEY);
        if (err && err.status !== 404 && err.status !== 410) {
          toast(I18N.t('connLost'));
        }
        renderStatus();
      });
  }

  function createInbox() {
    el.newInbox.disabled = true;
    api('/api/session', { method: 'POST' })
      .then(adopt)
      .catch(function () {
        toast(I18N.t('createFailed'));
      })
      .then(function () {
        el.newInbox.disabled = false;
      });
  }

  function adopt(data) {
    teardown();

    state.token = data.token;
    state.uploadURL = data.upload_url;
    state.expiresAt = data.expires_at;
    state.status = data.status === 'open' ? 'open' : 'idle';
    localStorage.setItem(SESSION_KEY, state.token);

    el.qrImage.src = data.qr_url;
    el.qrImage.hidden = false;
    el.qrPlaceholder.hidden = true;
    el.qrCaption.hidden = false;
    el.qrMeta.hidden = false;
    el.closeInbox.hidden = false;
    el.copyLink.disabled = false;

    var dir = data.save_dir || '';
    el.saveDir.textContent = dir || '—';
    el.saveDir.title = dir;

    setFiles(data.files || []);
    renderStatus();
    startTick();
    connectWS();
  }

  function closeInbox() {
    if (!state.token || state.closing) {
      return;
    }
    if (!window.confirm(I18N.t('confirmClose'))) {
      return;
    }

    state.closing = true;
    api('/api/session/close?session=' + encodeURIComponent(state.token), { method: 'POST' })
      .catch(function () {
        // Even if the request fails the host intent is to stop; reflect it locally.
      })
      .then(function () {
        state.closing = false;
        toast(I18N.t('fileRetained'));
        markEnded('closed');
      });
  }

  // ---------- rendering ----------

  function setFiles(list) {
    state.files = {};
    state.order = [];
    list.forEach(function (meta) {
      state.files[meta.id] = meta;
      state.order.push(meta.id);
    });
    renderFiles();
  }

  function renderFiles() {
    el.fileList.textContent = '';
    state.order.forEach(function (id) {
      el.fileList.appendChild(buildRow(state.files[id], false));
    });
    updateEmptyState();
  }

  function addFile(meta) {
    if (!meta || state.files[meta.id]) {
      return;
    }
    state.files[meta.id] = meta;
    state.order.push(meta.id);
    el.fileList.appendChild(buildRow(meta, true));
    updateEmptyState();
  }

  function buildRow(meta, isNew) {
    var row = document.createElement('li');
    row.className = 'file-row' + (isNew ? ' is-new' : '');

    var name = document.createElement('a');
    name.className = 'file-name';
    name.href = downloadURL(meta.id);
    name.target = '_blank';
    name.rel = 'noopener';
    name.title = I18N.t('openFile');
    name.textContent = meta.name;

    var size = document.createElement('span');
    size.className = 'file-size';
    size.textContent = humanSize(meta.size);

    var download = document.createElement('a');
    download.className = 'file-check';
    download.href = downloadURL(meta.id);
    download.setAttribute('download', meta.name);
    download.title = I18N.t('download');
    download.innerHTML = CHECK_ICON;

    var folder = document.createElement('button');
    folder.className = 'file-open';
    folder.type = 'button';
    folder.title = I18N.t('openFolder');
    folder.innerHTML = FOLDER_ICON;
    folder.addEventListener('click', function () {
      openFolder(meta.id);
    });

    row.appendChild(name);
    row.appendChild(size);
    row.appendChild(download);
    row.appendChild(folder);
    return row;
  }

  function updateEmptyState() {
    el.fileEmpty.hidden = state.order.length > 0;
  }

  function renderStatus() {
    var chips = {
      idle: ['chip-idle', 'statusIdle'],
      open: ['chip-open', 'statusOpen'],
      closed: ['chip-closed', 'statusClosed'],
      expired: ['chip-expired', 'statusExpired']
    };
    var chip = chips[state.status] || chips.idle;
    el.statusChip.className = 'chip ' + chip[0];
    el.statusText.textContent = I18N.t(chip[1]);
  }

  function downloadURL(id) {
    return '/api/download?session=' + encodeURIComponent(state.token) + '&id=' + encodeURIComponent(id);
  }

  function humanSize(bytes) {
    if (bytes === null || bytes === undefined) {
      return '';
    }
    if (bytes < 1024) {
      return bytes + ' B';
    }
    var units = ['KB', 'MB', 'GB', 'TB'];
    var value = bytes / 1024;
    var i = 0;
    while (value >= 1024 && i < units.length - 1) {
      value /= 1024;
      i += 1;
    }
    return (value >= 10 ? value.toFixed(0) : value.toFixed(1)) + ' ' + units[i];
  }

  // ---------- countdown ----------

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
    var remaining = Math.max(0, state.expiresAt - Date.now());
    el.countdown.textContent = formatClock(remaining);
    if (remaining <= 0) {
      markEnded('expired');
    }
  }

  function formatClock(ms) {
    var total = Math.floor(ms / 1000);
    return pad(Math.floor(total / 60)) + ':' + pad(total % 60);
  }

  function pad(n) {
    return n < 10 ? '0' + n : String(n);
  }

  // ---------- realtime ----------

  function connectWS() {
    if (!state.token || state.status !== 'open') {
      return;
    }
    stopWS();

    var scheme = location.protocol === 'https:' ? 'wss:' : 'ws:';
    var url = scheme + '//' + location.host + '/ws?session=' + encodeURIComponent(state.token);
    var ws;

    try {
      ws = new WebSocket(url);
    } catch (e) {
      startPolling();
      return;
    }

    state.ws = ws;

    ws.onopen = function () {
      state.wsRetry = 0;
      stopPolling();
    };

    ws.onmessage = function (event) {
      var message;
      try {
        message = JSON.parse(event.data);
      } catch (e) {
        return;
      }
      handleMessage(message);
    };

    ws.onclose = function () {
      if (state.ws !== ws) {
        return;
      }
      state.ws = null;
      scheduleReconnect();
    };
  }

  function stopWS() {
    if (state.reconnectTimer) {
      clearTimeout(state.reconnectTimer);
      state.reconnectTimer = null;
    }
    if (state.ws) {
      var ws = state.ws;
      state.ws = null;
      ws.onopen = null;
      ws.onmessage = null;
      ws.onclose = null;
      ws.onerror = null;
      try {
        ws.close();
      } catch (e) {
        // already closed
      }
    }
    state.wsRetry = 0;
  }

  function scheduleReconnect() {
    if (!state.token || state.status !== 'open') {
      return;
    }

    state.wsRetry += 1;
    if (state.wsRetry > WS_MAX_RETRY) {
      // Corporate proxies sometimes block WebSocket; fall back to polling.
      startPolling();
    }

    var delay = Math.min(1000 * Math.pow(2, state.wsRetry), 15000);
    state.reconnectTimer = window.setTimeout(connectWS, delay);
  }

  function handleMessage(message) {
    if (message.type === 'snapshot') {
      if (message.expires_at) {
        state.expiresAt = message.expires_at;
      }
      setFiles(message.files || []);
      return;
    }
    if (message.type === 'file') {
      addFile(message.file);
      return;
    }
    if (message.type === 'status' && message.status === 'closed') {
      markEnded('closed');
      return;
    }
    if (message.type === 'expired') {
      markEnded('expired');
    }
  }

  // ---------- polling fallback ----------

  function startPolling() {
    if (state.pollTimer) {
      return;
    }
    state.pollTimer = window.setInterval(pollOnce, POLL_MS);
    pollOnce();
  }

  function stopPolling() {
    if (state.pollTimer) {
      clearInterval(state.pollTimer);
      state.pollTimer = null;
    }
  }

  function pollOnce() {
    if (!state.token || state.status !== 'open') {
      return;
    }
    api('/api/files?session=' + encodeURIComponent(state.token))
      .then(function (data) {
        if (data.expires_at) {
          state.expiresAt = data.expires_at;
        }
        setFiles(data.files || []);
        if (!state.tickTimer) {
          startTick();
        }
      })
      .catch(function (err) {
        if (err && (err.status === 404 || err.status === 410)) {
          markEnded('expired');
        }
      });
  }

  // ---------- terminal states ----------

  function markEnded(status) {
    if (state.status === 'expired' || state.status === 'closed') {
      return;
    }
    teardown();
    state.status = status;
    localStorage.removeItem(SESSION_KEY);
    el.copyLink.disabled = true;
    renderStatus();
  }

  function teardown() {
    stopTick();
    stopWS();
    stopPolling();
  }

  // ---------- settings + history ----------

  function openSettings() {
    el.settingsOverlay.hidden = false;
    loadSettings();
  }

  function closeSettings() {
    el.settingsOverlay.hidden = true;
  }

  function loadSettings() {
    api('/api/settings')
      .then(function (data) {
        var pref = data.save_dir_pref || 'default';
        Array.prototype.forEach.call(document.getElementsByName('savePref'), function (r) {
          r.checked = (r.value === pref);
        });
        var isCustom = !pref || (pref !== 'desktop' && pref !== 'default');
        el.customPath.hidden = !isCustom;
        if (isCustom) {
          el.customPath.value = pref;
        }
        el.resolvedDir.textContent = data.resolved_dir || '—';
        if (data.valid) {
          el.pathStatus.hidden = true;
        } else {
          el.pathStatus.hidden = false;
          el.pathStatus.className = 'path-status';
          el.pathStatus.textContent = I18N.t('pathInvalid') + (data.error || '');
        }
      })
      .catch(function () {
        toast(I18N.t('loadFailed'));
      });
  }

  function currentPrefValue() {
    var sel = 'default';
    Array.prototype.forEach.call(document.getElementsByName('savePref'), function (r) {
      if (r.checked) { sel = r.value; }
    });
    return sel;
  }

  function onPrefChange() {
    el.customPath.hidden = (currentPrefValue() !== 'custom');
  }

  function saveSettings() {
    var val = currentPrefValue();
    var pref = val === 'custom' ? el.customPath.value.trim() : val;
    if (val === 'custom' && !pref) {
      toast(I18N.t('saveCustomPlaceholder'));
      return;
    }
    el.saveSettingsBtn.disabled = true;
    api('/api/settings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ save_dir: pref })
    })
      .then(function (data) {
        el.resolvedDir.textContent = data.resolved_dir || el.resolvedDir.textContent;
        el.pathStatus.hidden = true;
        toast(I18N.t('settingsSaved'));
        closeSettings();
      })
      .catch(function (err) {
        var msg = (err && err.message) ? err.message : I18N.t('settingsFailed');
        el.pathStatus.hidden = false;
        el.pathStatus.className = 'path-status';
        el.pathStatus.textContent = I18N.t('settingsFailed') + msg;
        toast(I18N.t('settingsFailed') + msg);
      })
      .then(function () {
        el.saveSettingsBtn.disabled = false;
        loadSettings();
      });
  }

  function openFolder(id, token) {
    token = token || state.token;
    if (!token) {
      return;
    }
    api('/api/open-folder?session=' + encodeURIComponent(token) + '&id=' + encodeURIComponent(id), { method: 'POST' })
      .then(function () {
        toast(I18N.t('openFolder'));
      })
      .catch(function (err) {
        toast((err && err.message) ? err.message : I18N.t('netError'));
      });
  }

  function openHistory() {
    el.historyOverlay.hidden = false;
    api('/api/history')
      .then(function (data) {
        renderHistory(data.sessions || []);
      })
      .catch(function () {
        toast(I18N.t('loadFailed'));
      });
  }

  function closeHistory() {
    el.historyOverlay.hidden = true;
  }

  function renderHistory(sessions) {
    el.historyList.textContent = '';
    if (!sessions.length) {
      el.historyEmpty.hidden = false;
      return;
    }
    el.historyEmpty.hidden = true;
    sessions.forEach(function (sess) {
      el.historyList.appendChild(buildHistoryItem(sess));
    });
  }

  function buildHistoryItem(sess) {
    var li = document.createElement('li');
    li.className = 'history-item';

    var head = document.createElement('div');
    head.className = 'history-head';

    var tokenEl = document.createElement('span');
    tokenEl.className = 'history-token mono';
    tokenEl.textContent = (sess.token || '').slice(0, 8) + '…';

    var meta = document.createElement('span');
    meta.className = 'history-meta';
    var parts = [I18N.t('histCreated') + ' ' + new Date(sess.created_at).toLocaleString()];
    if (sess.closed_at) {
      parts.push(I18N.t('histClosed') + ' ' + new Date(sess.closed_at).toLocaleString());
    }
    parts.push(sess.files.length + ' ' + I18N.t('histFiles'));
    meta.textContent = parts.join(' · ');

    head.appendChild(tokenEl);
    head.appendChild(meta);

    var loc = document.createElement('p');
    loc.className = 'history-loc';
    loc.textContent = I18N.t('histLocation') + '：' + (sess.save_dir || '—');

    li.appendChild(head);
    li.appendChild(loc);

    if (sess.files && sess.files.length) {
      var ul = document.createElement('ul');
      ul.className = 'history-files';
      sess.files.forEach(function (f) {
        var fri = document.createElement('li');
        fri.className = 'file-row';

        var name = document.createElement('a');
        name.className = 'file-name';
        var href = '/api/download?session=' + encodeURIComponent(sess.token) + '&id=' + encodeURIComponent(f.id);
        name.href = href;
        name.target = '_blank';
        name.rel = 'noopener';
        name.textContent = f.name;

        var size = document.createElement('span');
        size.className = 'file-size';
        size.textContent = humanSize(f.size);

        var dl = document.createElement('a');
        dl.className = 'file-check';
        dl.href = href;
        dl.setAttribute('download', f.name);
        dl.title = I18N.t('download');
        dl.innerHTML = CHECK_ICON;

        var folder = document.createElement('button');
        folder.className = 'file-open';
        folder.type = 'button';
        folder.title = I18N.t('openFolder');
        folder.innerHTML = FOLDER_ICON;
        folder.addEventListener('click', function () {
          openFolder(f.id, sess.token);
        });

        fri.appendChild(name);
        fri.appendChild(size);
        fri.appendChild(dl);
        fri.appendChild(folder);
        ul.appendChild(fri);
      });
      li.appendChild(ul);
    }
    return li;
  }

  // ---------- clipboard + toast ----------

  function copyLink() {
    if (!state.uploadURL) {
      return;
    }
    if (navigator.clipboard && window.isSecureContext) {
      navigator.clipboard.writeText(state.uploadURL).then(function () {
        toast(I18N.t('copied'));
      }, function () {
        fallbackCopy(state.uploadURL);
      });
      return;
    }
    fallbackCopy(state.uploadURL);
  }

  function fallbackCopy(text) {
    var helper = document.createElement('textarea');
    helper.value = text;
    helper.setAttribute('readonly', '');
    helper.style.position = 'fixed';
    helper.style.top = '-1000px';
    document.body.appendChild(helper);
    helper.select();

    var ok = false;
    try {
      ok = document.execCommand('copy');
    } catch (e) {
      ok = false;
    }
    document.body.removeChild(helper);
    toast(I18N.t(ok ? 'copied' : 'copyFailed'));
  }

  function toast(message) {
    el.toast.textContent = message;
    el.toast.hidden = false;
    if (toastTimer) {
      clearTimeout(toastTimer);
    }
    toastTimer = window.setTimeout(function () {
      el.toast.hidden = true;
    }, 2200);
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
