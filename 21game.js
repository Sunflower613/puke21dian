// 21点游戏 - WebSocket客户端

class BlackjackGame {
    constructor() {
        this.ws = null;
        this.playerId = null;
        this.nickname = '玩家' + Math.floor(Math.random() * 1000);
        this.roomId = null;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 5;

        this.init();
    }

    init() {
        // 从URL获取房间ID
        const urlParams = new URLSearchParams(window.location.search);
        this.roomId = urlParams.get('roomId');

        if (!this.roomId) {
            alert('房间ID不存在');
            window.location.href = '21dian.html';
            return;
        }

        // 更新房间显示
        document.getElementById('room-id').textContent = `{${this.roomId}}`;

        // 绑定按钮事件
        document.getElementById('hit-button').addEventListener('click', () => this.hit());
        document.getElementById('stand-button').addEventListener('click', () => this.stand());
        document.getElementById('send-button').addEventListener('click', () => this.sendMessage());
        document.getElementById('message').addEventListener('keypress', (e) => {
            if (e.key === 'Enter') this.sendMessage();
        });

        // 连接WebSocket
        this.connect();

        // 请求开始游戏
        setTimeout(() => {
            this.send({ type: 'start', data: { roomId: this.roomId, playerId: this.playerId } });
        }, 1000);
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
            this.send({ type: 'connect', data: { playerId: this.playerId, nickname: this.nickname } });

            // 加入房间
            this.send({ type: 'join', data: { roomId: this.roomId, playerId: this.playerId, nickname: this.nickname } });
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
                break;

            case 'roomInfo':
                console.log('🏠 房间信息:', message.data);
                break;

            case 'players':
                this.updatePlayers(message.data.players);
                break;

            case 'update':
                this.updatePlayer(message.data);
                break;

            case 'chat':
                this.addChatMessage(message.data);
                break;

            case 'start':
                console.log('🎮 游戏开始');
                this.updateStatus('游戏进行中', 'yellow');
                this.enableButtons(true);
                break;

            case 'gameEnd':
                this.handleGameEnd(message.data);
                break;

            case 'error':
                console.error('❌ 错误:', message.error);
                alert('错误: ' + message.error);
                break;

            default:
                console.log('❓ 未知消息类型:', message.type);
        }
    }

    hit() {
        this.send({ type: 'hit', data: { roomId: this.roomId, playerId: this.playerId } });
    }

    stand() {
        this.send({ type: 'stand', data: { roomId: this.roomId, playerId: this.playerId } });
    }

    sendMessage() {
        const input = document.getElementById('message');
        const message = input.value.trim();

        if (message) {
            this.send({ type: 'chat', data: { roomId: this.roomId, playerId: this.playerId, message: message } });
            input.value = '';
        }
    }

    updatePlayers(players) {
        const playersDiv = document.getElementById('players');
        playersDiv.innerHTML = '';

        players.forEach(player => {
            const isSelf = player.id === this.playerId;
            const playerDiv = document.createElement('div');
            playerDiv.className = 'player' + (isSelf ? ' player-self' : '');
            playerDiv.id = isSelf ? 'player-self' : `player-${player.id}`;

            const cardsHtml = player.cards.map(card => `<div class="card ${card}"></div>`).join('');

            playerDiv.innerHTML = `
                ${player.nickname}的牌: {${player.cardCount}}张 ${player.status === '操作中' ? '?' : player.handValue} 分
                <span class="status" style="color: ${player.statusColor}">${player.status}</span>
                <div class="cards">${cardsHtml}</div>
            `;

            playersDiv.appendChild(playerDiv);
        });
    }

    updatePlayer(player) {
        const playerDiv = document.getElementById(`player-${player.id}`) || document.getElementById('player-self');
        if (playerDiv) {
            const cardsHtml = player.cards.map(card => `<div class="card ${card}"></div>`).join('');
            const isSelf = player.id === this.playerId;
            const displayValue = isSelf || player.status !== '操作中' ? player.handValue : '?';

            playerDiv.innerHTML = `
                ${player.nickname}的牌: {${player.cardCount}}张 ${displayValue} 分
                <span class="status" style="color: ${player.statusColor}">${player.status}</span>
                <div class="cards">${cardsHtml}</div>
            `;

            // 如果是自己爆牌了，禁用按钮
            if (isSelf && player.status === '爆牌') {
                this.enableButtons(false);
            }
        }
    }

    handleGameEnd(data) {
        console.log('🏁 游戏结束:', data);

        // 禁用按钮
        this.enableButtons(false);

        // 显示结果
        let resultHtml = '<div style="margin-top: 20px; padding: 15px; background: #34495e; border-radius: 5px;">';
        resultHtml += '<h3>🏆 游戏结果</h3>';

        data.results.forEach(result => {
            const statusClass = result.isWinner ? 'green' : (result.status === '已爆牌' ? 'red' : 'gray');
            const winnerIcon = result.isWinner ? '👑 ' : '';
            resultHtml += `<div style="margin: 10px 0; color: ${statusClass};">
                ${winnerIcon}${result.nickname}: ${result.score}分 (${result.status})
            </div>`;
        });

        resultHtml += '</div>';

        const statusDiv = document.getElementById('status');
        statusDiv.innerHTML = resultHtml;
    }

    addChatMessage(data) {
        const chatMessages = document.getElementById('chat-messages');
        const msgDiv = document.createElement('div');
        msgDiv.style.margin = '5px 0';
        msgDiv.textContent = `${data.nickname}: ${data.message}`;
        chatMessages.appendChild(msgDiv);
        chatMessages.scrollTop = chatMessages.scrollHeight;
    }

    updateStatus(text, color) {
        const statusDiv = document.getElementById('status');
        statusDiv.textContent = text;
        statusDiv.style.color = color === 'green' ? '#2ecc71' : (color === 'red' ? '#e74c3c' : (color === 'yellow' ? '#f1c40f' : '#ecf0f1'));
    }

    enableButtons(enabled) {
        const hitButton = document.getElementById('hit-button');
        const standButton = document.getElementById('stand-button');

        if (hitButton) hitButton.disabled = !enabled;
        if (standButton) standButton.disabled = !enabled;

        hitButton.style.opacity = enabled ? '1' : '0.5';
        standButton.style.opacity = enabled ? '1' : '0.5';
    }
}

// 页面加载完成后初始化游戏
document.addEventListener('DOMContentLoaded', () => {
    window.game = new BlackjackGame();
});
