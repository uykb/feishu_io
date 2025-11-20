# 币安加密货币市场监控程序 (Copycat Bot)

[![Docker Image](https://img.shields.io/badge/docker-ghcr.io%2Fuykb%2FCopycat--bot-blue)](https://ghcr.io/uykb/copycat-bot)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.21+-00ADD8)](https://go.dev/)

一个基于Go语言的高性能加密货币市场监控系统，实时监控币安U本位永续合约市场，捕捉由持仓量(OI)和价格共同驱动的交易信号。

## 功能特点

- 🚀 **高并发架构**: 使用Goroutines和Channels实现高效并发处理
- 📊 **实时数据**: WebSocket订阅所有交易对的15分钟K线数据
- 🔄 **OI监控**: 并发REST API轮询获取持仓量数据
- 🎯 **智能信号**: 四种市场信号自动检测
- 📱 **即时通知**: 飞书机器人Webhook实时推送交易信号
- 🐳 **Docker部署**: 容器化部署，支持云平台一键拉取

## 信号类型

程序监控以下四种信号组合：

1. **🔴 看涨突破 (Bullish Breakout)**: OI↑ + Price↑
   - OI变化率 > 5% 且 价格变化率 > 2%

2. **🟢 看跌动量 (Bearish Momentum)**: OI↑ + Price↓
   - OI变化率 > 5% 且 价格变化率 < -2%

3. **🟡 可能假突破 (Possible Fakeout)**: OI↓ + Price↑
   - OI变化率 < -5% 且 价格变化率 > 2%

4. **🔵 市场收缩 (Market Contraction)**: OI↓ + Price↓
   - OI变化率 < -5% 且 价格变化率 < -2%

## 技术架构

```
┌─────────────────┐
│  Binance API    │
└────────┬────────┘
         │
    ┌────┴────┐
    │         │
┌───▼──┐  ┌──▼───┐
│ WS   │  │ REST │
│Kline │  │ OI   │
└───┬──┘  └──┬───┘
    │        │
    │  ┌─────▼─────┐
    │  │ Goroutines│
    │  │ Pool (20) │
    │  └─────┬─────┘
    │        │
┌───▼────────▼────┐
│ Signal Detector │
└────────┬────────┘
         │
    ┌────▼────┐
    │ Telegram│
    │   Bot   │
    └─────────┘
┌─────────────────┐
│  Binance API    │
└────────┬────────┘
         │
    ┌────┴────┐
    │         │
┌───▼──┐  ┌──▼───┐
│ WS   │  │ REST │
│Kline │  │ OI   │
└───┬──┘  └──┬───┘
    │        │
    │  ┌─────▼─────┐
    │  │ Goroutines│
    │  │ Pool (20) │
    │  └─────┬─────┘
    │        │
┌───▼────────▼────┐
│ Signal Detector │
└────────┬────────┘
         │
    ┌────▼────┐
    │  Lark   │
    │ Webhook │
    └─────────┘
```

### 核心模块

- **binance/websocket.go**: WebSocket连接管理，订阅K线数据
- **binance/rest.go**: REST API并发轮询持仓量数据
- **strategy/detector.go**: 信号检测和策略计算
- **lark/bot.go**: 飞书机器人消息格式化和发送
- **config/config.go**: 配置管理
- **models/types.go**: 数据模型定义

## 部署方式

### 🚀 方式一：从 GitHub Container Registry 拉取（推荐）

这是最简单快捷的部署方式，无需克隆代码，直接拉取预构建镜像。

#### 快速启动

```bash
# 拉取最新镜像
docker pull ghcr.io/uykb/copycat-bot:main

# 运行容器
docker run -d \
  --name copycat-bot \
  --restart unless-stopped \
  -e LARK_WEBHOOK_URL="https://open.feishu.cn/open-apis/bot/v2/hook/你的webhook" \
  -e OI_THRESHOLD=5.0 \
  -e PRICE_THRESHOLD=2.0 \
  -e CHECK_INTERVAL=60 \
  ghcr.io/uykb/copycat-bot:main

# 查看日志
docker logs -f copycat-bot
```

#### 使用 docker-compose

创建 `docker-compose.yml`：

```yaml
version: '3.8'

services:
  copycat-bot:
    image: ghcr.io/uykb/copycat-bot:main
    container_name: copycat-bot
    restart: unless-stopped
    environment:
      - LARK_WEBHOOK_URL=${LARK_WEBHOOK_URL}
      - OI_THRESHOLD=${OI_THRESHOLD:-5.0}
      - PRICE_THRESHOLD=${PRICE_THRESHOLD:-2.0}
      - CHECK_INTERVAL=${CHECK_INTERVAL:-60}
      - TZ=Asia/Shanghai
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

创建 `.env` 文件：

```env
LARK_WEBHOOK_URL=https://open.feishu.cn/open-apis/bot/v2/hook/你的webhook
OI_THRESHOLD=5.0
PRICE_THRESHOLD=2.0
CHECK_INTERVAL=60
```

启动服务：

```bash
docker-compose up -d
```

#### 可用的镜像标签

| 标签 | 说明 | 使用场景 |
|------|------|----------|
| `main` | 主分支最新版本 | 生产环境（推荐） |
| `latest` | 同 main | 生产环境 |
| `v1.0.0` | 特定版本号 | 稳定版本 |
| `sha-xxxxxx` | 特定提交 | 特定版本锁定 |

### 方式二：Docker Compose 本地构建

从源码构建并部署。

#### 1. 获取飞书机器人 Webhook URL

#### 2. 获取飞书机器人 Webhook URL
2. 选择「群机器人」→「添加机器人」→「自定义机器人」
2. 选择「群机器人」→「添加机器人」→「自定义机器人」
4. 复制生成的 Webhook URL（格式如：`https://open.feishu.cn/open-apis/bot/v2/hook/xxxxx`）

#### 2. 配置环境变量

4. 复制生成的 Webhook URL（格式如：`https://open.feishu.cn/open-apis/bot/v2/hook/xxxxx`）

#### 3. 配置环境变量
```bash
# 复制环境变量模板

cp .env.docker.example .env.docker

# 编辑配置文件
nano .env.docker
```

填写以下内容：

```env
# 飞书机器人 Webhook URL（必填）
LARK_WEBHOOK_URL=https://open.feishu.cn/open-apis/bot/v2/hook/你的webhook_token

# 以下参数可选，使用默认值即可
OI_THRESHOLD=5.0
PRICE_THRESHOLD=2.0
CHECK_INTERVAL=60
```

#### 4. 启动服务

```bash
# 构建并启动
docker-compose --env-file .env.docker up -d

# 查看日志
docker-compose logs -f

# 停止服务
docker-compose down
```

**或使用 Makefile（更简单）：**

```bash
# 完整部署
make deploy

# 查看日志
make logs

# 重启服务
make restart

# 停止服务
make stop
```

**或使用自动化脚本：**

```bash
chmod +x deploy.sh
./deploy.sh
```

#### 4. 验证部署

```bash
# 查看容器状态
docker-compose ps

# 查看实时日志
docker-compose logs -f binance-monitor
```

### 方式二：云平台 Docker 部署

#### Docker Hub / 阿里云镜像仓库

**1. 构建并推送镜像**

```bash
# 登录 Docker Hub
docker login

# 构建镜像
docker build -t yourname/binance-monitor:latest .

# 推送到 Docker Hub
docker push yourname/binance-monitor:latest
```

**2. 在云平台拉取部署**

```bash
# 拉取镜像
docker pull yourname/binance-monitor:latest

# 运行容器（使用环境变量）
docker run -d \
  --name binance-monitor \
  --restart unless-stopped \
  -e LARK_WEBHOOK_URL="https://open.feishu.cn/open-apis/bot/v2/hook/xxxxx" \
  -e OI_THRESHOLD=5.0 \
  -e PRICE_THRESHOLD=2.0 \
  -e CHECK_INTERVAL=60 \
  yourname/binance-monitor:latest
```

#### 腾讯云 / 阿里云 / AWS 等云平台

**使用 docker-compose.yml 部署：**

```bash
# 1. 上传项目文件到服务器
scp -r . user@your-server:/opt/binance-monitor

# 2. SSH 登录服务器
ssh user@your-server

# 3. 进入项目目录
cd /opt/binance-monitor

# 4. 配置环境变量
cp .env.docker.example .env.docker
nano .env.docker  # 填写 LARK_WEBHOOK_URL

# 5. 启动服务
docker-compose --env-file .env.docker up -d
```

### 方式三：本地开发运行

如果你需要本地开发或调试：

#### 1. 环境要求

- Go 1.21 或更高版本
- 互联网连接

#### 2. 安装依赖

```bash
go mod download
```

#### 3. 配置环境变量

```bash
cp .env.example .env
nano .env
```

#### 4. 运行程序

```bash
go run main.go
```

或编译后运行：

```bash
go build -o binance-monitor
./binance-monitor  # Linux/Mac
binance-monitor.exe  # Windows
```

## 配置说明

### 环境变量

| 变量名 | 说明 | 默认值 | 是否必填 |
|--------|------|--------|----------|
| `LARK_WEBHOOK_URL` | 飞书机器人 Webhook URL | - | ✅ 必填 |
| `OI_THRESHOLD` | OI变化率阈值（%） | 5.0 | 可选 |
| `PRICE_THRESHOLD` | 价格变化率阈值（%） | 2.0 | 可选 |
| `CHECK_INTERVAL` | OI检查间隔（秒） | 60 | 可选 |

### Docker 资源限制

在 `docker-compose.yml` 中已配置资源限制：

- CPU 限制：最大 1 核心
- 内存限制：最大 512MB
- 日志大小：单文件 10MB，最多保留 3 个文件

## 消息示例

当触发信号时，你会在飞书群组收到如下格式的消息：

```
🔴 合约OI入场信号触发 - BTCUSDT 🔴 看涨突破 (Bullish Breakout)

⏰ 时间: 2025-09-10T02:15:05Z (周期: 15)

📊 当前数据
🔴 OI变化: 33.08442674604453% (阈值: >5%)
🟢 价格变化: 2.354931034556343% (阈值: >2%)

💡 市场解读
当前组合: 33.08% OI + 2.35% Price

• OI↑ + Price↑ = 看涨突破 (Bullish Breakout)
• OI↑ + Price↓ = 看跌动量 (Bearish Momentum)
• OI↓ + Price↑ = 可能假突破 (Possible Fakeout)
• OI↓ + Price↓ = 市场收缩 (Market Contraction)

📊 信号解读
OI和价格同时大幅上升，表明强劲的看涨突破信号。

⚠️ 交易建议
• 结合支撑位确认入场点
• 止损设在关键支撑下方
• 目标看向近期阻力位

📌 信号强度: 33.08% OI + 2.35% Price
```

## 项目结构

```
binance-monitor/
├── main.go                 # 主程序入口
├── go.mod                  # Go模块定义
├── Dockerfile              # Docker构建文件
├── docker-compose.yml      # Docker Compose配置
├── .dockerignore           # Docker忽略文件
├── .env.example            # 本地开发配置示例
├── .env.docker             # Docker环境变量（需自行创建）
├── deploy.sh               # 自动化部署脚本
├── Makefile                # Make命令集合
├── README.md               # 项目文档
├── config/
│   └── config.go          # 配置加载
├── models/
│   └── types.go           # 数据模型
├── binance/
│   ├── websocket.go       # WebSocket客户端
│   └── rest.go            # REST API客户端
├── strategy/
│   └── detector.go        # 信号检测器
└── lark/
    └── bot.go             # 飞书机器人
```

## 性能特点

- **并发处理**: 使用20个并发协程轮询OI数据
- **通道缓冲**: 合理设置通道缓冲区避免阻塞
- **自动重连**: WebSocket断线自动重连机制
- **优雅关闭**: 支持Ctrl+C优雅退出
- **容器化**: Docker多阶段构建，镜像体积小（< 20MB）
- **资源限制**: 默认限制CPU和内存使用，防止资源耗尽

## 常用命令

### Docker Compose 命令

```bash
# 启动服务
docker-compose --env-file .env.docker up -d

# 查看日志
docker-compose logs -f

# 查看容器状态
docker-compose ps

# 重启服务
docker-compose restart

# 停止服务
docker-compose down

# 重新构建并启动
docker-compose build --no-cache && docker-compose up -d
```

### Makefile 命令

```bash
make help      # 显示所有可用命令
make deploy    # 完整部署（构建+启动）
make build     # 构建镜像
make run       # 启动服务
make logs      # 查看日志
make restart   # 重启服务
make stop      # 停止服务
make status    # 查看状态
make clean     # 清理所有容器和镜像
```

## 故障排查

### 1. 容器无法启动

```bash
# 查看详细日志
docker-compose logs binance-monitor

# 检查环境变量是否正确
docker-compose config
```

### 2. 飞书消息发送失败

- 确认 Webhook URL 格式正确
- 测试 Webhook 是否可访问：
  ```bash
  curl -X POST "你的Webhook URL" \
    -H "Content-Type: application/json" \
    -d '{"msg_type":"text","content":{"text":"测试消息"}}'
  ```

### 3. WebSocket连接失败

- 检查服务器网络是否能访问币安API
- 查看容器日志中的错误信息

### 4. 查看容器内部

```bash
# 进入容器
docker exec -it binance-monitor sh

# 检查进程
ps aux
```

## 生产环境建议

1. **使用环境变量**: 不要在代码中硬编码敏感信息
2. **监控日志**: 使用 `docker-compose logs -f` 或日志收集工具
3. **自动重启**: `restart: unless-stopped` 确保容器崩溃后自动重启
4. **资源限制**: 根据实际情况调整 `docker-compose.yml` 中的资源限制
5. **备份配置**: 定期备份 `.env.docker` 文件
6. **HTTPS代理**: 如果需要，可以配置反向代理

## 注意事项

1. **敏感信息安全**: 
   - 不要将 `.env.docker` 文件提交到 Git
   - 使用云平台的密钥管理服务存储 Webhook URL
   
2. **网络访问**: 确保服务器能访问币安API和飞书API

3. **飞书速率限制**: 程序已内置1秒延迟，避免触发速率限制

4. **OI数据频率**: 建议60秒以上，避免过于频繁的API请求

5. **交易风险**: 本程序仅供信息参考，不构成投资建议

## 开发调试

### 本地调试

```bash
# 启用详细日志
go run main.go 2>&1 | tee monitor.log
```

### Docker 调试

```bash
# 构建开发镜像
docker build -t binance-monitor:dev .

# 运行并查看日志
docker run --rm \
  -e LARK_WEBHOOK_URL="your_webhook" \
  binance-monitor:dev
```

## License

MIT License

## 贡献

欢迎提交Issue和Pull Request！

## 免责声明

本软件仅用于教育和研究目的。加密货币交易存在高风险，使用本软件产生的任何交易决策和损失由用户自行承担。作者不对使用本软件造成的任何直接或间接损失负责。
