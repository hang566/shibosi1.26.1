// ============================================================
//  搜索积分触发器 - Search Points Triggers
//  - 首次使用任意指令 +50
//  - 每日首次 /s +10
//  - 每日首次 /go +10
//  - 每日首次 /email +10
//  - 每日搜索递减奖励：10→5→3→2→1→1...
//  - 每日5个随机隐藏字，每命中一个 +50
// ============================================================
(function () {
    'use strict';

    const DAILY_KEYS = {
        FIRST_CMD: 'first_cmd_ever',
        CMD_S: 'cmd_s',
        CMD_GO: 'cmd_go',
        CMD_EMAIL: 'cmd_email',
        SEARCH: 'search_count'
    };

    const SEARCH_TIER = [10, 5, 3, 2, 1];

    const RANDOM_WORD_CHARS = '市舶司云枢山海风光明月星辰天地江河山川万里乾坤清辉玉宇琼楼金阙星河浩渺';

    var initialized = false;

    function addPointsWithToast(gameKey, amount, message) {
        if (!window.PointsSystem || amount <= 0) return;
        PointsSystem.addPoints(gameKey, amount);
        if (message && typeof showToast === 'function') {
            showToast(message, 2000, 'success');
        }
    }

    function getRandomChars(count) {
        if (!window.PointsSystem) return { chars: [], used: {} };
        const today = PointsSystem.getTodayKey();
        const stored = localStorage.getItem('daily_random_word_data');
        if (stored) {
            try {
                const data = JSON.parse(stored);
                if (data.date === today && data.chars && data.chars.length === count) {
                    return data;
                }
            } catch (e) {}
        }
        const shuffled = RANDOM_WORD_CHARS.split('').sort(function () { return Math.random() - 0.5; });
        const chars = shuffled.slice(0, count);
        const data = { date: today, chars: chars, used: {} };
        localStorage.setItem('daily_random_word_data', JSON.stringify(data));
        return data;
    }

    function checkRandomWordBonus(query) {
        if (!query || query.startsWith('/') || !window.PointsSystem) return 0;
        const data = getRandomChars(5);
        if (!data.chars || data.chars.length === 0) return 0;
        let bonus = 0;
        let foundChars = [];
        for (let i = 0; i < data.chars.length; i++) {
            const ch = data.chars[i];
            if (!data.used[ch] && query.indexOf(ch) !== -1) {
                data.used[ch] = true;
                bonus += 50;
                foundChars.push(ch);
            }
        }
        if (bonus > 0) {
            localStorage.setItem('daily_random_word_data', JSON.stringify(data));
            addPointsWithToast(PointsSystem.GAME_KEYS.SEARCH, bonus,
                '🎯 隐藏字奖励 +' + bonus + ' 分 (' + foundChars.join(' ') + ')');
        }
        return bonus;
    }

    function getSearchTierPoints(count) {
        if (count <= 0) return 0;
        if (count <= SEARCH_TIER.length) return SEARCH_TIER[count - 1];
        return SEARCH_TIER[SEARCH_TIER.length - 1];
    }

    function triggerCommandPoints(cmdType) {
        if (!window.PointsSystem) return;
        const key = PointsSystem.GAME_KEYS.SEARCH;

        const firstEverData = PointsSystem.getDailyData(DAILY_KEYS.FIRST_CMD);
        if (firstEverData.firstTime) {
            PointsSystem.markDailyUsed(DAILY_KEYS.FIRST_CMD);
            addPointsWithToast(key, 50, '🚀 首次使用指令 +50 积分');
        }

        const dailyKey = {
            's': DAILY_KEYS.CMD_S,
            'go': DAILY_KEYS.CMD_GO,
            'email': DAILY_KEYS.CMD_EMAIL
        }[cmdType];

        if (dailyKey) {
            const data = PointsSystem.getDailyData(dailyKey);
            if (data.firstTime) {
                PointsSystem.markDailyUsed(dailyKey);
                const cmdNames = { 's': '/s', 'go': '/go', 'email': '/Email' };
                addPointsWithToast(key, 10, '⚡ ' + cmdNames[cmdType] + ' 每日首次 +10 积分');
            }
        }
    }

    function triggerSearchPoints(query) {
        if (!window.PointsSystem) return;
        const key = PointsSystem.GAME_KEYS.SEARCH;

        const searchData = PointsSystem.markDailyUsed(DAILY_KEYS.SEARCH);
        const count = searchData.count;
        const points = getSearchTierPoints(count);

        if (points > 0) {
            addPointsWithToast(key, points, '🔍 第 ' + count + ' 次搜索 +' + points + ' 积分');
        }

        checkRandomWordBonus(query);
    }

    function parseCommandType(query) {
        if (!query || !query.startsWith('/')) return null;
        const parts = query.split(/\s+/);
        const cmd = parts[0].slice(1).toLowerCase();
        return cmd;
    }

    function onSearchSubmit(query) {
        if (query.startsWith('/')) {
            const cmd = parseCommandType(query);
            if (cmd === 's' || cmd === 'go' || cmd === 'email') {
                triggerCommandPoints(cmd);
            }
        } else if (query) {
            triggerSearchPoints(query);
        }
    }

    function wrapRedirectToSearch() {
        if (typeof window.redirectToSearch !== 'function') return false;
        if (window.redirectToSearch.__pointsWrapped) return true;

        var origFn = window.redirectToSearch;
        window.redirectToSearch = function () {
            const input = document.getElementById('searchQuery');
            const query = input ? input.value.trim() : '';
            onSearchSubmit(query);
            return origFn.apply(this, arguments);
        };
        window.redirectToSearch.__pointsWrapped = true;
        return true;
    }

    function init() {
        if (initialized) return;
        initialized = true;

        if (!wrapRedirectToSearch()) {
            var tries = 0;
            var timer = setInterval(function () {
                tries++;
                if (wrapRedirectToSearch() || tries > 20) {
                    clearInterval(timer);
                }
            }, 100);
        }

        var input = document.getElementById('searchQuery');
        if (input) {
            input.addEventListener('keydown', function (e) {
                if (e.key === 'Enter') {
                    const val = input.value.trim();
                    if (val) {
                        setTimeout(function () { onSearchSubmit(val); }, 0);
                    }
                }
            });
        }
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }

    window.SearchPoints = {
        triggerCommandPoints: triggerCommandPoints,
        triggerSearchPoints: triggerSearchPoints,
        getRandomChars: getRandomChars,
        checkRandomWordBonus: checkRandomWordBonus,
        getSearchTierPoints: getSearchTierPoints,
        onSearchSubmit: onSearchSubmit
    };
})();
