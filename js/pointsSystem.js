const PointsSystem = (function() {
    const STORAGE_KEY = 'game_total_points';
    const GAME_STATS_KEY = 'game_stats';
    
    const GAME_KEYS = {
        LIGHT_CORRIDOR: 'light_corridor',
        STAR_LUSTER: 'star_luster',
        TIC_TAC_TOE: 'tic_tac_toe',
        TETRIS: 'tetris',
        MONOPOLY: 'monopoly',
        DOU_DIZHU: 'dou_dizhu',
        DAILY_LOGIN: 'daily_login',
        SEARCH: 'search',
        GACHA: 'gacha',
        STOCK: 'stock',
        OC: 'oc',
        COLLECTOR: 'collector'
    };

    let totalPoints = 0;
    let gameRecords = {};
    let updateCallbacks = [];

    function loadPoints() {
        try {
            const saved = localStorage.getItem(STORAGE_KEY);
            if (saved) {
                totalPoints = parseInt(saved) || 0;
            }
            
            const statsSaved = localStorage.getItem(GAME_STATS_KEY);
            if (statsSaved) {
                gameRecords = JSON.parse(statsSaved);
            } else {
                gameRecords = {
                    [GAME_KEYS.LIGHT_CORRIDOR]: { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 },
                    [GAME_KEYS.STAR_LUSTER]: { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 },
                    [GAME_KEYS.TIC_TAC_TOE]: { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 },
                    [GAME_KEYS.TETRIS]: { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 },
                    [GAME_KEYS.MONOPOLY]: { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 },
                    [GAME_KEYS.DOU_DIZHU]: { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 },
                    [GAME_KEYS.DAILY_LOGIN]: { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 },
                    [GAME_KEYS.SEARCH]: { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 },
                    [GAME_KEYS.GACHA]: { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 },
                    [GAME_KEYS.STOCK]: { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 },
                    [GAME_KEYS.OC]: { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 },
                    [GAME_KEYS.COLLECTOR]: { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 }
                };
            }
        } catch (e) {
            console.error('Failed to load points:', e);
            totalPoints = 0;
            gameRecords = {};
        }
    }

    function savePoints() {
        try {
            localStorage.setItem(STORAGE_KEY, totalPoints.toString());
            localStorage.setItem(GAME_STATS_KEY, JSON.stringify(gameRecords));
        } catch (e) {
            console.error('Failed to save points:', e);
        }
    }

    function notifyUpdate() {
        updateCallbacks.forEach(callback => {
            try {
                callback(totalPoints, gameRecords);
            } catch (e) {
                console.error('Error in update callback:', e);
            }
        });
    }

    function addToHistory(gameKey, amount, type, desc) {
        try {
            const HISTORY_KEY = 'points_history';
            let records = [];
            const saved = localStorage.getItem(HISTORY_KEY);
            if (saved) {
                records = JSON.parse(saved);
            }
            const isSpend = type === 'spend';
            const isSearch = gameKey === 'search' || gameKey === 'daily_login';
            records.unshift({
                type: type || 'earn',
                category: isSpend ? 'spend' : (isSearch ? 'search' : 'game'),
                gameKey: gameKey,
                title: desc || getGameDisplayName(gameKey),
                desc: isSpend ? ('消耗 ' + amount + ' 积分') : ('获得 ' + amount + ' 积分'),
                points: amount,
                timestamp: Date.now()
            });
            records = records.slice(0, 200);
            localStorage.setItem(HISTORY_KEY, JSON.stringify(records));
        } catch (e) {
            console.warn('Failed to save points history:', e);
        }
    }

    function getGameDisplayName(key) {
        const names = {
            'light_corridor': '光之回廊',
            'star_luster': '星空璀璨',
            'tic_tac_toe': '井字棋',
            'tetris': '俄罗斯方块',
            'monopoly': '大富翁',
            'dou_dizhu': '斗地主',
            'daily_login': '每日登录',
            'search': '搜索奖励',
            'gacha': '抽卡',
            'stock': '股票',
            'oc': '原创角色',
            'collector': '收集系统',
            'spend': '积分消耗'
        };
        return names[key] || key;
    }

    function addPoints(gameKey, amount, desc) {
        const validKeys = Object.values(GAME_KEYS);
        if (!validKeys.includes(gameKey)) {
            console.warn('Invalid game key:', gameKey, 'Valid keys:', validKeys);
            return;
        }

        amount = Math.max(0, parseInt(amount) || 0);
        totalPoints += amount;
        
        if (!gameRecords[gameKey]) {
            gameRecords[gameKey] = { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 };
        }
        
        gameRecords[gameKey].totalPoints += amount;
        if (amount > gameRecords[gameKey].highScore) {
            gameRecords[gameKey].highScore = amount;
        }
        
        savePoints();
        notifyUpdate();
        addToHistory(gameKey, amount, 'earn', desc);
        console.log('Points added:', amount, 'Total:', totalPoints, 'Game:', gameKey);
    }

    function addWin(gameKey) {
        if (!gameRecords[gameKey]) {
            gameRecords[gameKey] = { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 };
        }
        gameRecords[gameKey].wins++;
        savePoints();
        notifyUpdate();
    }

    function addGamePlayed(gameKey) {
        if (!gameRecords[gameKey]) {
            gameRecords[gameKey] = { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 };
        }
        gameRecords[gameKey].gamesPlayed++;
        savePoints();
        notifyUpdate();
    }

    function spendPoints(amount, desc) {
        amount = Math.max(0, parseInt(amount) || 0);
        if (totalPoints >= amount) {
            totalPoints -= amount;
            savePoints();
            notifyUpdate();
            addToHistory('spend', amount, 'spend', desc || '积分消耗');
            console.log('Points spent:', amount, 'Remaining:', totalPoints);
            return true;
        }
        console.log('Not enough points:', totalPoints, 'Needed:', amount);
        return false;
    }

    function hasEnoughPoints(amount) {
        return totalPoints >= parseInt(amount) || 0;
    }

    function getTotalPoints() {
        return totalPoints;
    }

    function getGameRecord(gameKey) {
        return gameRecords[gameKey] || { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 };
    }

    function getAllRecords() {
        return { ...gameRecords };
    }

    function resetAll() {
        totalPoints = 0;
        gameRecords = {
            [GAME_KEYS.LIGHT_CORRIDOR]: { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 },
            [GAME_KEYS.STAR_LUSTER]: { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 },
            [GAME_KEYS.TIC_TAC_TOE]: { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 },
            [GAME_KEYS.TETRIS]: { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 },
            [GAME_KEYS.MONOPOLY]: { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 },
            [GAME_KEYS.DOU_DIZHU]: { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 },
            [GAME_KEYS.DAILY_LOGIN]: { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 },
            [GAME_KEYS.SEARCH]: { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 },
            [GAME_KEYS.GACHA]: { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 },
            [GAME_KEYS.STOCK]: { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 },
            [GAME_KEYS.OC]: { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 },
            [GAME_KEYS.COLLECTOR]: { highScore: 0, gamesPlayed: 0, wins: 0, totalPoints: 0 }
        };
        savePoints();
        notifyUpdate();
    }

    function onUpdate(callback) {
        if (typeof callback === 'function') {
            updateCallbacks.push(callback);
        }
    }

    function getTodayKey() {
        const d = new Date();
        return d.getFullYear() + '-' + (d.getMonth() + 1) + '-' + d.getDate();
    }

    function getDailyData(key) {
        try {
            const raw = localStorage.getItem('daily_' + key);
            if (raw) {
                const data = JSON.parse(raw);
                if (data.date === getTodayKey()) {
                    return data;
                }
            }
        } catch (e) {}
        return { date: getTodayKey(), count: 0, firstTime: true, extra: {} };
    }

    function setDailyData(key, data) {
        try {
            data.date = getTodayKey();
            localStorage.setItem('daily_' + key, JSON.stringify(data));
        } catch (e) {}
    }

    function isFirstDaily(key) {
        const data = getDailyData(key);
        return data.firstTime;
    }

    function markDailyUsed(key) {
        const data = getDailyData(key);
        data.firstTime = false;
        data.count = (data.count || 0) + 1;
        setDailyData(key, data);
        return data;
    }

    function getDailyCount(key) {
        const data = getDailyData(key);
        return data.count || 0;
    }

    // 导出全量积分日志（给用户中心入云调用）
    function exportLogs() {
        const HISTORY_KEY = 'points_history';
        const result = [];
        try {
            const raw = localStorage.getItem(HISTORY_KEY);
            if (!raw) return result;
            const list = JSON.parse(raw) || [];
            list.forEach(function(r) {
                if (!r) return;
                const createdAt = r.timestamp
                    ? (function formatLocal(ts) {
                        const d = new Date(ts);
                        const p = (n) => (n < 10 ? '0' + n : '' + n);
                        return d.getFullYear() + '-' + p(d.getMonth() + 1) + '-' + p(d.getDate()) + ' '
                            + p(d.getHours()) + ':' + p(d.getMinutes()) + ':' + p(d.getSeconds());
                    })(r.timestamp) : '';
                result.push({
                    points: Number(r.points) || 0,
                    balance_after: 0,
                    action: r.category || r.gameKey || r.type || 'local',
                    description: (r.title ? r.title + ' — ' : '') + (r.desc || ''),
                    created_at: createdAt
                });
            });
        } catch (e) {
            console.warn('Failed to export points logs:', e);
        }
        return result;
    }

    loadPoints();

    return {
        GAME_KEYS,
        addPoints,
        addWin,
        addGamePlayed,
        spendPoints,
        hasEnoughPoints,
        getTotalPoints,
        getGameRecord,
        getAllRecords,
        resetAll,
        onUpdate,
        loadPoints,
        getTodayKey,
        getDailyData,
        setDailyData,
        isFirstDaily,
        markDailyUsed,
        getDailyCount,
        exportLogs
    };
})();

window.PointsSystem = PointsSystem;