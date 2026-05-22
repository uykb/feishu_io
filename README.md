# 币安加密货币市场监控程序 (Copycat Bot)

[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.21+-00ADD8)](https://go.dev/)

一个基于Go语言的高性能加密货币市场监控系统，支持 Binance 和 Hyperliquid 双数据源，实时监控市场数据，捕捉由持仓量(OI)、价格和资金费率共同驱动的交易信号。

## 功能特点

- 🚀 **高并发架构**: 使用Goroutines和Channels实现高效并发处理
- 📊 **实时数据**: WebSocket订阅K线数据（支持 Binance + Hyperliquid）
- 🔄 **OI监控**: 并发REST API轮询获取持仓量数据
- 💰 **资金费率**: 独立轮询获取资金费率，捕捉空头回补信号
- 🔀 **双源并行**: 同时接入 Binance 和 Hyperliquid，信号标注来源
- 🎯 **智能信号**: 四种通用市场信号 + HYPE专属五阶段策略
- 📱 **即时通知**: 飞书机器人Webhook实时推送交易信号
- 🐳 **Docker部署**: 容器化部署，支持云平台一键拉取

## 信号类型

### 通用信号（所有交易对）

| 信号 | 条件 | 含义 |
|:--|:--|:--|
| 🟢 看涨突破 | OI↑ + Price↑ | 新资金入场推动价格上涨 |
| 🔴 看跌动量 | OI↑ + Price↓ | 新资金做空推动价格下跌 |
| ⚪ 假突破 | OI↓ + Price↑ | 空头平仓导致的反弹，可能不可持续 |
| ⚪ 市场收缩 | OI↓ + Price↓ | 资金离场，市场收缩 |

### HYPE专属策略（HYPEUSDT）

基于真实市场复盘分析，HYPE策略通过**状态机**串联OI背离、资金费率反转和高低点形态，实现从筑底到拉升的全流程预警。

#### 五阶段信号

| 阶段 | 信号 | 图标 | 核心条件 |
|:--|:--|:--:|:--|
| 📉 加速下跌 | 下跌预警 | 📉 | OI持续降 + 费率正(多头拥挤) + ADX>15 |
| 🔍 底部吸筹 | 底部信号 | 🔍 | 价格新低 + **OI止跌** + 费率极负(<-0.0005%) |
| ✅ 底部确认 | Higher Low | ✅ | 价格回踩不破前低 + 费率收窄30%+ |
| 🚀 轧空拉升 | 轧空行情 | 🚀 | 价格涨0.5%+ + **OI下降**(空头平仓驱动) |
| 📈 趋势反转 | 反转确认 | 📈 | OI回升(多头建仓) + 费率转正 + ADX>20 |

#### 关键信号解读

**底部吸筹**是最核心的信号，识别逻辑：
```
价格创新低  ← 表面看是下跌
但OI不再降  ← 说明有人在底部接盘
费率极负    ← 空头极度拥挤，轧空一触即发
```

**轧空拉升**的确认逻辑：
```
价格上涨    ← 看起来是多头推动
但OI在下降  ← 实际是空头平仓买入
费率转正    ← 空头已撤退，多头开始主导
```

#### HYPE策略状态机

```
Normal ──→ Downtrend ──→ PotentialBottom ──→ BottomConfirmed ──→ Rallying
           (持续下跌)      (OI背离+费率极负)   (Higher Low确认)     (轧空/反转)
```

## 技术架构

```
┌─────────────────┐
│  Binance API    │
└────────┬────────┘
         │
    ┌────┴────────────────────┐
    │                         │
┌───▼──┐  ┌──▼───┐      ┌────▼─────┐
│ WS   │  │ REST │      │ REST     │
│Kline │  │ OI   │      │ Funding  │
└───┬──┘  └──┬───┘      └────┬─────┘
    │        │               │
    │  ┌─────▼─────┐         │
    │  │ Goroutines│         │
    │  │ Pool (20) │         │
    │  └─────┬─────┘         │
    │        │               │
┌───▼────────▼────┐  ┌───────▼───────┐
│ Signal Detector │  │ Hype Detector │
└────────┬────────┘  └───────┬───────┘
         │                   │
    ┌────▼────┐         ┌────▼────┐
    │  Lark   │         │  Lark   │
    │ Webhook │         │ Webhook │
    └─────────┘         └─────────┘
```

### 核心模块

| 模块 | 文件 | 功能 |
|:--|:--|:--|
| WebSocket | `binance/websocket.go` | 15分钟K线实时订阅 |
| OI轮询 | `binance/rest.go` | 并发获取持仓量数据 |
| 资金费率 | `binance/funding.go` | HYPE资金费率独立轮询 |
| 通用检测 | `strategy/detector.go` | OI+价格四象限信号 |
| HYPE检测 | `strategy/hype_detector.go` | 五阶段状态机策略 |
| 状态追踪 | `strategy/hype_state.go` | OI/费率历史存储 |
| 飞书通知 | `lark/bot.go` | 通用信号卡片 |
| HYPE卡片 | `lark/hype_card.go` | HYPE专属消息格式 |
| 配置管理 | `config/config.go` | 环境变量加载 |
| 数据模型 | `models/types.go` | 通用数据结构 |
| HYPE模型 | `models/hype_types.go` | HYPE专用数据结构 |

## 部署方式

### 🚀 方式一：Docker 镜像部署（推荐）

#### 快速启动

```bash
docker run -d \
  --name copycat-bot \
  --restart unless-stopped \
  -e LARK_WEBHOOK_URL="https://open.feishu.cn/open-apis/bot/v2/hook/你的webhook" \
  -e OI_THRESHOLD=5.0 \
  -e PRICE_THRESHOLD=2.0 \
  -e CHECK_INTERVAL=60 \
  copycat-bot:latest

docker logs -f copycat-bot
```

#### 使用 docker-compose

创建 `docker-compose.yml`：

```yaml
version: '3.8'

services:
  copycat-bot:
    image: copycat-bot:latest
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

### 方式二：Docker Compose 本地构建

#### 1. 获取飞书机器人 Webhook URL

1. 打开飞书群组设置
2. 选择「群机器人」→「添加机器人」→「自定义机器人」
3. 复制生成的 Webhook URL

#### 2. 配置环境变量

```bash
cp .env.docker.example .env.docker
nano .env.docker
```

填写以下内容：

```env
# 飞书机器人 Webhook URL（必填）
LARK_WEBHOOK_URL=https://open.feishu.cn/open-apis/bot/v2/hook/你的webhook_token

# 通用策略参数（可选）
OI_THRESHOLD=5.0
PRICE_THRESHOLD=2.0
CHECK_INTERVAL=60

# HYPE专属策略参数（可选，使用默认值即可）
HYPE_SYMBOL=HYPEUSDT
HYPE_OI_STOP_THRESHOLD=-0.15
HYPE_FR_EXTREME_THRESHOLD=-0.0005
HYPE_FR_RECOVERY_THRESHOLD=0.3
HYPE_HIGHER_LOW_PCT=0.3
HYPE_SQUEEZE_PRICE_PCT=0.5
HYPE_SQUEEZE_OI_DECLINE_PCT=0.05
HYPE_COOLDOWN_MINUTES=15
HYPE_LOOKBACK_KLINES=12
HYPE_FUNDING_INTERVAL=30
```

#### 3. 启动服务

```bash
docker-compose --env-file .env.docker up -d
docker-compose logs -f
```

**或使用 Makefile：**

```bash
make deploy    # 完整部署
make logs      # 查看日志
make restart   # 重启服务
make stop      # 停止服务
```

### 方式三：本地开发运行

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
./binance-monitor        # Linux/Mac
binance-monitor.exe      # Windows
```

## 配置说明

### 通用环境变量

| 变量名 | 说明 | 默认值 | 必填 |
|--------|------|--------|:----:|
| `LARK_WEBHOOK_URL` | 飞书机器人 Webhook URL | - | ✅ |
| `OI_THRESHOLD` | OI变化率阈值（%） | 5.0 | 否 |
| `PRICE_THRESHOLD` | 价格变化率阈值（%） | 2.0 | 否 |
| `CHECK_INTERVAL` | OI检查最大间隔（秒） | 60 | 否 |
| `MIN_CHECK_INTERVAL` | OI检查最小间隔（秒） | 10 | 否 |
| `ADX_THRESHOLD` | ADX最低阈值 | 20.0 | 否 |
| `SOCKS5_PROXY` | SOCKS5代理地址 | - | 否 |

### HYPE专属环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `HYPE_ONLY_MODE` | 是否仅监控HYPE（开启后跳过全量交易对） | false |
| `HYPE_SYMBOL` | HYPE交易对名称 | HYPEUSDT |
| `HYPE_OI_STOP_THRESHOLD` | OI止跌阈值（%），OI变化率大于此值认为止跌 | -0.15 |
| `HYPE_FR_EXTREME_THRESHOLD` | 资金费率极端阈值，低于此值认为空头极度拥挤 | -0.0005 |
| `HYPE_FR_RECOVERY_THRESHOLD` | 费率回升比例，从极值回升超过此比例触发底部确认 | 0.3 |
| `HYPE_HIGHER_LOW_PCT` | Higher Low确认比例（%），回踩不低于前低的此比例 | 0.3 |
| `HYPE_SQUEEZE_PRICE_PCT` | 轧空价格阈值（%），15分钟涨幅超过此值可能触发轧空 | 0.5 |
| `HYPE_SQUEEZE_OI_DECLINE_PCT` | 轧空OI下降阈值（%），OI下降超过此值确认空头平仓 | 0.05 |
| `HYPE_COOLDOWN_MINUTES` | 同类型信号冷却时间（分钟） | 15 |
| `HYPE_LOOKBACK_KLINES` | 回溯K线数量 | 12 |
| `HYPE_FUNDING_INTERVAL` | 资金费率轮询间隔（秒） | 30 |

## 运行模式

### 模式一：全量模式（Binance）

监控全部 524 个 USDT 永续合约交易对，同时叠加 HYPE 专属策略。

```env
HYPE_ONLY_MODE=false
HYPERLIQUID_ENABLED=false
```

**资源消耗：**
- WebSocket 订阅：524 个流
- OI 轮询：524 个交易对（~1680 req/min）
- 通用检测器：全部交易对
- HYPE 检测器：HYPEUSDT (Binance)
- 资金费率：HYPEUSDT

### 模式二：HYPE 专属模式（Binance）

仅监控 HYPEUSDT 一个交易对，大幅降低 API 压力。

```env
HYPE_ONLY_MODE=true
HYPERLIQUID_ENABLED=false
```

**资源消耗：**
- WebSocket 订阅：1 个流
- OI 轮询：1 个交易对（~6 req/min）
- 通用检测器：跳过
- HYPE 检测器：HYPEUSDT (Binance)
- 资金费率：HYPEUSDT

### 模式三：双源并行模式（Binance + Hyperliquid）

同时接入两个交易所数据源，HYPE 策略独立运行两套检测器，信号标注来源。

```env
HYPE_ONLY_MODE=false
HYPERLIQUID_ENABLED=true
```

**资源消耗：**
- Binance WebSocket：524 个流
- Binance OI 轮询：524 个交易对（~1680 req/min）
- Hyperliquid WebSocket：1 个流（HYPE）
- Hyperliquid REST：1 次/10s（获取全部合约 OI+费率，weight=20）
- HYPE 检测器：2 套（Binance + Hyperliquid 独立状态）
- 资金费率：2 路（Binance + Hyperliquid）

**双源优势：**
- 对比两个交易所的 OI/费率差异，发现跨所信号
- Hyperliquid 链上数据更透明，可作为 Binance 数据的验证
- 信号标注来源，方便追踪哪个交易所先出现信号

### 模式对比

| | 全量模式 | HYPE专属 | 双源并行 |
|:--|:--|:--|:--|
| Binance WS | 524流 | 1流 | 524流 |
| Binance OI | 524个 | 1个 | 524个 |
| Hyperliquid | ✗ | ✗ | ✓ (1个) |
| 通用检测器 | ✓ | ✗ | ✓ |
| HYPE检测器 | 1套(BN) | 1套(BN) | 2套(BN+HL) |
| API压力 | 高 | 极低 | 高+低 |

## 消息示例

### 通用信号

```
🟢 BTCUSDT Bullish Breakout Trading Signal

Time: 09-10 02:15:05    Count: t1
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Bullish Breakout OI↑ | Price↑
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Long: 65432.10    StopLoss: 64800.00
Quantity: 0.15    ATR: 520.30
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
OI Change: 33.08%    Price Change: 2.35%
ADX(14): 28.50
```

### HYPE专属信号

**Binance 数据源：**

```
🔍 HYPEUSDT (Binance) 底部吸筹

时间: 05-18 10:34:00    阶段: 底部吸筹
🔵 来源: Binance
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
价格创新低但OI止跌，资金费率极负，空头拥挤
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
价格: $41.37    最低价: $41.37
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
OI: 22330000    OI趋势: 止跌/微增 (-0.09%)
💰 资金费率: -0.00100%    状态: 极度拥挤
🔄 费率变化: 0.0%
📊 ADX(14): 22.50
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
💡 建议: 观望，等待Higher Low确认
```

**Hyperliquid 数据源：**

```
🔍 HYPEUSDT (Hyperliquid) 底部吸筹

时间: 05-18 10:34:00    阶段: 底部吸筹
🟣 来源: Hyperliquid
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
价格创新低但OI止跌，资金费率极负，空头拥挤
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
价格: $41.35    最低价: $41.35
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
OI: 1234567    OI趋势: 止跌/微增 (-0.08%)
💰 资金费率: -0.00012    状态: 极度拥挤
🔄 费率变化: 0.0%
📊 ADX(14): 21.80
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
💡 建议: 观望，等待Higher Low确认
```

```
🚀 HYPEUSDT 轧空拉升

时间: 05-18 11:40:00    阶段: 轧空拉升
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
价格拉升但OI下降，空头平仓驱动的轧空行情
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
价格: $42.32    价格变化: +1.40%
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
OI: 22180000    OI趋势: 下降(空头平仓) (-0.67%)
💰 资金费率: +0.00050%    状态: 费率回升
🔄 费率变化: 85.0%
📊 ADX(14): 25.30
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
💡 建议: 持有仓位，关注费率转正
```

## 项目结构

```
binance-monitor/
├── main.go                     # 主程序入口
├── go.mod                      # Go模块定义
├── Dockerfile                  # Docker构建文件
├── docker-compose.yml          # Docker Compose配置
├── .env.example                # 本地开发配置示例
├── .env.docker.example         # Docker环境变量示例
├── deploy.sh                   # 自动化部署脚本
├── Makefile                    # Make命令集合
├── README.md                   # 项目文档
├── config/
│   └── config.go              # 配置加载
├── models/
│   ├── types.go               # 通用数据模型
│   ├── hype_types.go          # HYPE专用数据模型
│   └── exchange.go            # 数据源枚举(Binance/Hyperliquid)
├── binance/
│   ├── websocket.go           # Binance WebSocket客户端
│   ├── rest.go                # Binance REST API客户端(OI)
│   ├── funding.go             # Binance REST API客户端(资金费率)
│   ├── hyperliquid_ws.go      # Hyperliquid WebSocket客户端
│   └── hyperliquid_rest.go    # Hyperliquid REST API客户端(OI+费率)
├── strategy/
│   ├── detector.go            # 通用信号检测器
│   ├── indicators.go          # 技术指标(ATR/ADX)
│   ├── hype_detector.go       # HYPE五阶段检测器(双源适配)
│   └── hype_state.go          # HYPE状态追踪
└── lark/
    ├── bot.go                 # 飞书机器人(通用)
    └── hype_card.go           # HYPE专属卡片格式
```

## 性能特点

- **并发处理**: 20个并发协程轮询OI数据 + 独立资金费率协程
- **通道缓冲**: K线(1000)、OI(1000)、信号(100/50)合理缓冲
- **自动重连**: WebSocket断线指数退避重连
- **优雅关闭**: 支持Ctrl+C优雅退出
- **容器化**: Docker多阶段构建，镜像体积小（< 20MB）

## 常用命令

### Docker Compose

```bash
docker-compose --env-file .env.docker up -d   # 启动
docker-compose logs -f                        # 日志
docker-compose ps                             # 状态
docker-compose restart                        # 重启
docker-compose down                           # 停止
```

### Makefile

```bash
make help      # 显示所有可用命令
make deploy    # 完整部署
make build     # 构建镜像
make run       # 启动服务
make logs      # 查看日志
make restart   # 重启
make stop      # 停止
make status    # 状态
make clean     # 清理
```

## 故障排查

### 1. 容器无法启动

```bash
docker-compose logs copycat-bot
docker-compose config
```

### 2. 飞书消息发送失败

- 确认 Webhook URL 格式正确
- 测试 Webhook 是否可访问：
  ```bash
  curl -X POST "你的Webhook URL" \
    -H "Content-Type: application/json" \
    -d '{"msg_type":"text","content":{"text":"测试"}}'
  ```

### 3. HYPE信号未触发

- 确认 `HYPE_SYMBOL` 配置正确（默认 HYPEUSDT）
- 检查资金费率是否正常获取：日志中应有 `开始轮询 HYPEUSDT 资金费率`
- 调整阈值参数：如果市场波动小，可适当放宽 `HYPE_OI_STOP_THRESHOLD` 等参数

### 4. WebSocket连接失败

- 检查服务器网络是否能访问币安API
- 如需要代理，配置 `SOCKS5_PROXY=socks5://127.0.0.1:1080`

## 生产环境建议

1. **敏感信息**: 使用云平台密钥管理服务存储 Webhook URL
2. **监控日志**: 使用日志收集工具（如 ELK、Loki）
3. **自动重启**: `restart: unless-stopped` 确保容器崩溃后自动重启
4. **资源限制**: 根据实际情况调整 CPU 和内存限制
5. **参数调优**: 根据实际市场表现调整 HYPE 策略阈值

## 注意事项

1. **敏感信息安全**: 不要将 `.env` 文件提交到 Git
2. **网络访问**: 确保服务器能访问币安API和飞书API
3. **飞书速率限制**: 程序已内置1秒延迟
4. **交易风险**: 本程序仅供信息参考，不构成投资建议
5. **HYPE策略**: 策略参数基于历史数据优化，实盘前请充分测试

## License

MIT License

## 贡献

欢迎提交Issue和Pull Request！

## 免责声明

本软件仅用于教育和研究目的。加密货币交易存在高风险，使用本软件产生的任何交易决策和损失由用户自行承担。作者不对使用本软件造成的任何直接或间接损失负责。
