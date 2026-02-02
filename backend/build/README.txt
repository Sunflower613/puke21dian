21点游戏服务器 - 部署说明
================================

文件说明:
-----------
- blackjack-linux-amd64  - Linux x64可执行文件
- blackjack-linux-arm64  - Linux ARM64可执行文件（树莓派等）
- start.sh              - 启动脚本
- blackjack.service     - systemd服务文件

快速启动:
---------
1. 赋予执行权限:
   chmod +x blackjack-linux-amd64 start.sh

2. 运行服务器:
   ./start.sh

3. 访问游戏:
   http://localhost:8080

自定义端口:
---------
方法1: 使用环境变量
   PORT=3000 ./blackjack-linux-amd64

方法2: 修改start.sh中的PORT变量

作为系统服务安装:
------------------
1. 复制文件到目标目录:
   sudo mkdir -p /opt/blackjack
   sudo cp blackjack-linux-amd64 /opt/blackjack/
   sudo cp blackjack.service /etc/systemd/system/

2. 重载systemd并启动服务:
   sudo systemctl daemon-reload
   sudo systemctl enable blackjack
   sudo systemctl start blackjack

3. 查看服务状态:
   sudo systemctl status blackjack

4. 查看日志:
   sudo journalctl -u blackjack -f

防火墙设置:
-----------
如果需要外部访问，请开放端口:
sudo ufw allow 8080/tcp

或使用firewalld:
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --reload

注意事项:
---------
- 默认端口为8080
- 静态文件目录应相对于可执行文件位置
- 生产环境建议配置反向代理（Nginx）
- 建议使用HTTPS

技术支持:
---------
Go版本要求: 1.21+
依赖: 无（静态编译）
