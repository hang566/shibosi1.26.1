// ============================================================
//  全局搜索：聚合站点页面、用户应用、跳转指令
//  开箱即用，无需手动配置
// ============================================================
(function () {
  'use strict';

  // ===== 内置站点页面清单（自动维护，无需用户配置）=====
  var SITE_PAGES = [
    { name: '首页', url: 'index.html', desc: '市舶司主页', icon: '🏠' },
    { name: '游戏中心', url: 'Service/game/home.html', desc: '休闲小游戏合集', icon: '🎮' },
    { name: '斗地主', url: 'Service/game/DouDizhu/DouDizhu.html', desc: '经典扑克游戏', icon: '🃏' },
    { name: '光之回廊', url: 'Service/game/LightCorridor/LightCorridor.html', desc: '冒险游戏', icon: '💡' },
    { name: '大富翁', url: 'Service/game/Monopoly/Monopoly.html', desc: '棋盘游戏', icon: '🎲' },
    { name: 'OC养成', url: 'Service/game/OC/oc.html', desc: '角色养成', icon: '🎭' },
    { name: '星之光芒', url: 'Service/game/Star Luster/Star Luster.html', desc: '太空立体战', icon: '🌟' },
    { name: '俄罗斯方块', url: 'Service/game/Tetris/Tetris.html', desc: '经典益智游戏', icon: '🟦' },
    { name: '井字棋', url: 'Service/game/Tic-tac-toe/Tic-Tac-Toe.html', desc: '双人对战', icon: '⭕' },
    { name: '股票', url: 'Service/game/Stock.html', desc: '模拟炒股', icon: '📈' },
    { name: '抽奖', url: 'Service/game/gacha.html', desc: '幸运抽奖', icon: '🎁' },
    { name: '网页抓取', url: 'Service/Scraper/Scraper.html', desc: '网页内容抓取工具', icon: '🕷️' },
    { name: '搜索引擎', url: 'Service/SearchEngine/index.html', desc: '本地搜索引擎', icon: '🔍' },
    { name: '帮助中心', url: 'help/home.html', desc: '使用帮助', icon: '❓' }
  ];

  // ===== /go 指令缓存（自动加载）=====
  var commandsCache = null;
  function loadCommands() {
    if (commandsCache) return Promise.resolve(commandsCache);
    return fetch('js/search/instruction/commands.json?' + Date.now())
      .then(function (r) { return r.ok ? r.json() : {}; })
      .then(function (data) { commandsCache = data || {}; return commandsCache; })
      .catch(function () { commandsCache = {}; return commandsCache; });
  }

  // ===== 用户应用（自动从 localStorage 读取）=====
  function loadApps() {
    try {
      var raw = localStorage.getItem('apps_data_v3');
      if (raw) {
        var parsed = JSON.parse(raw);
        if (Array.isArray(parsed)) return parsed;
      }
    } catch (e) {}
    return [];
  }

  // ===== 模糊匹配（直接包含 + 子序列匹配）=====
  function matchScore(text, kw) {
    var t = String(text == null ? '' : text).toLowerCase();
    var k = String(kw == null ? '' : kw).toLowerCase();
    if (!k) return 0;
    // 直接包含
    var idx = t.indexOf(k);
    if (idx === 0) return 100;
    if (idx !== -1) return 80;
    // 子序列：kw 的每个字符按顺序出现在 text 中
    var score = 0, ti = 0, lastIdx = -1, gapSum = 0;
    for (var i = 0; i < k.length; i++) {
      var ch = k.charAt(i);
      var found = t.indexOf(ch, ti);
      if (found === -1) return 0;
      if (lastIdx >= 0) gapSum += (found - lastIdx - 1);
      lastIdx = found;
      ti = found + 1;
      score += 10;
    }
    score -= gapSum;
    return score;
  }

  // ===== 聚合搜索 =====
  function searchAll(keyword) {
    var kw = (keyword || '').toLowerCase().trim();
    if (!kw) return [];
    var results = [];

    SITE_PAGES.forEach(function (p) {
      var s1 = matchScore(p.name, kw);
      var s2 = matchScore(p.desc, kw);
      var score = Math.max(s1, s2);
      if (score > 0) {
        results.push({
          type: 'page', title: p.name, desc: p.desc,
          icon: p.icon, url: p.url, badge: '页面', score: score
        });
      }
    });

    loadApps().forEach(function (a) {
      var s1 = matchScore(a.name, kw);
      var s2 = matchScore(a.desc, kw);
      var s3 = matchScore(a.url, kw);
      var score = Math.max(s1, s2, s3);
      if (score > 0) {
        results.push({
          type: 'app', title: a.name, desc: a.desc || a.url,
          icon: '🔗', url: a.url, badge: '应用', score: score
        });
      }
    });

    var cmds = commandsCache || {};
    Object.keys(cmds).forEach(function (k) {
      var s = matchScore(k, kw);
      if (s > 0) {
        results.push({
          type: 'cmd', title: k, desc: cmds[k],
          icon: '↗', url: cmds[k], badge: '跳转', score: s
        });
      }
    });

    results.sort(function (a, b) { return b.score - a.score; });
    return results.slice(0, 20);
  }

  // ===== UI 状态 =====
  var dropdown, list, input;
  var selectedIndex = -1, items = [];
  var rafPending = false;
  var isComposing = false;
  var debounceTimer = null;

  function ensureDOM() {
    if (dropdown) return;

    var styleId = 'gs-theme-vars';
    if (!document.getElementById(styleId)) {
      var style = document.createElement('style');
      style.id = styleId;
      style.textContent = [
        ':root {',
        '  --gs-bg: var(--card-bg, #1e1e1e);',
        '  --gs-text: var(--text, #fff);',
        '  --gs-hint: var(--text-placeholder, #888);',
        '  --gs-border: var(--border, rgba(255,255,255,0.1));',
        '  --gs-accent: var(--main, #ff8c00);',
        '  --gs-accent-bg: color-mix(in srgb, var(--main) 15%, transparent);',
        '  --gs-badge-page: var(--success, #4caf50);',
        '  --gs-badge-app: var(--main, #2196f3);',
        '  --gs-badge-cmd: var(--warning, #ff8c00);',
        '}',
        '@supports not (color: color-mix(in srgb, red, blue)) {',
        '  :root { --gs-accent-bg: rgba(255,140,0,0.15); }',
        '}'
      ].join('\n');
      document.head.appendChild(style);
    }

    dropdown = document.createElement('div');
    dropdown.id = 'globalSearchSuggest';
    dropdown.className = 'gs-dropdown';
    dropdown.style.cssText = [
      'display:none', 'position:fixed', 'max-height:340px', 'overflow-y:auto',
      'background:var(--gs-bg)', 'color:var(--gs-text)',
      'border-radius:0 0 16px 16px',
      'border:1px solid var(--gs-border)', 'border-top:0',
      'z-index:9999', 'padding:0',
      'box-shadow:0 12px 48px rgba(0,0,0,0.4)',
      'backdrop-filter:blur(12px)', '-webkit-backdrop-filter:blur(12px)'
    ].join(';');

    var header = document.createElement('div');
    header.style.cssText = 'padding:6px 16px 10px;font-size:11px;color:var(--gs-hint);border-bottom:1px solid var(--gs-border);';
    header.textContent = '全局搜索 (↑↓选择 Enter 打开 Esc 关闭)';
    dropdown.appendChild(header);

    list = document.createElement('div');
    list.style.cssText = 'display:flex;flex-direction:column;';
    dropdown.appendChild(list);

    document.body.appendChild(dropdown);
  }

  function show() {
    if (!dropdown || !input) return;
    // 与 go.js 的 cmdSuggest 互斥
    var cmdSuggest = document.getElementById('cmdSuggest');
    if (cmdSuggest) cmdSuggest.style.display = 'none';

    var mainBox = document.getElementById('mainSearchBox');
    var rect = input.getBoundingClientRect();
    var parentRect = mainBox ? mainBox.getBoundingClientRect() : rect;
    dropdown.style.left = parentRect.left + 'px';
    dropdown.style.top = rect.bottom + 'px';
    dropdown.style.width = parentRect.width + 'px';
    dropdown.style.display = 'block';
  }

  function hide() {
    if (dropdown) dropdown.style.display = 'none';
    selectedIndex = -1;
    items = [];
  }

  function escapeHtml(s) {
    var d = document.createElement('div');
    d.textContent = s == null ? '' : String(s);
    return d.innerHTML;
  }

  function highlight(text, kw) {
    var safe = escapeHtml(text);
    if (!kw) return safe;
    try {
      var regex = new RegExp('(' + kw.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + ')', 'gi');
      return safe.replace(regex, '<mark style="background:var(--gs-accent-bg);color:var(--gs-accent);border-radius:2px;padding:0 2px;">$1</mark>');
    } catch (e) { return safe; }
  }

  function render(keyword) {
    if (!list) return;
    items = searchAll(keyword);
    var kw = (keyword || '').toLowerCase().trim();
    if (items.length === 0) {
      list.innerHTML = '<div style="padding:10px 16px;font-size:13px;color:var(--gs-hint);">无本地匹配，回车使用当前搜索引擎</div>';
      return;
    }
    var html = '';
    items.forEach(function (it, idx) {
      var badgeVar = it.type === 'page' ? 'var(--gs-badge-page)' : (it.type === 'app' ? 'var(--gs-badge-app)' : 'var(--gs-badge-cmd)');
      html += '<div class="gs-item" data-index="' + idx + '" style="padding:8px 16px;font-size:13px;color:var(--gs-text);cursor:pointer;border-left:3px solid transparent;display:flex;align-items:center;gap:8px;transition:background 0.15s;">';
      html += '<span style="font-size:16px;width:22px;text-align:center;flex-shrink:0;">' + it.icon + '</span>';
      html += '<div style="flex:1;min-width:0;">';
      html += '<div style="display:flex;align-items:center;gap:6px;">';
      html += '<span style="font-weight:500;">' + highlight(it.title, kw) + '</span>';
      html += '<span style="font-size:10px;padding:1px 6px;border-radius:8px;background:' + badgeVar + '22;color:' + badgeVar + ';">' + it.badge + '</span>';
      html += '</div>';
      html += '<div style="font-size:11px;opacity:0.6;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;color:var(--gs-hint);">' + highlight(it.desc, kw) + '</div>';
      html += '</div>';
      html += '</div>';
    });
    list.innerHTML = html;

    var nodes = list.querySelectorAll('.gs-item');
    nodes.forEach(function (el) {
      el.addEventListener('click', function () {
        openResult(parseInt(this.getAttribute('data-index')));
      });
      el.addEventListener('mouseenter', function () {
        selectedIndex = parseInt(this.getAttribute('data-index'));
        updateSelection();
      });
    });
  }

  function updateSelection() {
    if (!list) return;
    var nodes = list.querySelectorAll('.gs-item');
    nodes.forEach(function (el, idx) {
      if (idx === selectedIndex) {
        el.style.background = 'var(--gs-accent-bg)';
        el.style.borderLeftColor = 'var(--gs-accent)';
        el.scrollIntoView({ block: 'nearest' });
      } else {
        el.style.background = 'transparent';
        el.style.borderLeftColor = 'transparent';
      }
    });
  }

  function openResult(idx) {
    if (idx < 0 || idx >= items.length) return;
    var it = items[idx];
    if (!it) return;
    var url = it.url;
    if (/^https?:\/\//i.test(url) || url.indexOf('mailto:') === 0) {
      window.open(url, '_blank');
    } else {
      window.location.href = url;
    }
    if (input) input.value = '';
    hide();
  }

  // ===== 核心搜索触发（统一入口）=====
  function triggerSearch() {
    if (!input) return;
    var val = input.value;
    if (val.trim().indexOf('/') === 0) {
      hide();
      return;
    }
    var kw = val.trim();
    if (!kw) { hide(); return; }
    render(kw);
    show();
  }

  // 防抖版本（用于 input 事件）
  function triggerSearchDebounced() {
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(triggerSearch, 80);
  }

  // ===== 滚动跟随 =====
  function onScroll() {
    if (!dropdown || dropdown.style.display === 'none') return;
    if (rafPending) return;
    rafPending = true;
    requestAnimationFrame(function () {
      rafPending = false;
      if (!dropdown || dropdown.style.display === 'none' || !input) return;
      var rect = input.getBoundingClientRect();
      if (rect.bottom < 0 || rect.top > window.innerHeight) {
        hide();
        return;
      }
      var mainBox = document.getElementById('mainSearchBox');
      var parentRect = mainBox ? mainBox.getBoundingClientRect() : rect;
      var topPos = rect.bottom;
      var maxTop = window.innerHeight - dropdown.offsetHeight - 8;
      if (topPos > maxTop) {
        topPos = rect.top - dropdown.offsetHeight;
        if (topPos < 8) topPos = 8;
      }
      dropdown.style.left = parentRect.left + 'px';
      dropdown.style.top = topPos + 'px';
      dropdown.style.width = parentRect.width + 'px';
    });
  }

  function init() {
    input = document.getElementById('searchQuery');
    if (!input) return;
    ensureDOM();
    loadCommands();

    // --- IME 输入法处理 ---
    input.addEventListener('compositionstart', function () {
      isComposing = true;
      hide();
    });
    input.addEventListener('compositionend', function () {
      isComposing = false;
      // 延迟一帧等 input.value 更新
      setTimeout(triggerSearch, 0);
    });

    // --- input 事件（跳过 IME 中间态）---
    input.addEventListener('input', function (e) {
      if (isComposing) return;
      if (e && e.inputType === 'insertCompositionText') return;
      triggerSearchDebounced();
    });

    // --- keyup 事件（兜底，确保 IME 确认后一定触发）---
    input.addEventListener('keyup', function (e) {
      if (isComposing) return;
      // 方向键/功能键不触发搜索
      var skipKeys = ['Shift', 'Control', 'Alt', 'Meta', 'CapsLock',
        'ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown',
        'Home', 'End', 'PageUp', 'PageDown', 'Escape', 'Enter'];
      if (skipKeys.indexOf(e.key) !== -1) return;
      triggerSearch();
    });

    // --- keydown 交互 ---
    input.addEventListener('keydown', function (e) {
      if (!dropdown || dropdown.style.display === 'none') return;
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        if (items.length === 0) return;
        selectedIndex = (selectedIndex + 1) % items.length;
        updateSelection();
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        if (items.length === 0) return;
        selectedIndex = selectedIndex <= 0 ? items.length - 1 : selectedIndex - 1;
        updateSelection();
      } else if (e.key === 'Enter') {
        if (selectedIndex >= 0 && selectedIndex < items.length) {
          e.preventDefault();
          e.stopPropagation();
          openResult(selectedIndex);
        }
      } else if (e.key === 'Escape') {
        hide();
      }
    });

    // --- 点击外部关闭 ---
    document.addEventListener('click', function (e) {
      if (dropdown && !dropdown.contains(e.target) && e.target !== input) {
        hide();
      }
    });

    // --- cmdSuggest 显示时关闭我们的下拉 ---
    var cmdSuggest = document.getElementById('cmdSuggest');
    if (cmdSuggest) {
      var observer = new MutationObserver(function () {
        if (cmdSuggest.style.display !== 'none' && cmdSuggest.style.display !== '') {
          hide();
        }
      });
      observer.observe(cmdSuggest, { attributes: true, attributeFilter: ['style'] });
    }

    // --- 窗口变化 ---
    window.addEventListener('resize', function () {
      if (dropdown && dropdown.style.display !== 'none') show();
    });

    // --- 滚动跟随 ---
    window.addEventListener('scroll', onScroll, { passive: true });
    window.addEventListener('wheel', onScroll, { passive: true });
    window.addEventListener('touchmove', onScroll, { passive: true });
    document.addEventListener('scroll', onScroll, { passive: true });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
