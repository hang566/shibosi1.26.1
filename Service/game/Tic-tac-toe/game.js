/**
 * 智能井字棋游戏 - 模块化实现
 * 支持双人对战、人机对战、自定义棋盘大小、多难度AI
 */

/**
 * 游戏核心类
 */
class TicTacToeGame {
    /**
     * 构造函数
     * @param {Object} options 游戏配置选项
     * @param {number} options.size 棋盘尺寸，默认3
     * @param {number} options.winLine 获胜连子数，默认3
     * @param {string} options.mode 游戏模式：'pvp' | 'ai'
     * @param {string} options.aiDifficulty AI难度：'easy' | 'medium' | 'hard'
     */
    constructor(options = {}) {
        this.size = options.size || 3;
        this.winLine = options.winLine || 3;
        this.mode = options.mode || 'pvp';
        this.aiDifficulty = options.aiDifficulty || 'medium';
        
        // 游戏状态
        this.board = [];
        this.currentPlayer = 'X';
        this.gameOver = false;
        this.history = [];
        this.score = { X: 0, O: 0 };
        
        // 四个方向：水平、垂直、主对角线、副对角线
        this.directions = [[1, 0], [0, 1], [1, 1], [1, -1]];
        
        // DOM元素缓存
        this.$ = (selector) => document.querySelector(selector);
        this.$$ = (selector) => document.querySelectorAll(selector);
        
        // 绑定事件
        this.bindEvents();
        
        // 初始化渲染
        this.renderBoard();
    }
    
    /**
     * 绑定DOM事件
     */
    bindEvents() {
        this.$('#createBtn')?.addEventListener('click', () => this.handleCreateBoard());
        this.$('#resetBtn')?.addEventListener('click', () => this.resetBoard());
        this.$('#undoBtn')?.addEventListener('click', () => this.undoMove());
        this.$('#hintBtn')?.addEventListener('click', () => this.showHint());
        this.$('#boardSize')?.addEventListener('change', (e) => this.validateBoardSize(e));
        this.$('#winCount')?.addEventListener('change', (e) => this.validateWinCount(e));
    }
    
    /**
     * 验证棋盘尺寸
     */
    validateBoardSize(event) {
        let value = parseInt(event.target.value);
        value = Math.max(3, Math.min(10, value));
        event.target.value = value;
        
        // 更新获胜连子数限制
        const winCountInput = this.$('#winCount');
        if (parseInt(winCountInput.value) > value) {
            winCountInput.value = value;
            this.winLine = value;
        }
    }
    
    /**
     * 验证获胜连子数
     */
    validateWinCount(event) {
        let value = parseInt(event.target.value);
        const maxValue = parseInt(this.$('#boardSize').value);
        value = Math.max(3, Math.min(maxValue, value));
        event.target.value = value;
    }
    
    /**
     * 处理生成棋盘
     */
    handleCreateBoard() {
        this.size = parseInt(this.$('#boardSize').value) || 3;
        this.winLine = parseInt(this.$('#winCount').value) || 3;
        this.aiDifficulty = this.$('#aiDifficulty')?.value || 'medium';
        
        if (this.winLine > this.size) {
            this.winLine = this.size;
            this.$('#winCount').value = this.size;
        }
        
        this.renderBoard();
    }
    
    /**
     * 渲染棋盘
     */
    renderBoard() {
        const boardElement = this.$('#board');
        if (!boardElement) return;
        
        // 清空棋盘
        boardElement.innerHTML = '';
        
        // 设置网格列数
        boardElement.style.gridTemplateColumns = `repeat(${this.size}, ${this.getCellSize()}px)`;
        
        // 初始化二维数组
        this.board = Array(this.size).fill(null).map(() => Array(this.size).fill(null));
        this.history = [];
        this.gameOver = false;
        this.currentPlayer = 'X';
        
        // 清除提示
        this.clearHint();
        
        // 更新状态
        this.updateStatus();
        
        // 创建格子
        for (let y = 0; y < this.size; y++) {
            for (let x = 0; x < this.size; x++) {
                const cell = document.createElement('div');
                cell.className = 'cell';
                cell.dataset.x = x;
                cell.dataset.y = y;
                cell.addEventListener('click', () => this.clickCell(x, y));
                boardElement.appendChild(cell);
            }
        }
        
        // 更新历史记录显示
        this.updateHistoryDisplay();
    }
    
    /**
     * 根据棋盘大小获取格子尺寸
     */
    getCellSize() {
        if (this.size <= 4) return 55;
        if (this.size <= 6) return 48;
        if (this.size <= 8) return 42;
        return 36;
    }
    
    /**
     * 处理格子点击
     * @param {number} x 列坐标
     * @param {number} y 行坐标
     */
    clickCell(x, y) {
        // 游戏已结束或格子已有棋子
        if (this.gameOver || this.board[y][x]) return;
        
        // AI模式下，电脑回合不能点击
        if (this.mode === 'ai' && this.currentPlayer === 'O') return;
        
        // 清除提示
        this.clearHint();
        
        // 落子
        this.setCell(x, y, this.currentPlayer);
        this.history.push({ x, y, player: this.currentPlayer, turn: this.history.length + 1 });
        
        // 检查胜利
        if (this.checkWin(x, y, this.currentPlayer)) {
            this.gameOver = true;
            this.score[this.currentPlayer]++;
            this.updateScore();
            this.highlightWinner(x, y, this.currentPlayer);
            this.updateStatus(`${this.currentPlayer} 获胜！`);
            return;
        }
        
        // 检查平局
        if (this.isBoardFull()) {
            this.gameOver = true;
            this.updateStatus('平局！棋盘已满');
            return;
        }
        
        // 切换玩家
        this.switchPlayer();
        
        // AI模式下，电脑自动下棋
        if (this.mode === 'ai' && this.currentPlayer === 'O') {
            setTimeout(() => this.aiPlay(), 400);
        }
        
        // 更新历史记录显示
        this.updateHistoryDisplay();
    }
    
    /**
     * 设置格子内容
     * @param {number} x 列坐标
     * @param {number} y 行坐标
     * @param {string} player 玩家标识 'X' 或 'O'
     */
    setCell(x, y, player) {
        this.board[y][x] = player;
        const cell = this.$(`.cell[data-x="${x}"][data-y="${y}"]`);
        if (cell) {
            cell.textContent = player;
            cell.className = `cell ${player}`;
        }
    }
    
    /**
     * 清除格子内容
     * @param {number} x 列坐标
     * @param {number} y 行坐标
     */
    clearCell(x, y) {
        this.board[y][x] = null;
        const cell = this.$(`.cell[data-x="${x}"][data-y="${y}"]`);
        if (cell) {
            cell.textContent = '';
            cell.className = 'cell';
        }
    }
    
    /**
     * 切换当前玩家
     */
    switchPlayer() {
        this.currentPlayer = this.currentPlayer === 'X' ? 'O' : 'X';
        this.updateStatus();
    }
    
    /**
     * 更新状态显示
     * @param {string} text 自定义状态文本
     */
    updateStatus(text = null) {
        const statusElement = this.$('#status');
        if (statusElement) {
            statusElement.textContent = text || `当前落子：${this.currentPlayer}`;
        }
    }
    
    /**
     * 更新分数显示
     */
    updateScore() {
        const scoreX = this.$('#scoreX');
        const scoreO = this.$('#scoreO');
        if (scoreX) scoreX.textContent = this.score.X;
        if (scoreO) scoreO.textContent = this.score.O;
    }
    
    /**
     * 检查棋盘是否已满
     */
    isBoardFull() {
        return this.board.every(row => row.every(cell => cell !== null));
    }
    
    /**
     * 检查是否获胜
     * @param {number} cx 落子列坐标
     * @param {number} cy 落子行坐标
     * @param {string} player 玩家标识
     */
    checkWin(cx, cy, player) {
        for (const [dx, dy] of this.directions) {
            let count = 1;
            
            // 正向检查
            let x = cx + dx;
            let y = cy + dy;
            while (this.isValidPosition(x, y) && this.board[y][x] === player) {
                count++;
                x += dx;
                y += dy;
            }
            
            // 反向检查
            x = cx - dx;
            y = cy - dy;
            while (this.isValidPosition(x, y) && this.board[y][x] === player) {
                count++;
                x -= dx;
                y -= dy;
            }
            
            if (count >= this.winLine) {
                return true;
            }
        }
        return false;
    }
    
    /**
     * 验证坐标是否有效
     * @param {number} x 列坐标
     * @param {number} y 行坐标
     */
    isValidPosition(x, y) {
        return x >= 0 && x < this.size && y >= 0 && y < this.size;
    }
    
    /**
     * 高亮获胜棋子
     * @param {number} cx 获胜落子列坐标
     * @param {number} cy 获胜落子行坐标
     * @param {string} player 获胜玩家
     */
    highlightWinner(cx, cy, player) {
        for (const [dx, dy] of this.directions) {
            const winnerCells = [{ x: cx, y: cy }];
            
            let x = cx + dx;
            let y = cy + dy;
            while (this.isValidPosition(x, y) && this.board[y][x] === player) {
                winnerCells.push({ x, y });
                x += dx;
                y += dy;
            }
            
            x = cx - dx;
            y = cy - dy;
            while (this.isValidPosition(x, y) && this.board[y][x] === player) {
                winnerCells.push({ x, y });
                x -= dx;
                y -= dy;
            }
            
            if (winnerCells.length >= this.winLine) {
                winnerCells.forEach(pos => {
                    const cell = this.$(`.cell[data-x="${pos.x}"][data-y="${pos.y}"]`);
                    cell?.classList.add('winner');
                });
                break;
            }
        }
    }
    
    /**
     * 重置棋盘
     */
    resetBoard() {
        this.renderBoard();
    }
    
    /**
     * 悔棋
     */
    undoMove() {
        if (!this.history.length || this.gameOver) return;
        
        this.clearHint();
        
        const lastMove = this.history.pop();
        this.clearCell(lastMove.x, lastMove.y);
        this.currentPlayer = lastMove.player;
        this.gameOver = false;
        
        this.updateStatus();
        this.updateHistoryDisplay();
    }
    
    /**
     * 显示智能提示
     */
    showHint() {
        if (this.gameOver) {
            this.updateStatus('游戏已结束，重置棋盘再使用提示');
            return;
        }
        
        this.clearHint();
        
        const me = this.currentPlayer;
        const enemy = me === 'X' ? 'O' : 'X';
        const target = this.getBestPoint(me, enemy, 'hard') || this.getRandomEmpty();
        
        if (target) {
            const cell = this.$(`.cell[data-x="${target.x}"][data-y="${target.y}"]`);
            if (cell) {
                cell.classList.add('hint');
                this.updateStatus('黄色高亮=最优落子，可赢/防对手绝杀');
            }
        }
    }
    
    /**
     * 清除提示
     */
    clearHint() {
        this.$$('.cell').forEach(cell => cell.classList.remove('hint'));
    }
    
    /**
     * 更新历史记录显示
     */
    updateHistoryDisplay() {
        const historyElement = this.$('#historyList');
        if (!historyElement) return;
        
        if (this.history.length === 0) {
            historyElement.innerHTML = '<div>暂无记录</div>';
            return;
        }
        
        historyElement.innerHTML = this.history
            .map((move, index) => {
                const playerColor = move.player === 'X' ? '#4cc9f0' : '#f72585';
                return `<div>第${move.turn}步: <span style="color:${playerColor}">${move.player}</span> (${move.x + 1}, ${move.y + 1})</div>`;
            })
            .join('');
        
        // 滚动到底部
        historyElement.scrollTop = historyElement.scrollHeight;
    }
    
    /**
     * AI下棋
     */
    aiPlay() {
        if (this.gameOver) return;
        
        let pos;
        
        switch (this.aiDifficulty) {
            case 'easy':
                // 简单模式：随机落子
                pos = this.getRandomEmpty();
                break;
            case 'medium':
                // 中等模式：有50%概率使用高级策略
                pos = Math.random() > 0.5 
                    ? this.getBestPoint('O', 'X', 'medium') 
                    : this.getRandomEmpty();
                break;
            case 'hard':
            default:
                // 困难模式：使用高级AI算法
                pos = this.getBestPoint('O', 'X', 'hard');
                break;
        }
        
        if (!pos) return;
        
        this.setCell(pos.x, pos.y, 'O');
        this.history.push({ x: pos.x, y: pos.y, player: 'O', turn: this.history.length + 1 });
        
        // 检查胜利
        if (this.checkWin(pos.x, pos.y, 'O')) {
            this.gameOver = true;
            this.score.O++;
            this.updateScore();
            this.highlightWinner(pos.x, pos.y, 'O');
            this.updateStatus('电脑(O)获胜！');
            return;
        }
        
        // 检查平局
        if (this.isBoardFull()) {
            this.gameOver = true;
            this.updateStatus('平局！棋盘已满');
            return;
        }
        
        this.switchPlayer();
        this.updateHistoryDisplay();
    }
    
    /**
     * 获取随机空位
     */
    getRandomEmpty() {
        const empty = [];
        for (let y = 0; y < this.size; y++) {
            for (let x = 0; x < this.size; x++) {
                if (!this.board[y][x]) {
                    empty.push({ x, y });
                }
            }
        }
        return empty.length ? empty[Math.floor(Math.random() * empty.length)] : null;
    }
    
    /**
     * 获取最佳落子点（AI核心算法）
     * @param {string} me 当前玩家
     * @param {string} enemy 对手玩家
     * @param {string} difficulty 难度级别
     */
    getBestPoint(me, enemy, difficulty) {
        let winPoints = [];      // 能直接获胜的点
        let blockPoints = [];    // 需要防守的点
        let goodPoints = [];     // 有进攻价值的点
        let centerPoints = [];   // 中心点
        let edgePoints = [];     // 边缘点
        
        const center = Math.floor(this.size / 2);
        
        for (let y = 0; y < this.size; y++) {
            for (let x = 0; x < this.size; x++) {
                if (this.board[y][x]) continue;
                
                // 检查是否能获胜
                this.board[y][x] = me;
                if (this.checkWin(x, y, me)) {
                    winPoints.push({ x, y });
                    this.board[y][x] = null;
                    continue;
                }
                this.board[y][x] = null;
                
                // 检查是否需要防守
                this.board[y][x] = enemy;
                if (this.checkWin(x, y, enemy)) {
                    blockPoints.push({ x, y });
                    this.board[y][x] = null;
                    continue;
                }
                this.board[y][x] = null;
                
                // 计算位置评分（困难模式）
                if (difficulty === 'hard') {
                    const score = this.calculatePositionScore(x, y, me, enemy);
                    const pos = { x, y, score };
                    
                    if (score >= 3) {
                        goodPoints.push(pos);
                    } else if (x === center && y === center) {
                        centerPoints.push(pos);
                    } else if (this.isCorner(x, y)) {
                        // 角落位置优先
                        goodPoints.push({ ...pos, score: pos.score + 0.5 });
                    } else {
                        edgePoints.push(pos);
                    }
                } else {
                    // 中等模式：简化评分
                    if (x === center && y === center) {
                        centerPoints.push({ x, y });
                    } else if (this.isCorner(x, y)) {
                        goodPoints.push({ x, y });
                    } else {
                        edgePoints.push({ x, y });
                    }
                }
            }
        }
        
        // 优先级：必胜 > 必防 > 高价值点 > 中心 > 边缘
        if (winPoints.length) return winPoints[0];
        if (blockPoints.length) return blockPoints[0];
        if (goodPoints.length) {
            goodPoints.sort((a, b) => b.score - a.score);
            return goodPoints[0];
        }
        if (centerPoints.length) return centerPoints[0];
        if (edgePoints.length) {
            edgePoints.sort((a, b) => (b.score || 0) - (a.score || 0));
            return edgePoints[0];
        }
        
        return null;
    }
    
    /**
     * 判断是否是角落位置
     * @param {number} x 列坐标
     * @param {number} y 行坐标
     */
    isCorner(x, y) {
        return (x === 0 && y === 0) ||
               (x === 0 && y === this.size - 1) ||
               (x === this.size - 1 && y === 0) ||
               (x === this.size - 1 && y === this.size - 1);
    }
    
    /**
     * 计算位置评分（高级AI算法）
     * @param {number} x 列坐标
     * @param {number} y 行坐标
     * @param {string} me 当前玩家
     * @param {string} enemy 对手玩家
     */
    calculatePositionScore(x, y, me, enemy) {
        let score = 0;
        const weights = {
            winThreat: 100,    // 成四威胁
            blockThreat: 90,   // 防守威胁
            threeInRow: 20,    // 三连
            blockThree: 15,    // 堵三连
            twoInRow: 5,       // 二连
            center: 3,         // 中心加成
            corner: 2          // 角落加成
        };
        
        // 中心位置加成
        const center = Math.floor(this.size / 2);
        if (x === center && y === center) {
            score += weights.center;
        }
        
        // 角落位置加成
        if (this.isCorner(x, y)) {
            score += weights.corner;
        }
        
        // 检查四个方向的连子情况
        for (const [dx, dy] of this.directions) {
            const lineInfo = this.getLineInfo(x, y, dx, dy, me, enemy);
            score += this.evaluateLine(lineInfo, weights);
        }
        
        return score;
    }
    
    /**
     * 获取某方向的连子信息
     * @param {number} x 列坐标
     * @param {number} y 行坐标
     * @param {number} dx 方向增量x
     * @param {number} dy 方向增量y
     * @param {string} me 当前玩家
     * @param {string} enemy 对手玩家
     */
    getLineInfo(x, y, dx, dy, me, enemy) {
        let myCount = 0;
        let enemyCount = 0;
        let myOpenEnds = 0;
        let enemyOpenEnds = 0;
        
        // 正向检查
        let nx = x + dx;
        let ny = y + dy;
        while (this.isValidPosition(nx, ny)) {
            const cell = this.board[ny][nx];
            if (cell === me) {
                myCount++;
            } else if (cell === enemy) {
                enemyCount++;
                break;
            } else {
                myOpenEnds++;
                break;
            }
            nx += dx;
            ny += dy;
        }
        
        // 反向检查
        nx = x - dx;
        ny = y - dy;
        while (this.isValidPosition(nx, ny)) {
            const cell = this.board[ny][nx];
            if (cell === me) {
                myCount++;
            } else if (cell === enemy) {
                enemyCount++;
                break;
            } else {
                myOpenEnds++;
                break;
            }
            nx -= dx;
            ny -= dy;
        }
        
        return {
            myCount,
            enemyCount,
            myOpenEnds,
            enemyOpenEnds
        };
    }
    
    /**
     * 评估连线价值
     * @param {Object} lineInfo 连线信息
     * @param {Object} weights 权重配置
     */
    evaluateLine(lineInfo, weights) {
        let score = 0;
        const { myCount, enemyCount, myOpenEnds, enemyOpenEnds } = lineInfo;
        
        // 进攻评估
        if (myCount >= this.winLine - 1 && myOpenEnds >= 1) {
            score += weights.winThreat;
        } else if (myCount === this.winLine - 2 && myOpenEnds >= 2) {
            score += weights.threeInRow;
        } else if (myCount === this.winLine - 3 && myOpenEnds >= 2) {
            score += weights.twoInRow;
        }
        
        // 防守评估
        if (enemyCount >= this.winLine - 1 && enemyOpenEnds >= 1) {
            score += weights.blockThreat;
        } else if (enemyCount === this.winLine - 2 && enemyOpenEnds >= 2) {
            score += weights.blockThree;
        }
        
        return score;
    }
    
    /**
     * 设置游戏模式
     * @param {string} newMode 'pvp' | 'ai'
     */
    setMode(newMode) {
        this.mode = newMode;
        this.renderBoard();
    }
    
    /**
     * 重置计分
     */
    resetScore() {
        this.score = { X: 0, O: 0 };
        this.updateScore();
    }
}

// 页面切换逻辑
class PageManager {
    constructor() {
        this.startMask = document.getElementById('startMask');
        this.gameMask = document.getElementById('gameMask');
        this.game = null;
        
        this.bindNavigationEvents();
    }
    
    bindNavigationEvents() {
        document.getElementById('goPvp')?.addEventListener('click', () => {
            this.switchToGame('pvp', '双人对战');
        });
        
        document.getElementById('goAi')?.addEventListener('click', () => {
            this.switchToGame('ai', '对战智能电脑');
        });
        
        document.getElementById('backHome')?.addEventListener('click', () => {
            this.switchToStart();
        });
    }
    
    switchToGame(mode, modeText) {
        document.getElementById('modeText').value = modeText;
        this.startMask.classList.remove('active');
        this.gameMask.classList.add('active');
        
        // 创建游戏实例
        this.game = new TicTacToeGame({
            size: 3,
            winLine: 3,
            mode: mode,
            aiDifficulty: 'medium'
        });
    }
    
    switchToStart() {
        this.gameMask.classList.remove('active');
        this.startMask.classList.add('active');
        
        // 重置分数
        if (this.game) {
            this.game.resetScore();
        }
    }
}

// 初始化页面管理器
function initApp() {
    try {
        new PageManager();
        console.log('Tic-Tac-Toe game initialized successfully');
    } catch (error) {
        console.error('Failed to initialize game:', error);
    }
}

// DOM加载完成后初始化
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initApp);
} else {
    initApp();
}