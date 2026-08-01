/**
 * syncManager.js - 本地优先 + 云端同步管理器
 *
 * 设计原则：
 * 1. Local-First：所有读写操作优先使用 localStorage，保证即时响应
 * 2. 登录入云：用户登录后自动将本地数据推送到云端
 * 3. 增量同步：每次变更都标记 dirty，后台定时或事件触发同步
 * 4. 离线可用：网络断开时自动降级为纯本地模式
 * 5. 合并优先：登录时云端与本地数据合并，而非覆盖
 *
 * 同步的数据：
 * - 积分：game_total_points, points_history, game_stats
 * - 应用：apps_data_v3
 * - 搜索：custom_engines, search_history, selected_engine
 * - OC养成：oc_config, oc_status, oc_backpack, oc_ui_config, oc_idle_points
 * - 主题外观：primoUI_theme, chooseBg, 搜索框自定义设置
 * - 用户数据：shibosi_settings, shibosi_shortcuts, shibosi_groups, shibosi_favorites, shibosi_history
 */
(function (window) {
    'use strict';

    var SYNC_KEY = 'shibosi_sync_state';
    var SYNC_INTERVAL = 30000; // 30 秒自动同步
    var BATCH_DEBOUNCE = 800;  // 批量操作防抖

    var state = {
        loggedIn: false,
        userId: null,
        dirty: {},
        syncing: false,
        timer: null,
        listeners: {}
    };

    // ==================== 同步键配置 ====================
    // merge 策略：max(取最大), localWins(本地优先), mergeObjects(浅合并),
    //             mergeArrayById(按id合并), mergeArrayByKey(按指定字段合并),
    //             mergePointsHistory, mergeGameStats, mergeEngines, mergeOCStatus, mergeOCBackpack
    var SYNC_KEYS = [
        // 积分
        { local: 'game_total_points', cloud: 'points', type: 'number', merge: 'max' },
        { local: 'points_history', cloud: 'points_history', type: 'json', merge: 'mergePointsHistory' },
        { local: 'game_stats', cloud: 'game_stats', type: 'json', merge: 'mergeGameStats' },

        // 应用
        { local: 'apps_data_v3', cloud: 'apps', type: 'json', merge: 'mergeArrayByKey', key: 'url' },

        // 搜索引擎
        { local: 'custom_engines', cloud: 'custom_engines', type: 'json', merge: 'mergeEngines' },
        { local: 'search_history', cloud: 'search_history', type: 'json', merge: 'mergeArrayByKey', key: 'keyword' },
        { local: 'selected_engine', cloud: 'selected_engine', type: 'string', merge: 'localWins' },

        // OC 养成
        { local: 'oc_config', cloud: 'oc_config', type: 'json', merge: 'mergeObjects' },
        { local: 'oc_status', cloud: 'oc_status', type: 'json', merge: 'mergeOCStatus' },
        { local: 'oc_backpack', cloud: 'oc_backpack', type: 'json', merge: 'mergeOCBackpack' },
        { local: 'oc_ui_config', cloud: 'oc_ui_config', type: 'json', merge: 'mergeObjects' },
        { local: 'oc_idle_points', cloud: 'oc_idle_points', type: 'number', merge: 'max' },

        // 主题与外观
        { local: 'primoUI_theme', cloud: 'theme', type: 'string', merge: 'localWins' },
        { local: 'chooseBg', cloud: 'chooseBg', type: 'string', merge: 'localWins' },
        { local: 'searchLogoVisible', cloud: 'searchLogoVisible', type: 'string', merge: 'localWins' },
        { local: 'logoType', cloud: 'logoType', type: 'string', merge: 'localWins' },
        { local: 'logoText', cloud: 'logoText', type: 'string', merge: 'localWins' },
        { local: 'logoSize', cloud: 'logoSize', type: 'string', merge: 'localWins' },
        { local: 'searchOpacity', cloud: 'searchOpacity', type: 'string', merge: 'localWins' },
        { local: 'searchWidth', cloud: 'searchWidth', type: 'string', merge: 'localWins' },
        { local: 'searchRadius', cloud: 'searchRadius', type: 'string', merge: 'localWins' },
        { local: 'searchPlaceholder', cloud: 'searchPlaceholder', type: 'string', merge: 'localWins' },
        { local: 'autoSlashVisible', cloud: 'autoSlashVisible', type: 'string', merge: 'localWins' },
        { local: 'cardsHidden', cloud: 'cardsHidden', type: 'string', merge: 'localWins' },

        // 用户数据
        { local: 'shibosi_settings', cloud: 'settings', type: 'json', merge: 'mergeObjects' },
        { local: 'shibosi_shortcuts', cloud: 'shortcuts', type: 'json', merge: 'mergeArrayById' },
        { local: 'shibosi_groups', cloud: 'groups', type: 'json', merge: 'mergeArrayById' },
        { local: 'shibosi_favorites', cloud: 'favorites', type: 'json', merge: 'mergeArrayById' },
        { local: 'shibosi_history', cloud: 'history', type: 'json', merge: 'mergeArrayById' }
    ];

    // 构建 localStorage key -> dirty type 映射
    var DIRTY_KEY_MAP = {};
    (function buildDirtyMap() {
        var typeMap = {
            'points': ['game_total_points', 'points_history', 'game_stats'],
            'apps': ['apps_data_v3'],
            'search': ['custom_engines', 'search_history', 'selected_engine'],
            'oc': ['oc_config', 'oc_status', 'oc_backpack', 'oc_ui_config', 'oc_idle_points'],
            'appearance': ['primoUI_theme', 'chooseBg', 'searchLogoVisible', 'logoType', 'logoText',
                'logoSize', 'searchOpacity', 'searchWidth', 'searchRadius', 'searchPlaceholder',
                'autoSlashVisible', 'cardsHidden'],
            'settings': ['shibosi_settings', 'shibosi_shortcuts', 'shibosi_groups'],
            'favorites': ['shibosi_favorites'],
            'history': ['shibosi_history']
        };
        Object.keys(typeMap).forEach(function (type) {
            typeMap[type].forEach(function (key) {
                DIRTY_KEY_MAP[key] = type;
            });
        });
    })();

    // ==================== 工具函数 ====================

    function getLocal(key, defaultValue) {
        try {
            var val = localStorage.getItem(key);
            return val !== null ? JSON.parse(val) : defaultValue;
        } catch (e) {
            // 非 JSON 值（纯字符串）
            try {
                var raw = localStorage.getItem(key);
                return raw !== null ? raw : defaultValue;
            } catch (e2) {
                return defaultValue;
            }
        }
    }

    function getLocalRaw(key, defaultValue) {
        try {
            var val = localStorage.getItem(key);
            return val !== null ? val : defaultValue;
        } catch (e) {
            return defaultValue;
        }
    }

    function setLocal(key, value) {
        try {
            localStorage.setItem(key, typeof value === 'string' ? value : JSON.stringify(value));
        } catch (e) {
            console.warn('[SyncManager] localStorage 写入失败:', key, e);
        }
    }

    function getSyncState() {
        return getLocal(SYNC_KEY, { lastSync: 0, version: 0 });
    }

    function setSyncState(s) {
        setLocal(SYNC_KEY, s);
    }

    // ==================== API 辅助 ====================

    function getApiBase() {
        if (window.__APP_CONFIG__ && window.__APP_CONFIG__.apiBase) {
            return window.__APP_CONFIG__.apiBase;
        }
        return 'http://localhost:8084/api/v1';
    }

    function getToken() {
        return localStorage.getItem('shibosi_token') || '';
    }

    function apiRequest(path, options) {
        var url = getApiBase() + path;
        var opts = options || {};
        opts.headers = opts.headers || {};
        opts.headers['Content-Type'] = 'application/json';
        var token = getToken();
        if (token) {
            opts.headers['Authorization'] = 'Bearer ' + token;
        }
        opts.credentials = 'include';

        return fetch(url, opts).then(function (res) {
            if (res.status === 401) {
                window.dispatchEvent(new CustomEvent('auth:logout'));
                throw new Error('Unauthorized');
            }
            return res.json();
        }).then(function (data) {
            if (data.code !== 0) {
                throw new Error(data.msg || 'API 错误');
            }
            return data.data;
        });
    }

    // ==================== 事件监听 ====================

    function on(event, handler) {
        if (!state.listeners[event]) {
            state.listeners[event] = [];
        }
        state.listeners[event].push(handler);
    }

    function emit(event, data) {
        var handlers = state.listeners[event] || [];
        for (var i = 0; i < handlers.length; i++) {
            try { handlers[i](data); } catch (e) {}
        }
    }

    // ==================== 同步标记 ====================

    function markDirty(type) {
        state.dirty[type] = true;
        scheduleSync();
    }

    function clearDirty(type) {
        delete state.dirty[type];
    }

    function clearAllDirty() {
        state.dirty = {};
    }

    // ==================== 自动同步调度 ====================

    function scheduleSync() {
        if (!state.loggedIn) return;
        if (state.timer) return;
        state.timer = setTimeout(function () {
            state.timer = null;
            syncNow();
        }, BATCH_DEBOUNCE);
    }

    function startPeriodicSync() {
        if (state._periodicTimer) return;
        state._periodicTimer = setInterval(function () {
            if (state.loggedIn) syncNow();
        }, SYNC_INTERVAL);
    }

    // ==================== 合并策略 ====================

    function mergeMax(localVal, cloudVal) {
        var l = parseFloat(localVal) || 0;
        var c = parseFloat(cloudVal) || 0;
        return Math.max(l, c);
    }

    function mergeLocalWins(localVal, cloudVal) {
        // 本地有值则用本地，否则用云端
        if (localVal !== null && localVal !== undefined && localVal !== '') {
            return localVal;
        }
        return cloudVal;
    }

    function mergeObjects(localVal, cloudVal) {
        if (typeof localVal !== 'object' || localVal === null) localVal = {};
        if (typeof cloudVal !== 'object' || cloudVal === null) cloudVal = {};
        var result = {};
        Object.keys(cloudVal).forEach(function (k) { result[k] = cloudVal[k]; });
        Object.keys(localVal).forEach(function (k) { result[k] = localVal[k]; });
        return result;
    }

    // 通用数组合并：按指定字段去重，本地优先
    function mergeArrayByKey(localVal, cloudVal, keyField) {
        if (!Array.isArray(localVal)) localVal = [];
        if (!Array.isArray(cloudVal)) cloudVal = [];
        var map = {};
        // 云端先入
        cloudVal.forEach(function (item) {
            if (item && item[keyField] !== undefined && item[keyField] !== null && item[keyField] !== '') {
                map[String(item[keyField])] = item;
            }
        });
        // 本地覆盖（本地优先）
        localVal.forEach(function (item) {
            if (item && item[keyField] !== undefined && item[keyField] !== null && item[keyField] !== '') {
                map[String(item[keyField])] = item;
            } else if (item) {
                // 无指定字段的项：用 JSON 字符串作为 key 防止丢失
                var k = '__noid__' + JSON.stringify(item);
                map[k] = item;
            }
        });
        return Object.values(map);
    }

    // 按 id 合并（兼容无 id 的项）
    function mergeArrayById(localVal, cloudVal) {
        return mergeArrayByKey(localVal, cloudVal, 'id');
    }

    // 积分历史合并：按 timestamp 去重
    function mergePointsHistory(localVal, cloudVal) {
        if (!Array.isArray(localVal)) localVal = [];
        if (!Array.isArray(cloudVal)) cloudVal = [];
        var map = {};
        localVal.forEach(function (item) {
            if (item) {
                var k = item.timestamp || item.time || JSON.stringify(item);
                map[String(k)] = item;
            }
        });
        cloudVal.forEach(function (item) {
            if (item) {
                var k = item.timestamp || item.time || JSON.stringify(item);
                if (!map[String(k)]) map[String(k)] = item;
            }
        });
        return Object.values(map).sort(function (a, b) {
            var ta = a.timestamp || a.time || 0;
            var tb = b.timestamp || b.time || 0;
            return tb - ta;
        }).slice(0, 200);
    }

    // 游戏统计合并：每个游戏取最大 highScore，其余取本地
    function mergeGameStats(localVal, cloudVal) {
        if (typeof localVal !== 'object' || localVal === null) localVal = {};
        if (typeof cloudVal !== 'object' || cloudVal === null) cloudVal = {};
        var result = {};
        Object.keys(cloudVal).forEach(function (k) { result[k] = cloudVal[k]; });
        Object.keys(localVal).forEach(function (k) {
            if (!result[k]) {
                result[k] = localVal[k];
            } else {
                // 合并：highScore 取大值，其余取本地
                var merged = Object.assign({}, result[k], localVal[k]);
                merged.highScore = Math.max(
                    (result[k].highScore || 0),
                    (localVal[k].highScore || 0)
                );
                result[k] = merged;
            }
        });
        return result;
    }

    // 自定义引擎合并：对象 map，按 key 合并，本地优先
    function mergeEngines(localVal, cloudVal) {
        if (typeof localVal !== 'object' || localVal === null) localVal = {};
        if (typeof cloudVal !== 'object' || cloudVal === null) cloudVal = {};
        var result = {};
        Object.keys(cloudVal).forEach(function (k) { result[k] = cloudVal[k]; });
        Object.keys(localVal).forEach(function (k) { result[k] = localVal[k]; });
        return result;
    }

    // OC 状态合并：数值类字段取较大值，其余本地优先
    function mergeOCStatus(localVal, cloudVal) {
        if (typeof localVal !== 'object' || localVal === null) localVal = {};
        if (typeof cloudVal !== 'object' || cloudVal === null) cloudVal = {};
        var numericFields = ['life', 'hunger', 'energy', 'mood', 'level', 'exp', 'money', 'intimacy'];
        var result = {};
        Object.keys(cloudVal).forEach(function (k) { result[k] = cloudVal[k]; });
        Object.keys(localVal).forEach(function (k) {
            if (numericFields.indexOf(k) !== -1) {
                result[k] = Math.max(
                    parseFloat(cloudVal[k]) || 0,
                    parseFloat(localVal[k]) || 0
                );
            } else {
                result[k] = localVal[k];
            }
        });
        return result;
    }

    // OC 背包合并：合并 items 数组，capacity 取较大值
    function mergeOCBackpack(localVal, cloudVal) {
        if (typeof localVal !== 'object' || localVal === null) localVal = {};
        if (typeof cloudVal !== 'object' || cloudVal === null) cloudVal = {};
        var result = {
            capacity: Math.max(localVal.capacity || 0, cloudVal.capacity || 12),
            items: []
        };
        var localItems = Array.isArray(localVal.items) ? localVal.items : [];
        var cloudItems = Array.isArray(cloudVal.items) ? cloudVal.items : [];
        // 背包物品合并：按 id 去重不太合适（同 id 物品可叠加），这里直接合并去重
        var seen = {};
        cloudItems.forEach(function (item) {
            var k = JSON.stringify(item);
            if (!seen[k]) { seen[k] = true; result.items.push(item); }
        });
        localItems.forEach(function (item) {
            var k = JSON.stringify(item);
            if (!seen[k]) { seen[k] = true; result.items.push(item); }
        });
        return result;
    }

    // 执行合并
    function doMerge(mergeStrategy, localVal, cloudVal, keyField) {
        switch (mergeStrategy) {
            case 'max': return mergeMax(localVal, cloudVal);
            case 'localWins': return mergeLocalWins(localVal, cloudVal);
            case 'mergeObjects': return mergeObjects(localVal, cloudVal);
            case 'mergeArrayById': return mergeArrayById(localVal, cloudVal);
            case 'mergeArrayByKey': return mergeArrayByKey(localVal, cloudVal, keyField || 'id');
            case 'mergePointsHistory': return mergePointsHistory(localVal, cloudVal);
            case 'mergeGameStats': return mergeGameStats(localVal, cloudVal);
            case 'mergeEngines': return mergeEngines(localVal, cloudVal);
            case 'mergeOCStatus': return mergeOCStatus(localVal, cloudVal);
            case 'mergeOCBackpack': return mergeOCBackpack(localVal, cloudVal);
            default: return localVal !== undefined ? localVal : cloudVal;
        }
    }

    // ==================== 核心同步逻辑 ====================

    function syncNow() {
        if (!state.loggedIn || state.syncing) return;
        state.syncing = true;

        // 全量推送（有 dirty 标记的数据）
        pushToCloud().then(function () {
            setSyncState({ lastSync: Date.now(), version: getSyncState().version + 1 });
            emit('sync:complete');
        }).catch(function (err) {
            console.warn('[SyncManager] 同步失败:', err.message);
        }).then(function () {
            state.syncing = false;
        });
    }

    /**
     * 从云端拉取数据，合并到本地（合并而非覆盖）
     */
    function pullFromCloud() {
        if (!state.loggedIn) return Promise.resolve({});

        return apiRequest('/user/data').then(function (data) {
            if (!data) return {};

            SYNC_KEYS.forEach(function (cfg) {
                var cloudRaw = data[cfg.cloud];
                if (cloudRaw === undefined || cloudRaw === null) return;

                var localRaw = getLocalRaw(cfg.local, null);
                var localVal, cloudVal;

                // 解析本地值
                if (localRaw !== null) {
                    if (cfg.type === 'number') {
                        localVal = parseFloat(localRaw) || 0;
                    } else if (cfg.type === 'string') {
                        localVal = localRaw;
                    } else {
                        try { localVal = JSON.parse(localRaw); } catch (e) { localVal = null; }
                    }
                } else {
                    localVal = null;
                }

                // 解析云端值
                if (cfg.type === 'number') {
                    cloudVal = parseFloat(cloudRaw) || 0;
                } else if (cfg.type === 'string') {
                    cloudVal = cloudRaw;
                } else {
                    try { cloudVal = JSON.parse(cloudRaw); } catch (e) { cloudVal = null; }
                }

                if (cloudVal === null) return;

                // 合并
                var merged = doMerge(cfg.merge, localVal, cloudVal, cfg.key);

                // 写入本地（跳过 null/undefined）
                if (merged !== null && merged !== undefined) {
                    if (cfg.type === 'string') {
                        setLocal(cfg.local, String(merged));
                    } else if (cfg.type === 'number') {
                        setLocal(cfg.local, String(merged));
                    } else {
                        setLocal(cfg.local, JSON.stringify(merged));
                    }

                    // 主题特殊处理：应用主题
                    if (cfg.local === 'primoUI_theme' && typeof window.setTheme === 'function') {
                        try { window.setTheme(merged); } catch (e) {}
                    }
                }
            });

            emit('sync:dataUpdated');
            return data;
        });
    }

    /**
     * 将本地数据推送到云端
     */
    function pushToCloud() {
        if (!state.loggedIn) return Promise.resolve();

        var batchData = {};

        SYNC_KEYS.forEach(function (cfg) {
            var raw = getLocalRaw(cfg.local, null);
            if (raw !== null && raw !== '') {
                batchData[cfg.cloud] = raw;
            }
        });

        if (Object.keys(batchData).length === 0) return Promise.resolve();

        return apiRequest('/user/data/batch', {
            method: 'POST',
            body: JSON.stringify({ data: batchData })
        }).then(function () {
            clearAllDirty();
        }).catch(function (err) {
            console.warn('[SyncManager] 推送失败:', err.message);
        });
    }

    // ==================== 登录/登出 ====================

    /**
     * 用户登录后调用：先拉取云端数据合并到本地，再推送合并后的本地数据到云端
     * 合并策略：所有数据均合并，不覆盖
     */
    function handleLogin(userInfo, options) {
        options = options || {};
        state.loggedIn = true;
        state.userId = (userInfo && (userInfo.id || userInfo.user_id)) || null;

        if (options.skipSync) {
            startPeriodicSync();
            emit('sync:ready');
            return Promise.resolve({ skipped: true });
        }

        // 1. 拉取云端数据并合并到本地
        return pullFromCloud().then(function () {
            // 2. 推送合并后的本地数据到云端
            return pushToCloud();
        }).then(function () {
            startPeriodicSync();
            emit('sync:ready');
            emit('sync:dataUpdated');
        }).catch(function (err) {
            console.warn('[SyncManager] 登录同步完成（离线模式）:', err.message);
            emit('sync:ready');
        });
    }

    function handleLogout() {
        state.loggedIn = false;
        state.userId = null;
        state.dirty = {};

        if (state.timer) {
            clearTimeout(state.timer);
            state.timer = null;
        }
        if (state._periodicTimer) {
            clearInterval(state._periodicTimer);
            state._periodicTimer = null;
        }

        emit('sync:logout');
    }

    // ==================== localStorage 变更监听 ====================

    var _origSetItem = localStorage.setItem.bind(localStorage);
    var _origRemoveItem = localStorage.removeItem.bind(localStorage);

    localStorage.setItem = function (key, value) {
        var result = _origSetItem(key, value);
        var type = DIRTY_KEY_MAP[key];
        if (type && state.loggedIn) {
            markDirty(type);
        }
        return result;
    };

    localStorage.removeItem = function (key) {
        var result = _origRemoveItem(key);
        var type = DIRTY_KEY_MAP[key];
        if (type && state.loggedIn) {
            markDirty(type);
        }
        return result;
    };

    // ==================== 在线状态监听 ====================

    window.addEventListener('online', function () {
        if (state.loggedIn) {
            console.log('[SyncManager] 网络恢复，触发同步');
            syncNow();
        }
    });

    window.addEventListener('offline', function () {
        console.log('[SyncManager] 网络断开，切换到离线模式');
    });

    // ==================== 初始化 ====================

    function init() {
        var token = getToken();
        if (!token) return;

        // 尝试从 sessionStorage 或 localStorage 获取用户信息
        var userInfo = null;
        try {
            var userStr = sessionStorage.getItem('shibosi_user_cache_v1') ||
                localStorage.getItem('shibosi_user_cache_v1');
            if (userStr) userInfo = JSON.parse(userStr);
        } catch (e) {}

        // 有 token 即视为已登录（即使没有用户缓存）
        state.loggedIn = true;
        if (userInfo) state.userId = userInfo.id || userInfo.user_id;

        // 拉取云端数据合并到本地，然后推送
        handleLogin(userInfo || {}).catch(function () {});
    }

    // 暴露 API
    var SyncManager = {
        init: init,
        on: on,
        handleLogin: handleLogin,
        handleLogout: handleLogout,
        syncNow: syncNow,
        isOnline: function () { return navigator.onLine; },
        isLoggedIn: function () { return state.loggedIn; },

        markDirty: markDirty,
        markFavoritesDirty: function () { markDirty('favorites'); },
        markHistoryDirty: function () { markDirty('history'); },

        // 数据操作
        getPoints: function () { return getLocal('game_total_points', 0); },
        getPointsHistory: function () { return getLocal('points_history', []); },
        getSettings: function () { return getLocal('shibosi_settings', {}); },
        getShortcuts: function () { return getLocal('shibosi_shortcuts', []); },
        getGroups: function () { return getLocal('shibosi_groups', []); },

        _getLocal: getLocal,
        _setLocal: setLocal
    };

    // 兼容 PointsSystem 接口
    window.PointsSystem = window.PointsSystem || {
        getPoints: function () { return getLocal('game_total_points', 0); },
        history: function () { return getLocal('points_history', []); }
    };

    window.SyncManager = SyncManager;

    // DOM Ready 后初始化
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }

})(window);
