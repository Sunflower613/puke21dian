// 21点游戏 - WebSocket客户端

class BlackjackGame {
    constructor() {
        this.ws = null;
        this.playerId = this.getOrCreatePlayerId();
        this.nickname = '玩家' + Math.floor(Math.random() * 1000);
        this.roomId = null;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 5;
        this.isHost = false;
        this.gameStarted = false;
        this.accountName = '';
        this.accountScore = 0;
        this.prevCardCounts = {}; // 记录每个玩家上次的牌数，用于只给新牌加动画

        this.init();
    }

    // 获取或创建唯一的玩家ID
    getOrCreatePlayerId() {
        let playerId = sessionStorage.getItem('blackjack_player_id');
        if (!playerId) {
            const timestamp = Date.now();
            const random = Math.floor(Math.random() * 1000000);
            playerId = `player_${timestamp}_${random}`;
            sessionStorage.setItem('blackjack_player_id', playerId);
        }
        console.log('playerId', playerId);
        return playerId;
    }

    init() {
        // 从URL获取房间ID和昵称
        const urlParams = new URLSearchParams(window.location.search);
        this.roomId = urlParams.get('roomId');
        const urlNickname = urlParams.get('nickname');
        const urlAccount = urlParams.get('account');

        if (!this.roomId) {
            alert('房间ID不存在');
            window.location.href = '21dian.html';
            return;
        }

        // 设置昵称
        if (urlNickname) {
            this.nickname = decodeURIComponent(urlNickname);
        }
        if (urlAccount) {
            this.accountName = decodeURIComponent(urlAccount);
            this.playerId = `account_${this.accountName}`;
        } else {
            const storedAccount = this.getStoredAccount();
            if (storedAccount && !urlNickname) {
                this.accountName = storedAccount.username;
                this.nickname = storedAccount.username;
                this.playerId = `account_${this.accountName}`;
            }
        }

        // 更新房间显示
        document.getElementById('room-id').textContent = this.roomId;
        this.updateAccountScoreDisplay();

        // 绑定按钮事件
        document.getElementById('hit-button').addEventListener('click', () => this.hit());
        document.getElementById('stand-button').addEventListener('click', () => this.stand());
        document.getElementById('send-button').addEventListener('click', () => this.sendMessage());
        document.getElementById('start-game-button').addEventListener('click', () => this.startGame());
        document.getElementById('add-bot-button').addEventListener('click', () => this.addBot());
        document.getElementById('message').addEventListener('keypress', (e) => {
            if (e.key === 'Enter') this.sendMessage();
        });

        // 连接WebSocket
        this.connect();
    }

    startGame() {
        this.send({ type: 'start', data: { roomId: this.roomId, playerId: this.playerId } });
    }

    addBot() {
        this.send({ type: 'addBot', data: { roomId: this.roomId, playerId: this.playerId } });
    }

    getStoredAccount() {
        try {
            return JSON.parse(localStorage.getItem('blackjack_account'));
        } catch (e) {
            return null;
        }
    }

    updateAccountScoreDisplay(score = this.accountScore) {
        this.accountScore = score || 0;
        const scoreEl = document.getElementById('account-score');
        if (scoreEl) scoreEl.textContent = this.accountScore;
    }

    connect() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;

        this.ws = new WebSocket(wsUrl);

        this.ws.onopen = () => {
            console.log('✅ WebSocket连接成功');
            this.reconnectAttempts = 0;
            this.updateStatus('已连接', 'green');

            // 发送连接消息
            this.send({ type: 'connect', data: { playerId: this.playerId, nickname: this.nickname, accountName: this.accountName } });

            // 加入房间
            this.send({ type: 'join', data: { roomId: this.roomId, playerId: this.playerId, nickname: this.nickname, accountName: this.accountName } });
        };

        this.ws.onmessage = (event) => {
            const message = JSON.parse(event.data);
            this.handleMessage(message);
        };

        this.ws.onerror = (error) => {
            console.error('❌ WebSocket错误:', error);
        };

        this.ws.onclose = () => {
            console.log('🔌 WebSocket连接关闭');
            this.updateStatus('连接断开，尝试重连...', 'red');
            this.tryReconnect();
        };
    }

    tryReconnect() {
        if (this.reconnectAttempts < this.maxReconnectAttempts) {
            this.reconnectAttempts++;
            console.log(`🔄 尝试重连 (${this.reconnectAttempts}/${this.maxReconnectAttempts})...`);
            setTimeout(() => this.connect(), 3000);
        } else {
            this.updateStatus('无法连接到服务器', 'red');
            alert('无法连接到服务器，请刷新页面重试');
        }
    }

    send(message) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify(message));
        } else {
            console.warn('⚠️ WebSocket未连接，无法发送消息');
        }
    }

    handleMessage(message) {
        console.log('📨 收到消息:', message);

        switch (message.type) {
            case 'connect':
                console.log('✅ 已连接，玩家ID:', message.data.playerId);
                break;

            case 'join':
                console.log('✅ 已加入房间');
                this.updateStatus('等待游戏开始...', 'gray');
                this.showWaitingArea();
                break;

            case 'roomInfo':
                console.log('🏠 房间信息:', message.data);
                if (message.data.error) {
                    alert(message.data.error);
                    window.location.href = '21dian.html';
                } else if (message.data.status === 1 && !message.data.isExisting) {
                    alert('游戏已经开始，无法加入！');
                    window.location.href = '21dian.html';
                } else if (message.data.status === 1 && message.data.isExisting) {
                    console.log('🔄 重连成功，恢复游戏状态');
                    this.gameStarted = true;
                    this.updateStatus('游戏进行中', 'yellow');
                    document.getElementById('waiting-area').style.display = 'none';
                    document.getElementById('players').style.display = '';
                    document.getElementById('game-actions').style.display = 'flex';
                }
                break;

            case 'players':
                if (this.gameStarted) {
                    this.updatePlayers(message.data.players);
                } else {
                    this.updateWaitingPlayers(message.data.players);
                }
                break;

            case 'update':
                this.updatePlayer(message.data);
                break;

            case 'chat':
                this.addChatMessage(message.data);
                break;

            case 'start':
                console.log('🎮 游戏开始');
                this.gameStarted = true;
                this.updateStatus('游戏进行中', 'yellow');
                this.enableButtons(true);
                // 隐藏等待区域，显示游戏区域
                document.getElementById('waiting-area').style.display = 'none';
                document.getElementById('players').style.display = '';
                document.getElementById('game-actions').style.display = 'flex';
                break;

            case 'gameEnd':
                this.handleGameEnd(message.data);
                break;

            case 'error':
                console.error('❌ 错误:', message.error);
                const errMsg = message.error || '';
                // 房间不存在时返回主页
                if (errMsg.includes('房间不存在') || errMsg.includes('不存在') || errMsg.includes('not found')) {
                    alert(errMsg);
                    window.location.href = '21dian.html';
                } else if (typeof showToast === 'function') {
                    showToast('错误: ' + errMsg);
                } else {
                    alert('错误: ' + errMsg);
                }
                break;

            default:
                console.log('❓ 未知消息类型:', message.type);
        }
    }

    hit() {
        this.send({ type: 'hit', data: { roomId: this.roomId, playerId: this.playerId } });
        this.enableButtons(false);
    }

    stand() {
        this.send({ type: 'stand', data: { roomId: this.roomId, playerId: this.playerId } });
        this.enableButtons(false);
    }

    sendMessage() {
        const input = document.getElementById('message');
        const message = input.value.trim();

        if (message) {
            this.send({ type: 'chat', data: { roomId: this.roomId, playerId: this.playerId, message: message } });
            input.value = '';
        }
    }

    // ===== 获取状态样式 =====
    getStatusClass(status) {
        const map = {
            '等待中': 'status-waiting',
            '操作中': 'status-acting',
            '已停牌': 'status-stood',
            '已爆牌': 'status-bust',
            '爆牌': 'status-bust',
        };
        return map[status] || 'status-waiting';
    }

    getStatusColor(status) {
        const map = {
            '等待中': '#94a3b8',
            '操作中': '#f59e0b',
            '已停牌': '#10b981',
            '已爆牌': '#ef4444',
            '爆牌': '#ef4444',
        };
        return map[status] || '#94a3b8';
    }

    // ===== 更新游戏中的玩家列表 =====
    updatePlayers(players) {
        const playersDiv = document.getElementById('players');
        playersDiv.innerHTML = '';

        players.forEach((player, idx) => {
            const isSelf = player.id === this.playerId;
            const playerDiv = document.createElement('div');
            playerDiv.className = 'player' + (isSelf ? ' player-self' : '');
            playerDiv.id = isSelf ? 'player-self' : `player-${player.id}`;
            playerDiv.style.animationDelay = `${idx * 0.05}s`;

            const prevCount = this.prevCardCounts[player.id] || 0;
            const cardsHtml = player.cards.map((card, ci) => {
                const isNew = ci >= prevCount;
                const animStyle = isNew ? 'animation-delay:' + ((ci - prevCount) * 0.08) + 's' : 'animation:none';
                return `<div class="card-wrapper" style="z-index: ${ci + 1};"><div class="card ${card}" style="${animStyle}"></div></div>`;
            }).join('');
            this.prevCardCounts[player.id] = player.cards.length;

            // 显示点数逻辑：自己始终显示，其他人操作中或分数被隐藏(0)时显示?
            const displayValue = isSelf ? player.handValue : (player.status === '操作中' || player.handValue === 0 ? '?' : player.handValue);
            const statusClass = this.getStatusClass(player.status);
            const nameLabel = isSelf ? '我' : player.nickname;
            const botBadge = player.isBot ? '<span class="badge badge-bot">🤖</span>' : '';
            if (isSelf) this.updateAccountScoreDisplay(player.score);

            playerDiv.innerHTML = `
                <div class="player-header">
                    <div class="player-name-section">
                        <span class="player-name-text">${nameLabel}</span>
                        ${isSelf ? '<span class="badge badge-you">YOU</span>' : ''}${botBadge}
                    </div>
                    <div class="player-meta">
                        <span class="card-count">${player.cardCount}张</span>
                        <span class="account-score-badge">${player.score || 0}分</span>
                        <span class="score-badge">${displayValue}</span>
                        <span class="status ${statusClass}">${player.status}</span>
                    </div>
                </div>
                <div class="cards">${cardsHtml}</div>
            `;

            if (isSelf) {
                if (player.status === '操作中') {
                    this.enableButtons(true);
                } else {
                    this.enableButtons(false);
                }
            }

            playersDiv.appendChild(playerDiv);
        });
    }

    // ===== 更新单个玩家 =====
    updatePlayer(player) {
        const playerDiv = document.getElementById(`player-${player.id}`) || document.getElementById('player-self');
        if (playerDiv) {
            const prevCount = this.prevCardCounts[player.id] || 0;
            const cardsHtml = player.cards.map((card, ci) => {
                const isNew = ci >= prevCount;
                const animStyle = isNew ? 'animation-delay:' + ((ci - prevCount) * 0.08) + 's' : 'animation:none';
                return `<div class="card-wrapper" style="z-index: ${ci + 1};"><div class="card ${card}" style="${animStyle}"></div></div>`;
            }).join('');
            this.prevCardCounts[player.id] = player.cards.length;
            const isSelf = player.id === this.playerId;
            const displayValue = isSelf ? player.handValue : (player.status === '操作中' || player.handValue === 0 ? '?' : player.handValue);
            const statusClass = this.getStatusClass(player.status);
            const nameLabel = isSelf ? '我' : player.nickname;
            const botBadge = player.isBot ? '<span class="badge badge-bot">🤖</span>' : '';
            if (isSelf) this.updateAccountScoreDisplay(player.score);

            playerDiv.innerHTML = `
                <div class="player-header">
                    <div class="player-name-section">
                        <span class="player-name-text">${nameLabel}</span>
                        ${isSelf ? '<span class="badge badge-you">YOU</span>' : ''}${botBadge}
                    </div>
                    <div class="player-meta">
                        <span class="card-count">${player.cardCount}张</span>
                        <span class="account-score-badge">${player.score || 0}分</span>
                        <span class="score-badge">${displayValue}</span>
                        <span class="status ${statusClass}">${player.status}</span>
                    </div>
                </div>
                <div class="cards">${cardsHtml}</div>
            `;

            // 如果是自己，根据状态启用/禁用操作按钮
            if (isSelf) {
                if (player.status === '操作中') {
                    this.enableButtons(true);
                } else {
                    this.enableButtons(false);
                }
            }
        }
    }

    updatePlayerSelf(player) {
        const playerDiv = document.getElementById('player-self');
        if (playerDiv) {
            const cardsHtml = player.cards.map((card, ci) =>
                `<div class="card-wrapper" style="z-index: ${ci + 1};"><div class="card ${card}" style="animation-delay:${ci * 0.08}s"></div></div>`
            ).join('');
            playerDiv.innerHTML = `
                <div class="player-header">
                    <div class="player-name-section">
                        <span class="player-name-text">我</span>
                        <span class="badge badge-you">YOU</span>
                    </div>
                    <div class="player-meta">
                        <span class="card-count">${player.cardCount}张</span>
                        <span class="score-badge">${player.handValue}</span>
                    </div>
                </div>
                <div class="cards">${cardsHtml}</div>
            `;
        }
    }

    // ===== 游戏结束 =====
    handleGameEnd(data) {
        console.log('🏁 游戏结束:', data);

        // 禁用按钮
        this.enableButtons(false);

        // 构建结果面板
        let resultHtml = '<div class="result-panel">';
        resultHtml += '<h3>🏆 游戏结果</h3>';

        data.results.forEach(result => {
            const isWinner = result.isWinner;
            const statusColor = result.status === '已爆牌' ? 'var(--danger)' : (isWinner ? 'var(--gold-light)' : 'var(--text-secondary)');
            const winnerIcon = isWinner ? '👑 ' : '';
            const itemClass = isWinner ? 'result-item winner' : 'result-item';

            resultHtml += `
                <div class="${itemClass}">
                    <span>${winnerIcon}${result.nickname}</span>
                    <span>
                        <span class="result-score" style="color: ${statusColor}">${result.score}分</span>
                        <span style="color: var(--gold); font-size: 0.75rem; margin-left: 6px">总分 ${result.totalScore || 0}</span>
                        <span style="color: var(--text-muted); font-size: 0.75rem; margin-left: 6px">${result.status}</span>
                    </span>
                </div>`;
            if (result.accountName && result.accountName === this.accountName) {
                this.updateAccountScoreDisplay(result.totalScore);
                const storedAccount = this.getStoredAccount();
                if (storedAccount) {
                    storedAccount.score = result.totalScore || 0;
                    localStorage.setItem('blackjack_account', JSON.stringify(storedAccount));
                }
            }
        });

        resultHtml += '</div>';

        // 如果是房主，增加"重新开始"按钮
        if (this.isHost) {
            resultHtml += '<div style="text-align:center; margin-top: 16px;"><button id="restart-game-button" class="btn-success btn-lg btn-block">🔄 再来一局</button></div>';
        }

        const statusDiv = document.getElementById('status');
        statusDiv.innerHTML = resultHtml;

        // 绑定重新开始按钮
        if (this.isHost) {
            const restartBtn = document.getElementById('restart-game-button');
            if (restartBtn) {
                restartBtn.addEventListener('click', () => this.startGame());
            }
        }
    }

    // ===== 聊天 =====
    addChatMessage(data) {
        const chatMessages = document.getElementById('chat-messages');
        const msgDiv = document.createElement('div');
        msgDiv.className = 'chat-msg';
        msgDiv.innerHTML = `<strong>${data.nickname}:</strong> ${data.message}`;
        chatMessages.appendChild(msgDiv);
        chatMessages.scrollTop = chatMessages.scrollHeight;
    }

    // ===== 状态更新 =====
    updateStatus(text, color) {
        const statusDiv = document.getElementById('status');
        statusDiv.textContent = text;
        const colorMap = {
            'green': 'var(--success)',
            'red': 'var(--danger)',
            'yellow': 'var(--warning)',
            'gray': 'var(--text-secondary)',
        };
        statusDiv.style.color = colorMap[color] || 'var(--text)';
    }

    enableButtons(enabled) {
        const hitButton = document.getElementById('hit-button');
        const standButton = document.getElementById('stand-button');

        if (hitButton) hitButton.disabled = !enabled;
        if (standButton) standButton.disabled = !enabled;

        if (hitButton) hitButton.style.opacity = enabled ? '1' : '0.4';
        if (standButton) standButton.style.opacity = enabled ? '1' : '0.4';
    }

    showWaitingArea() {
        document.getElementById('waiting-area').style.display = 'flex';
        document.getElementById('players').style.display = 'none';
        document.getElementById('game-actions').style.display = 'none';
    }

    // ===== 等待区玩家列表 =====
    updateWaitingPlayers(players) {
        const playerListDiv = document.getElementById('player-list');
        const playerCountSpan = document.getElementById('player-count');
        const startButton = document.getElementById('start-game-button');

        // 更新玩家数量
        playerCountSpan.textContent = players.length;
        startButton.disabled = players.length < 2;

        // 判断是否是房主
        const addBotButton = document.getElementById('add-bot-button');
        if (players.length > 0 && players[0].id === this.playerId) {
            this.isHost = true;
            startButton.style.display = 'inline-flex';
            addBotButton.style.display = players.length < 6 ? 'inline-flex' : 'none';
        } else {
            this.isHost = false;
            startButton.style.display = 'none';
            addBotButton.style.display = 'none';
        }

        // 构建玩家列表
        playerListDiv.innerHTML = '<h3>房间玩家</h3>';
        players.forEach((player, index) => {
            const isSelf = player.id === this.playerId;
            const isHost = index === 0;

            const item = document.createElement('div');
            item.className = 'waiting-player-item' + (isSelf ? ' is-self' : '');

            let badges = '';
            if (isHost) badges += '<span class="badge badge-host">👑 房主</span>';
            if (player.isBot) badges += '<span class="badge badge-bot">🤖 人机</span>';
            if (isSelf) badges += '<span class="badge badge-you">你</span>';

            item.innerHTML = `
                <span class="waiting-player-name">${player.nickname}</span>
                <div class="waiting-player-badges">${badges}</div>
            `;

            playerListDiv.appendChild(item);
        });
    }
}

// 页面加载完成后初始化游戏
document.addEventListener('DOMContentLoaded', () => {
    window.game = new BlackjackGame();
});
