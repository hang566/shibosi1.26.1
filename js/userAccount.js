/**
 * shibosi-user-account
 * 跨页面用户账户组件
 * 提供：获取当前用户、登录状态检查、API 请求封装、收藏/历史记录管理
 */
(function () {
    'use strict';

    const TOKEN_KEY = 'shibosi_token';
    const USER_CACHE_KEY = 'shibosi_user_cache_v1';

    // 根据脚本 src 推断项目根路径
    function getAssetBase() {
        try {
            const scripts = document.querySelectorAll('script[src*="userAccount.js"]');
            if (scripts.length > 0) {
                const src = scripts[0].getAttribute('src') || '';
                // src 形如 /js/userAccount.js 或 ../../js/userAccount.js
                return src.replace(/js\/userAccount\.js.*$/, '');
            }
        } catch (e) {}
        return '/';
    }

    // 监听 storage 事件跨标签页同步
    const SYNC_DATA_KEYS = [
        TOKEN_KEY, USER_CACHE_KEY,
        'shibosi_favorites', 'shibosi_history',
        'game_total_points', 'points_history', 'game_stats',
        'shibosi_settings', 'shibosi_shortcuts', 'shibosi_groups'
    ];

    window.addEventListener('storage', function (e) {
        if (SYNC_DATA_KEYS.indexOf(e.key) !== -1) {
            notifyChange();
            // 触发跨标签页同步
            if (window.SyncManager && e.key !== TOKEN_KEY && e.key !== USER_CACHE_KEY) {
                var typeMap = {
                    'shibosi_favorites': 'favorites',
                    'shibosi_history': 'history',
                    'game_total_points': 'points',
                    'points_history': 'points',
                    'game_stats': 'points',
                    'shibosi_settings': 'settings',
                    'shibosi_shortcuts': 'settings',
                    'shibosi_groups': 'settings'
                };
                var type = typeMap[e.key];
                if (type && typeof SyncManager.markDirty === 'function') {
                    SyncManager.markDirty(type);
                }
            }
        }
    });

    // ---------- 状态管理 ----------
    let _userCache = null;
    let _changeListeners = [];

    function notifyChange() {
        _changeListeners.forEach(function (fn) {
            try { fn(getUser()); } catch (e) {}
        });
    }

    // ---------- 核心 API ----------

    /** 获取本地 token */
    function getToken() {
        return localStorage.getItem(TOKEN_KEY);
    }

    /** 检查是否已登录 */
    function isLoggedIn() {
        return !!getToken();
    }

    /** 获取当前用户信息（同步，先读缓存） */
    function getUser() {
        if (_userCache) return _userCache;
        try {
            const cached = sessionStorage.getItem(USER_CACHE_KEY);
            if (cached) {
                _userCache = JSON.parse(cached);
                return _userCache;
            }
        } catch (e) {}
        return null;
    }

    /** 获取用户头像 URL */
    function getAvatar() {
        const u = getUser();
        if (!u) return '';
        if (u.avatar) return u.avatar;
        return '';
    }

    /** 获取用户显示名称 */
    function getDisplayName() {
        const u = getUser();
        if (!u) return '访客';
        return u.nickname || u.username || '用户';
    }

    /** 获取用户角色 */
    function getRole() {
        const u = getUser();
        if (!u) return 'guest';
        return u.role || 'user';
    }

    /** 封装的 API 请求（自动带 token） */
    async function apiFetch(url, options) {
        options = options || {};
        const token = getToken();
        if (token) {
            options.headers = options.headers || {};
            options.headers['Authorization'] = 'Bearer ' + token;
        }
        // 后端 API 运行在 8084 端口
        var apiBase = (window.__APP_CONFIG__ && window.__APP_CONFIG__.apiBase) || 'http://localhost:8084';
        var fullUrl = url.startsWith('http') ? url : apiBase + url;
        const resp = await fetch(fullUrl, options);
        if (resp.status === 401) {
            // token 过期
            logout();
            throw new Error('登录已过期，请重新登录');
        }
        return resp;
    }

    /** 刷新用户信息（从服务器） */
    async function refresh() {
        try {
            const resp = await apiFetch('/api/v1/auth/profile', { method: 'GET' });
            const data = await resp.json();
            if (data && data.data) {
                _userCache = data.data;
                sessionStorage.setItem(USER_CACHE_KEY, JSON.stringify(data.data));
                notifyChange();
                return data.data;
            }
        } catch (e) {
            // 后端不可用时从缓存读取
            const cached = getUser();
            if (cached) return cached;
            throw e;
        }
        return null;
    }

    /** 保存登录信息 */
    function saveAuth(token, user) {
        localStorage.setItem(TOKEN_KEY, token);
        _userCache = user;
        try {
            sessionStorage.setItem(USER_CACHE_KEY, JSON.stringify(user));
        } catch (e) {}
        try {
            localStorage.setItem('shibosi_login_at', String(Date.now()));
        } catch (e) {}
        notifyChange();

        // 通知 SyncManager 登录，触发数据入云
        if (window.SyncManager && typeof SyncManager.handleLogin === 'function') {
            SyncManager.handleLogin(user).catch(function() {
                // 离线模式忽略
            });
        }
    }

    /** 退出登录 */
    function logout() {
        // 通知 SyncManager 登出
        if (window.SyncManager && typeof SyncManager.handleLogout === 'function') {
            SyncManager.handleLogout();
        }

        localStorage.removeItem(TOKEN_KEY);
        sessionStorage.removeItem(USER_CACHE_KEY);
        _userCache = null;
        // 保留本地收藏/历史等数据（只清 token 和用户缓存）
        // localStorage.removeItem('shibosi_favorites');
        // localStorage.removeItem('shibosi_history');
        notifyChange();
    }

    /** 跳转登录页 */
    function openLogin() {
        const redirect = encodeURIComponent(window.location.href);
        const base = getAssetBase();
        window.location.href = base + 'Service/user/SignUpORSignIn.html?redirect=' + redirect;
    }

    /** 跳转用户中心 */
    function goUserCenter(subView) {
        const hash = subView ? '#view=' + encodeURIComponent(subView) : '';
        const base = getAssetBase();
        window.location.href = base + 'Service/user/user.html' + hash;
    }

    /** 注册状态变化监听 */
    function onChange(fn) {
        if (typeof fn === 'function') {
            _changeListeners.push(fn);
        }
    }

    // ---------- DOM 注入：账户头像/登录按钮 ----------
    function injectAccountWidget() {
        // 支持多种容器选择器
        const selectors = [
            '[data-user-account]',
            '#user-account-slot',
            '#userAccountSlot',
            '[data-account-widget]'
        ];
        
        const containers = document.querySelectorAll(selectors.join(','));
        containers.forEach(function (container) {
            renderAccountWidget(container);
        });

        // 查找带有 data-login-action 属性的元素（点击跳转登录）
        document.querySelectorAll('[data-login-action]').forEach(function (el) {
            el.addEventListener('click', function (e) {
                e.preventDefault();
                openLogin();
            });
        });
    }

    function renderAccountWidget(container) {
        const user = getUser();
        if (user) {
            // 已登录状态
            const avatar = getAvatar();
            const initial = (user.nickname || user.username || 'U').charAt(0).toUpperCase();
            container.innerHTML = `
                <div class="account-widget logged-in" style="display:flex;align-items:center;gap:8px;cursor:pointer;" title="点击进入用户中心">
                    <span class="account-nickname" style="font-size:13px;color:var(--text);max-width:100px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;">${escapeHtml(getDisplayName())}</span>
                    <div class="account-avatar" style="width:32px;height:32px;border-radius:50%;background:linear-gradient(135deg,var(--main,#007aff),#5856d6);color:#fff;display:flex;align-items:center;justify-content:center;font-size:14px;font-weight:600;${avatar ? 'background-image:url(' + avatar + ');background-size:cover;background-position:center;' : ''}">${avatar ? '' : initial}</div>
                </div>
            `;
            container.querySelector('.account-widget').addEventListener('click', function () {
                goUserCenter();
            });
        } else {
            // 未登录状态
            container.innerHTML = `
                <div class="account-widget guest" style="display:flex;align-items:center;gap:8px;cursor:pointer;color:var(--text-sub);" title="点击登录">
                    <span>登录</span>
                </div>
            `;
            container.querySelector('.account-widget').addEventListener('click', function () {
                openLogin();
            });
        }
    }

    function escapeHtml(str) {
        if (!str) return '';
        return String(str).replace(/[&<>"]/g, function (m) {
            return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[m];
        });
    }

    // 初始化注入
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', injectAccountWidget);
    } else {
        injectAccountWidget();
    }

    // 用户变化时重新渲染
    onChange(function () {
        const selectors = [
            '[data-user-account]',
            '#user-account-slot',
            '#userAccountSlot',
            '[data-account-widget]'
        ];
        const containers = document.querySelectorAll(selectors.join(','));
        containers.forEach(function (container) {
            renderAccountWidget(container);
        });
    });

    // ---------- 暴露 API ----------
    const api = {
        getToken: getToken,
        isLoggedIn: isLoggedIn,
        getUser: getUser,
        getAvatar: getAvatar,
        getDisplayName: getDisplayName,
        getRole: getRole,
        apiFetch: apiFetch,
        refresh: refresh,
        saveAuth: saveAuth,
        logout: logout,
        openLogin: openLogin,
        goUserCenter: goUserCenter,
        onChange: onChange
    };

    window.ShiboSiAccount = api;

})();
