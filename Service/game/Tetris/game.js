const TetrisGame = (function() {
    const config = {
        COLS: 10,
        ROWS: 20,
        BLOCK_SIZE: 30,
        SMALL_BLOCK: 24,
        HIGH_KEY: 'tetris_max_score',
        SHAPES: {
            I: [[1, 1, 1, 1]],
            J: [[1, 0, 0], [1, 1, 1]],
            L: [[0, 0, 1], [1, 1, 1]],
            O: [[1, 1], [1, 1]],
            S: [[0, 1, 1], [1, 1, 0]],
            T: [[0, 1, 0], [1, 1, 1]],
            Z: [[1, 1, 0], [0, 1, 1]]
        },
        COLOR_NORMAL: {
            I: '#0cf', J: '#0066ff', L: '#ff9900', O: '#ffdd00',
            S: '#0ee044', T: '#aa44ff', Z: '#ff3344'
        },
        COLOR_BLIND: {
            I: '#00eeff', J: '#2244dd', L: '#ffaa22', O: '#ffdd11',
            S: '#22ee88', T: '#bb22ff', Z: '#ff2266'
        },
        SCORE_MAP: [0, 100, 300, 500, 800]
    };

    const state = {
        board: [],
        score: 0,
        lines: 0,
        level: 1,
        hardTotal: 0,
        baseSpeed: 800,
        dropSpeed: 800,
        gameRun: false,
        gamePause: false,
        showShadow: true,
        slowMode: false,
        isBlind: false,
        curShape: null,
        curX: 0,
        curY: 0,
        curType: null,
        nextType: null,
        holdType: null,
        canHold: true
    };

    const dom = {
        startView: document.getElementById('startView'),
        gameView: document.getElementById('gameView'),
        enterGameBtn: document.getElementById('enterGameBtn'),
        backHomeBtn: document.getElementById('backHomeBtn'),
        startHigh: document.getElementById('startHigh'),
        highScore: document.getElementById('highScore'),
        diffBtns: document.querySelectorAll('.diff-btn'),
        startShadow: document.getElementById('startShadow'),
        startSlow: document.getElementById('startSlow'),
        startBlind: document.getElementById('startBlind'),
        
        canvas: document.getElementById('gameBox'),
        nextCanvas: document.getElementById('nextCanvas'),
        holdCanvas: document.getElementById('holdCanvas'),
        mask: document.getElementById('gameMask'),
        gameOverOverlay: document.getElementById('gameOverOverlay'),
        
        scoreDom: document.getElementById('score'),
        linesDom: document.getElementById('lines'),
        levelDom: document.getElementById('level'),
        speedTxt: document.getElementById('speedTxt'),
        clearOnceDom: document.getElementById('clearOnce'),
        hardScoreDom: document.getElementById('hardScore'),
        finalScore: document.getElementById('finalScore'),
        finalLines: document.getElementById('finalLines'),
        finalLevel: document.getElementById('finalLevel'),
        
        restartBtn: document.getElementById('restartBtn'),
        pauseBtn: document.getElementById('pauseBtn'),
        hardDropBtn: document.getElementById('hardDropBtn'),
        holdBtn: document.getElementById('holdBtn'),
        retryBtn: document.getElementById('retryBtn'),
        
        tUp: document.getElementById('tUp'),
        tLeft: document.getElementById('tLeft'),
        tRight: document.getElementById('tRight'),
        tDown: document.getElementById('tDown'),
        tHard: document.getElementById('tHard'),
        tHold: document.getElementById('tHold')
    };

    const ctx = dom.canvas.getContext('2d');
    const nextCtx = dom.nextCanvas.getContext('2d');
    const holdCtx = dom.holdCanvas.getContext('2d');

    let timer = null;
    let maxLocalScore = 0;
    const SHAPE_KEYS = Object.keys(config.SHAPES);

    const audio = (function() {
        const AudioContextClass = window.AudioContext || window.webkitAudioContext;
        const audioCtx = new AudioContextClass();

        function playTone(freq, dur, vol = 0.15) {
            const osc = audioCtx.createOscillator();
            const gain = audioCtx.createGain();
            osc.connect(gain);
            gain.connect(audioCtx.destination);
            osc.frequency.value = freq;
            gain.gain.value = vol;
            osc.start();
            osc.stop(audioCtx.currentTime + dur);
        }

        return {
            move: () => playTone(160, 0.04),
            rotate: () => playTone(320, 0.06),
            softDrop: () => playTone(200, 0.03),
            lock: () => playTone(90, 0.08),
            clear1: () => playTone(400, 0.12),
            clear4: () => {
                playTone(520, 0.1);
                setTimeout(() => playTone(720, 0.1), 80);
            },
            gameOver: () => {
                playTone(110, 0.3);
                setTimeout(() => playTone(80, 0.4), 150);
            }
        };
    })();

    function getColor(type) {
        return state.isBlind ? config.COLOR_BLIND[type] : config.COLOR_NORMAL[type];
    }

    function randomType() {
        return SHAPE_KEYS[Math.floor(Math.random() * SHAPE_KEYS.length)];
    }

    function initBoard() {
        state.board = [];
        for (let r = 0; r < config.ROWS; r++) {
            state.board.push(new Array(config.COLS).fill(null));
        }
    }

    function drawCell(ctxObj, x, y, type, size, alpha = 1) {
        ctxObj.globalAlpha = alpha;
        const base = getColor(type);
        const grad = ctxObj.createLinearGradient(
            x * size, y * size,
            (x + 1) * size, (y + 1) * size
        );
        grad.addColorStop(0, '#fff');
        grad.addColorStop(0.2, base);
        grad.addColorStop(1, '#111');
        ctxObj.fillStyle = grad;
        ctxObj.fillRect(x * size + 1, y * size + 1, size - 3, size - 3);
        ctxObj.strokeStyle = '#ffffff88';
        ctxObj.lineWidth = 1;
        ctxObj.strokeRect(x * size + 1, y * size + 1, size - 3, size - 3);
        ctxObj.globalAlpha = 1;
    }

    function drawMiniShape(ctxObj, shapeType) {
        ctxObj.fillStyle = '#000';
        ctxObj.fillRect(0, 0, 120, 120);
        
        if (!shapeType) return;
        
        const s = config.SHAPES[shapeType];
        const totalWidth = s[0].length * config.SMALL_BLOCK;
        const totalHeight = s.length * config.SMALL_BLOCK;
        const ox = (120 - totalWidth) / 2;
        const oy = (120 - totalHeight) / 2;
        
        for (let r = 0; r < s.length; r++) {
            for (let c = 0; c < s[r].length; c++) {
                if (s[r][c]) {
                    drawCell(ctxObj, ox / config.SMALL_BLOCK + c, oy / config.SMALL_BLOCK + r, shapeType, config.SMALL_BLOCK);
                }
            }
        }
    }

    function drawNextMini() {
        drawMiniShape(nextCtx, state.nextType);
    }

    function drawHoldMini() {
        drawMiniShape(holdCtx, state.holdType);
    }

    function checkCollide(x, y, shape) {
        for (let r = 0; r < shape.length; r++) {
            for (let c = 0; c < shape[r].length; c++) {
                if (!shape[r][c]) continue;
                const nx = x + c;
                const ny = y + r;
                if (nx < 0 || nx >= config.COLS || ny >= config.ROWS || (ny >= 0 && state.board[ny][nx])) {
                    return true;
                }
            }
        }
        return false;
    }

    function getGhostY() {
        let sy = state.curY;
        while (!checkCollide(state.curX, sy + 1, state.curShape)) {
            sy++;
        }
        return sy;
    }

    function rotate() {
        const old = state.curShape;
        const rot = old[0].map((_, i) => old.map(r => r[i])).map(r => r.reverse());
        const offsetList = [0, -1, 1, -2, 2];
        
        for (const off of offsetList) {
            if (!checkCollide(state.curX + off, state.curY, rot)) {
                state.curShape = rot;
                state.curX += off;
                audio.rotate();
                return;
            }
        }
    }

    function lockToBoard() {
        for (let r = 0; r < state.curShape.length; r++) {
            for (let c = 0; c < state.curShape[r].length; c++) {
                if (state.curShape[r][c]) {
                    state.board[state.curY + r][state.curX + c] = state.curType;
                }
            }
        }
        clearLines();
        newBlock();
    }

    function clearLines() {
        let clearNum = 0;
        for (let r = config.ROWS - 1; r >= 0; r--) {
            if (state.board[r].every(v => v !== null)) {
                state.board.splice(r, 1);
                state.board.unshift(new Array(config.COLS).fill(null));
                r++;
                clearNum++;
            }
        }

        if (clearNum > 0) {
            state.score += config.SCORE_MAP[clearNum] * state.level;
            state.lines += clearNum;
            dom.clearOnceDom.textContent = clearNum;
            clearNum === 4 ? audio.clear4() : audio.clear1();
            
            state.level = Math.floor(state.lines / 10) + 1;
            state.dropSpeed = state.slowMode 
                ? state.baseSpeed * 1.8 
                : Math.max(100, state.baseSpeed - (state.level - 1) * 80);
            
            updateSpeedText();
            updateUI();
            resetTimer();
        }
    }

    function updateSpeedText() {
        if (state.dropSpeed >= 700) {
            dom.speedTxt.textContent = '慢速';
        } else if (state.dropSpeed >= 350) {
            dom.speedTxt.textContent = '标准';
        } else {
            dom.speedTxt.textContent = '高速';
        }
    }

    function softDrop() {
        if (checkCollide(state.curX, state.curY + 1, state.curShape)) {
            lockToBoard();
        } else {
            state.curY++;
            state.score += 1;
            audio.softDrop();
        }
        render();
    }

    function hardDrop() {
        if (!state.gameRun || state.gamePause) return;
        
        const gy = getGhostY();
        const dropDist = gy - state.curY;
        state.score += dropDist * 2;
        state.hardTotal += dropDist * 2;
        dom.hardScoreDom.textContent = state.hardTotal;
        state.curY = gy;
        audio.lock();
        lockToBoard();
        render();
    }

    function newBlock() {
        state.canHold = true;
        state.curType = state.nextType || randomType();
        state.nextType = randomType();
        state.curShape = config.SHAPES[state.curType].map(r => [...r]);
        state.curX = Math.floor((config.COLS - state.curShape[0].length) / 2);
        state.curY = 0;
        drawNextMini();
        
        if (checkCollide(state.curX, state.curY, state.curShape)) {
            gameOver();
        }
    }

    function swapHold() {
        if (!state.gameRun || state.gamePause || !state.canHold) return;
        
        state.canHold = false;
        audio.rotate();
        
        const tmp = state.holdType;
        state.holdType = state.curType;
        
        if (tmp) {
            state.curType = tmp;
            state.curShape = config.SHAPES[state.curType].map(r => [...r]);
            state.curX = Math.floor((config.COLS - state.curShape[0].length) / 2);
            state.curY = 0;
        } else {
            newBlock();
        }
        
        drawHoldMini();
        render();
    }

    function render() {
        ctx.fillStyle = '#000';
        ctx.fillRect(0, 0, dom.canvas.width, dom.canvas.height);
        
        ctx.strokeStyle = '#222';
        ctx.lineWidth = 0.5;
        for (let i = 0; i <= config.COLS; i++) {
            ctx.beginPath();
            ctx.moveTo(i * config.BLOCK_SIZE, 0);
            ctx.lineTo(i * config.BLOCK_SIZE, dom.canvas.height);
            ctx.stroke();
        }
        for (let i = 0; i <= config.ROWS; i++) {
            ctx.beginPath();
            ctx.moveTo(0, i * config.BLOCK_SIZE);
            ctx.lineTo(dom.canvas.width, i * config.BLOCK_SIZE);
            ctx.stroke();
        }

        for (let r = 0; r < config.ROWS; r++) {
            for (let c = 0; c < config.COLS; c++) {
                if (state.board[r][c]) {
                    drawCell(ctx, c, r, state.board[r][c], config.BLOCK_SIZE);
                }
            }
        }

        if (state.showShadow) {
            const gy = getGhostY();
            ctx.fillStyle = 'rgba(255, 255, 255, 0.12)';
            for (let r = 0; r < state.curShape.length; r++) {
                for (let c = 0; c < state.curShape[r].length; c++) {
                    if (state.curShape[r][c]) {
                        ctx.fillRect(
                            (state.curX + c) * config.BLOCK_SIZE + 1,
                            (gy + r) * config.BLOCK_SIZE + 1,
                            config.BLOCK_SIZE - 3,
                            config.BLOCK_SIZE - 3
                        );
                    }
                }
            }
        }

        for (let r = 0; r < state.curShape.length; r++) {
            for (let c = 0; c < state.curShape[r].length; c++) {
                if (state.curShape[r][c]) {
                    drawCell(ctx, state.curX + c, state.curY + r, state.curType, config.BLOCK_SIZE);
                }
            }
        }
    }

    function updateUI() {
        dom.scoreDom.textContent = state.score;
        dom.linesDom.textContent = state.lines;
        dom.levelDom.textContent = state.level;
    }

    function resetTimer() {
        clearInterval(timer);
        timer = setInterval(softDrop, state.dropSpeed);
    }

    function togglePause() {
        if (!state.gameRun) return;
        
        state.gamePause = !state.gamePause;
        dom.mask.style.display = state.gamePause ? 'flex' : 'none';
        
        if (state.gamePause) {
            clearInterval(timer);
            dom.pauseBtn.textContent = '继续';
        } else {
            resetTimer();
            dom.pauseBtn.textContent = '暂停';
        }
    }

    function gameOver() {
        state.gameRun = false;
        state.gamePause = false;
        clearInterval(timer);
        audio.gameOver();
        saveHigh();
        
        dom.finalScore.textContent = state.score;
        dom.finalLines.textContent = state.lines;
        dom.finalLevel.textContent = state.level;
        dom.gameOverOverlay.style.display = 'flex';
        dom.pauseBtn.textContent = '暂停';
        
        // 添加积分奖励
        if (window.PointsSystem) {
            // 根据得分和消除行数计算积分
            const basePoints = Math.floor(state.score / 100); // 游戏内每100分=1积分
            const lineBonus = state.lines * 5; // 每消除一行奖励5积分
            const levelBonus = (state.level - 1) * 20; // 每升一级奖励20积分
            
            const totalPoints = basePoints + lineBonus + levelBonus;
            
            if (totalPoints > 0) {
                PointsSystem.addPoints(PointsSystem.GAME_KEYS.TETRIS, totalPoints);
                PointsSystem.addWin(PointsSystem.GAME_KEYS.TETRIS);
            }
        }
    }

    function startGame() {
        initBoard();
        state.score = 0;
        state.lines = 0;
        state.level = 1;
        state.hardTotal = 0;
        state.dropSpeed = state.slowMode ? state.baseSpeed * 1.8 : state.baseSpeed;
        state.gameRun = true;
        state.gamePause = false;
        state.holdType = null;
        state.nextType = randomType();
        
        dom.mask.style.display = 'none';
        dom.gameOverOverlay.style.display = 'none';
        updateUI();
        updateSpeedText();
        dom.hardScoreDom.textContent = 0;
        
        newBlock();
        drawHoldMini();
        resetTimer();
        render();
        dom.pauseBtn.textContent = '暂停';
    }

    function loadLocalHighScore() {
        const save = localStorage.getItem(config.HIGH_KEY);
        maxLocalScore = save ? Number(save) : 0;
        dom.startHigh.textContent = maxLocalScore;
        dom.highScore.textContent = maxLocalScore;
    }

    function saveHigh() {
        if (state.score > maxLocalScore) {
            maxLocalScore = state.score;
            localStorage.setItem(config.HIGH_KEY, maxLocalScore);
            dom.highScore.textContent = maxLocalScore;
            dom.startHigh.textContent = maxLocalScore;
        }
    }

    function switchToGame() {
        dom.startView.classList.add('view-hidden');
        dom.gameView.classList.remove('view-hidden');
        initGameFromStartSetting();
    }

    function switchToHome() {
        clearInterval(timer);
        state.gameRun = false;
        state.gamePause = false;
        dom.gameView.classList.add('view-hidden');
        dom.startView.classList.remove('view-hidden');
        loadLocalHighScore();
    }

    function initGameFromStartSetting() {
        state.baseSpeed = baseSpeedSet;
        state.showShadow = startShadow;
        state.slowMode = startSlow;
        state.isBlind = startBlind;
        startGame();
    }

    let baseSpeedSet = 800;
    let startShadow = true;
    let startSlow = false;
    let startBlind = false;

    function bindEvents() {
        dom.enterGameBtn.addEventListener('click', switchToGame);
        dom.backHomeBtn.addEventListener('click', switchToHome);
        dom.retryBtn.addEventListener('click', startGame);

        dom.diffBtns.forEach(btn => {
            btn.addEventListener('click', () => {
                dom.diffBtns.forEach(b => b.classList.remove('active'));
                btn.classList.add('active');
                baseSpeedSet = Number(btn.dataset.speed);
            });
        });

        dom.startShadow.addEventListener('change', e => { startShadow = e.target.checked; });
        dom.startSlow.addEventListener('change', e => { startSlow = e.target.checked; });
        dom.startBlind.addEventListener('change', e => { startBlind = e.target.checked; });

        // 为所有按钮添加触屏支持
        const buttons = [
            { elem: dom.restartBtn, handler: startGame },
            { elem: dom.pauseBtn, handler: togglePause },
            { elem: dom.hardDropBtn, handler: hardDrop },
            { elem: dom.holdBtn, handler: swapHold },
            { elem: dom.retryBtn, handler: startGame },
            { elem: dom.backHomeBtn, handler: switchToHome },
            { elem: dom.enterGameBtn, handler: switchToGame }
        ];

        buttons.forEach(({ elem, handler }) => {
            elem.addEventListener('click', handler);
            elem.addEventListener('touchstart', (e) => {
                e.preventDefault();
                handler();
            }, { passive: false });
        });

        // 触屏按钮事件处理
        function bindTouchEvent(elem, callback) {
            elem.addEventListener('click', callback);
            elem.addEventListener('touchstart', (e) => {
                e.preventDefault();
                callback();
            }, { passive: false });
        }

        bindTouchEvent(dom.tUp, () => { rotate(); render(); });
        bindTouchEvent(dom.tLeft, () => {
            if (!checkCollide(state.curX - 1, state.curY, state.curShape)) {
                state.curX--;
                audio.move();
                render();
            }
        });
        bindTouchEvent(dom.tRight, () => {
            if (!checkCollide(state.curX + 1, state.curY, state.curShape)) {
                state.curX++;
                audio.move();
                render();
            }
        });
        bindTouchEvent(dom.tDown, softDrop);
        bindTouchEvent(dom.tHard, hardDrop);
        bindTouchEvent(dom.tHold, swapHold);

        document.addEventListener('keydown', e => {
            if (dom.gameView.classList.contains('view-hidden')) return;
            if (!state.gameRun || state.gamePause) return;

            switch (e.key) {
                case 'ArrowLeft':
                    e.preventDefault();
                    if (!checkCollide(state.curX - 1, state.curY, state.curShape)) {
                        state.curX--;
                        audio.move();
                        render();
                    }
                    break;
                case 'ArrowRight':
                    e.preventDefault();
                    if (!checkCollide(state.curX + 1, state.curY, state.curShape)) {
                        state.curX++;
                        audio.move();
                        render();
                    }
                    break;
                case 'ArrowDown':
                    e.preventDefault();
                    softDrop();
                    break;
                case 'ArrowUp':
                    e.preventDefault();
                    rotate();
                    render();
                    break;
                case ' ':
                    e.preventDefault();
                    hardDrop();
                    break;
                case 'c':
                case 'C':
                    e.preventDefault();
                    swapHold();
                    break;
            }
        });
    }

    function init() {
        bindEvents();
        loadLocalHighScore();
    }

    return { init };
})();

window.addEventListener('DOMContentLoaded', TetrisGame.init);