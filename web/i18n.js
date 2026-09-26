// Shared zh/en dictionary + toggle for both QRDrop pages. No framework.
(function () {
  'use strict';

  var dict = {
    zh: {
      brandSub: '扫码传文件',
      newInbox: '新建收件箱',
      closeInbox: '关闭收件箱',
      statusIdle: '未开启',
      statusOpen: '收件箱开启中',
      statusClosed: '收件箱已关闭',
      statusExpired: '会话已过期',
      qrHint: '点击「新建收件箱」生成二维码',
      qrTitle: '扫码上传文件',
      netHint: '手机需连同一 WiFi，不能只用流量',
      troubleTitle: '手机扫不开？',
      troubleStep1: '确认手机连的是与电脑相同的 WiFi，不要用移动流量',
      troubleStep2: '关闭手机上的 VPN / 代理软件',
      troubleStep3: '企业 / 公共 WiFi 常隔离设备，可让手机开热点、电脑连热点后再扫',
      troubleStep4: '换用系统相机或微信「扫一扫」（浏览器内置扫码有时不跳转）',
      waitingFiles: '等待对方上传…',
      openSaveDir: '打开保存位置',
      sessionTTL: '会话有效期',
      copyLink: '复制链接',
      copied: '已复制链接',
      copyFailed: '复制失败，请手动复制',
      receivedFiles: '已接收文件',
      noFiles: '暂无文件',
      saveTo: '自动保存到',
      download: '下载文件',
      openFile: '打开文件',
      confirmClose: '确定关闭收件箱吗？已接收的文件会保留在本地。',
      cancelBtn: '取消',
      confirmYes: '确定关闭',
      createFailed: '创建收件箱失败，请重试',
      connLost: '连接已断开，正在重连',

      uploadTitle: '文件上传',
      uploadSubtitle: '请选择 {types} 文件上传',
      dropHint: '点击或拖拽选择文件',
      upload: '上传',
      uploading: '上传中',
      uploaded: '已完成',
      uploadFailed: '上传失败',
      retry: '重试',
      remove: '移除',
      footCountdown: '无需登录 · 会话 {time} 后自动关闭',
      footIdle: '无需登录 · 会话 30 分钟后自动关闭',
      footEnded: '会话已结束',
      sessionInvalid: '会话无效或已过期',
      sessionEnded: '会话已结束，请重新扫码',
      typeNotAllowed: '不支持的文件类型',
      tooLarge: '超过 {size} 限制',
      emptyFile: '空文件',
      netError: '网络错误',
      pickFirst: '请先选择文件',
      loadFailed: '无法加载会话信息',
      uploadNetError: '无法连接：请确认手机与电脑在同一 WiFi，且收件箱未关闭',

      settings: '设置',
      settingsTitle: '保存位置',
      saveDesktop: '桌面',
      saveDefault: '默认目录',
      saveCustom: '自定义',
      saveCustomPlaceholder: '输入保存文件夹的完整路径',
      resolvedDir: '实际保存位置',
      pathValid: '路径可用',
      pathInvalid: '路径不可用：',
      settingsSaved: '设置已保存',
      settingsFailed: '保存失败：',
      openFolder: '打开所在文件夹',
      history: '历史记录',
      historyEmpty: '暂无历史记录',
      histCreated: '创建于',
      histClosed: '关闭于',
      histFiles: '个文件',
      histLocation: '保存位置',
      fileRetained: '已接收的文件已保留在本地',
      saveBtn: '保存',
      openSource: '开源地址',
      themeDark: '深色',
      themeLight: '浅色'
    },
    en: {
      brandSub: 'Scan & deliver',
      newInbox: 'New inbox',
      closeInbox: 'Close inbox',
      statusIdle: 'Not started',
      statusOpen: 'Inbox open',
      statusClosed: 'Inbox closed',
      statusExpired: 'Session expired',
      qrHint: 'Click "New inbox" to generate a QR code',
      qrTitle: 'Scan & upload',
      netHint: 'Your phone must be on the same WiFi (not mobile data)',
      troubleTitle: 'Cannot open it?',
      troubleStep1: 'Make sure your phone is on the same WiFi as this computer, not mobile data',
      troubleStep2: 'Turn off any VPN or proxy app on your phone',
      troubleStep3: 'Corporate or public WiFi often isolates devices — try using your phone as a hotspot and connecting the computer to it',
      troubleStep4: 'Try the system camera or WeChat Scan (an in-app browser scan sometimes stalls)',
      waitingFiles: 'Waiting for uploads…',
      openSaveDir: 'Open save location',
      sessionTTL: 'Expires in',
      copyLink: 'Copy link',
      copied: 'Link copied',
      copyFailed: 'Copy failed, please copy manually',
      receivedFiles: 'Received files',
      noFiles: 'No files yet',
      saveTo: 'Saved to',
      download: 'Download file',
      openFile: 'Open file',
      confirmClose: 'Close this inbox? Received files stay on disk.',
      cancelBtn: 'Cancel',
      confirmYes: 'Close inbox',
      createFailed: 'Could not create the inbox, please retry',
      connLost: 'Connection lost, reconnecting',

      uploadTitle: 'Upload files',
      uploadSubtitle: 'Choose {types} files to upload',
      dropHint: 'Tap or drop to choose files',
      upload: 'Upload',
      uploading: 'Uploading',
      uploaded: 'Done',
      uploadFailed: 'Upload failed',
      retry: 'Retry',
      remove: 'Remove',
      footCountdown: 'No sign-in needed · closes in {time}',
      footIdle: 'No sign-in needed · closes 30 minutes after opening',
      footEnded: 'This session has ended',
      sessionInvalid: 'This session is invalid or has expired',
      sessionEnded: 'Session ended — please scan again',
      typeNotAllowed: 'Unsupported file type',
      tooLarge: 'Over the {size} limit',
      emptyFile: 'Empty file',
      netError: 'Network error',
      pickFirst: 'Choose a file first',
      loadFailed: 'Could not load the session',
      uploadNetError: 'Cannot connect: make sure your phone and this computer share the same WiFi, and the inbox is still open',

      settings: 'Settings',
      settingsTitle: 'Save location',
      saveDesktop: 'Desktop',
      saveDefault: 'Default folder',
      saveCustom: 'Custom',
      saveCustomPlaceholder: 'Enter the full path to a folder',
      resolvedDir: 'Resolved location',
      pathValid: 'Path is usable',
      pathInvalid: 'Path is unusable: ',
      settingsSaved: 'Settings saved',
      settingsFailed: 'Save failed: ',
      openFolder: 'Open containing folder',
      history: 'History',
      historyEmpty: 'No history yet',
      histCreated: 'Created',
      histClosed: 'Closed',
      histFiles: 'files',
      histLocation: 'Location',
      fileRetained: 'Received files are kept on disk',
      saveBtn: 'Save',
      openSource: 'Open source',
      themeDark: 'Dark',
      themeLight: 'Light'
    }
  };

  var lang = localStorage.getItem('qrdrop.lang');
  if (lang !== 'zh' && lang !== 'en') {
    lang = 'zh';
  }

  function t(key, vars) {
    var table = dict[lang] || dict.zh;
    var text = table[key];
    if (text === undefined) {
      text = dict.zh[key];
    }
    if (text === undefined) {
      return key;
    }
    if (vars) {
      Object.keys(vars).forEach(function (name) {
        text = text.split('{' + name + '}').join(String(vars[name]));
      });
    }
    return text;
  }

  function apply(root) {
    var scope = root || document;

    each(scope.querySelectorAll('[data-i18n]'), function (el) {
      el.textContent = t(el.getAttribute('data-i18n'));
    });
    each(scope.querySelectorAll('[data-i18n-title]'), function (el) {
      el.title = t(el.getAttribute('data-i18n-title'));
    });
    each(scope.querySelectorAll('[data-i18n-placeholder]'), function (el) {
      el.placeholder = t(el.getAttribute('data-i18n-placeholder'));
    });

    document.documentElement.lang = lang === 'zh' ? 'zh-CN' : 'en';

    var toggle = document.getElementById('langToggle');
    if (toggle) {
      toggle.textContent = lang === 'zh' ? '中 | En' : 'En | 中';
    }
  }

  function each(list, fn) {
    Array.prototype.forEach.call(list, fn);
  }

  function setLang(next) {
    lang = next === 'en' ? 'en' : 'zh';
    localStorage.setItem('qrdrop.lang', lang);
    apply();
    document.dispatchEvent(new CustomEvent('qrdrop:langchange'));
  }

  function toggle() {
    setLang(lang === 'zh' ? 'en' : 'zh');
  }

  window.QRDropI18n = {
    t: t,
    apply: apply,
    toggle: toggle,
    each: each,
    get lang() {
      return lang;
    }
  };

  document.addEventListener('DOMContentLoaded', function () {
    var langBtn = document.getElementById('langToggle');
    if (langBtn) {
      langBtn.addEventListener('click', toggle);
    }
    apply();
  });
})();
