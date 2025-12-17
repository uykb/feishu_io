# 快速开始指南

## 使用 Docker 镜像部署（最简单）

### 1. 获取飞书 Webhook

1. 打开飞书群组
2. 点击右上角「设置」→「群机器人」→「添加机器人」
3. 选择「自定义机器人」
4. 复制 Webhook URL

### 2. 一键部署

```bash
docker run -d \
  --name copycat-bot \
  --restart unless-stopped \
  -e LARK_WEBHOOK_URL="你的飞书Webhook" \
  copycat-bot:latest
```

### 3. 查看日志

```bash
docker logs -f copycat-bot
```

### 4. 管理容器

```bash
# 停止
docker stop copycat-bot

# 启动
docker start copycat-bot

# 重启
docker restart copycat-bot

# 删除
docker rm -f copycat-bot
```

## 使用 docker-compose 部署

### 1. 创建 docker-compose.yml

```yaml
version: '3.8'

services:
  copycat-bot:
    image: copycat-bot:latest
    container_name: copycat-bot
    restart: unless-stopped
    environment:
      - LARK_WEBHOOK_URL=你的飞书Webhook
      - OI_THRESHOLD=5.0
      - PRICE_THRESHOLD=2.0
      - CHECK_INTERVAL=60
      - TZ=Asia/Shanghai
```

### 2. 启动服务

```bash
docker-compose up -d
```

### 3. 查看日志

```bash
docker-compose logs -f
```

## 高级配置

### 环境变量说明

| 变量名 | 说明 | 默认值 | 必填 |
|--------|------|--------|------|
| `LARK_WEBHOOK_URL` | 飞书机器人 Webhook URL | - | ✅ |
| `OI_THRESHOLD` | OI变化率阈值（%） | 5.0 | ❌ |
| `PRICE_THRESHOLD` | 价格变化率阈值（%） | 2.0 | ❌ |
| `CHECK_INTERVAL` | OI检查间隔（秒） | 60 | ❌ |

### 自定义配置示例

```bash
docker run -d \
  --name copycat-bot \
  --restart unless-stopped \
  -e LARK_WEBHOOK_URL="你的Webhook" \
  -e OI_THRESHOLD=8.0 \
  -e PRICE_THRESHOLD=3.0 \
  -e CHECK_INTERVAL=120 \
  copycat-bot:latest
```

## 故障排查

### 检查容器状态

```bash
docker ps -a | grep copycat-bot
```

### 查看详细日志

```bash
docker logs copycat-bot
```

### 测试飞书 Webhook

```bash
curl -X POST "你的Webhook" \
  -H "Content-Type: application/json" \
  -d '{"msg_type":"text","content":{"text":"测试消息"}}'
```

### 重新部署

```bash
# 停止并删除旧容器
docker rm -f copycat-bot

# 拉取最新镜像
docker pull copycat-bot:latest

# 重新运行
docker run -d \
  --name copycat-bot \
  --restart unless-stopped \
  -e LARK_WEBHOOK_URL="你的Webhook" \
  copycat-bot:latest
```

## 更多信息

- 完整文档: [README.md](README.md)
- Docker部署详解: [DOCKER_DEPLOY.md](DOCKER_DEPLOY.md)
- GitHub仓库: https://github.com/uykb/Copycat-bot
