# Docker 部署指南

本文档详细说明如何使用 Docker 部署币安加密货币市场监控程序。

## 快速开始

### 前置要求

- Docker 20.10+
- Docker Compose 1.29+
- 飞书机器人 Webhook URL

### 三步部署

```bash
# 1. 配置环境变量
cp .env.docker.example .env.docker
nano .env.docker  # 填写 LARK_WEBHOOK_URL

# 2. 启动服务
make deploy

# 3. 查看日志
make logs
```

## 详细步骤

### 1. 获取飞书 Webhook URL

1. 打开飞书群组
2. 点击右上角「···」→「设置」
3. 选择「群机器人」→「添加机器人」
4. 选择「自定义机器人」
5. 设置名称、描述（可选）
6. **复制生成的 Webhook URL**

格式示例：
```
https://open.feishu.cn/open-apis/bot/v2/hook/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

### 2. 配置环境变量

创建 `.env.docker` 文件：

```bash
cp .env.docker.example .env.docker
```

编辑内容：

```env
# 必填：飞书机器人 Webhook URL
LARK_WEBHOOK_URL=https://open.feishu.cn/open-apis/bot/v2/hook/你的token

# 可选：监控参数（使用默认值即可）
OI_THRESHOLD=5.0          # OI变化率阈值（%）
PRICE_THRESHOLD=2.0       # 价格变化率阈值（%）
CHECK_INTERVAL=60         # OI检查间隔（秒）
```

### 3. 部署服务

#### 方式 A：使用 Makefile（推荐）

```bash
# 查看所有可用命令
make help

# 完整部署（构建+启动）
make deploy

# 查看实时日志
make logs

# 查看容器状态
make status

# 重启服务
make restart

# 停止服务
make stop
```

#### 方式 B：使用 Docker Compose

```bash
# 构建镜像
docker-compose build

# 启动服务
docker-compose --env-file .env.docker up -d

# 查看日志
docker-compose logs -f

# 停止服务
docker-compose down
```

#### 方式 C：使用自动化脚本

```bash
chmod +x deploy.sh
./deploy.sh
```

### 4. 验证部署

```bash
# 检查容器状态
docker-compose ps

# 应该看到类似输出：
# NAME                  STATUS              PORTS
# binance-monitor       Up 10 seconds

# 查看启动日志
docker-compose logs binance-monitor

# 应该看到类似输出：
# === 币安加密货币市场监控程序启动 ===
# 配置加载成功: OI阈值=5.0%, 价格阈值=2.0%, 检查间隔=60s
# 正在获取USDT永续合约交易对列表...
# 获取到 XXX 个USDT永续合约交易对
# WebSocket已连接，订阅 XXX 个交易对的15分钟K线
# ✓ K线数据处理协程已启动
# ✓ OI数据处理协程已启动
# ✓ OI数据获取协程已启动
# ✓ 飞书消息发送协程已启动
# === 所有模块启动成功，开始监控... ===
```

## 云平台部署

### 腾讯云 / 阿里云 / 华为云

```bash
# 1. SSH 登录服务器
ssh user@your-server-ip

# 2. 安装 Docker 和 Docker Compose（如未安装）
curl -fsSL https://get.docker.com | sh
sudo systemctl start docker
sudo systemctl enable docker

# 3. 上传项目文件
# 在本地执行：
scp -r . user@your-server-ip:/opt/binance-monitor

# 4. 在服务器上部署
cd /opt/binance-monitor
cp .env.docker.example .env.docker
nano .env.docker  # 填写配置
make deploy
```

### AWS EC2

```bash
# Amazon Linux 2
sudo yum update -y
sudo amazon-linux-extras install docker -y
sudo service docker start
sudo usermod -a -G docker ec2-user

# 后续步骤同上
```

### Google Cloud Platform

```bash
# 使用 Container-Optimized OS
gcloud compute instances create binance-monitor \
  --image-family cos-stable \
  --image-project cos-cloud \
  --zone us-central1-a

# SSH 登录后部署
```

## Docker Hub 部署

### 构建并推送镜像

```bash
# 1. 登录 Docker Hub
docker login

# 2. 构建镜像
docker build -t yourname/binance-monitor:latest .

# 3. 推送镜像
docker push yourname/binance-monitor:latest
```

### 从 Docker Hub 拉取部署

```bash
# 拉取镜像
docker pull yourname/binance-monitor:latest

# 运行容器
docker run -d \
  --name binance-monitor \
  --restart unless-stopped \
  -e LARK_WEBHOOK_URL="https://open.feishu.cn/open-apis/bot/v2/hook/xxxxx" \
  -e OI_THRESHOLD=5.0 \
  -e PRICE_THRESHOLD=2.0 \
  -e CHECK_INTERVAL=60 \
  yourname/binance-monitor:latest

# 查看日志
docker logs -f binance-monitor
```

## 常见问题

### 1. 容器启动失败

**检查日志：**
```bash
docker-compose logs binance-monitor
```

**常见原因：**
- `LARK_WEBHOOK_URL` 未配置或格式错误
- 网络无法访问币安API或飞书API
- 端口冲突（虽然本程序不需要暴露端口）

### 2. 飞书消息发送失败

**测试 Webhook：**
```bash
curl -X POST "你的Webhook_URL" \
  -H "Content-Type: application/json" \
  -d '{
    "msg_type": "text",
    "content": {
      "text": "测试消息"
    }
  }'
```

**检查返回：**
- `{"code":0}` 表示成功
- 其他错误码请查看飞书开放平台文档

### 3. WebSocket 连接失败

**检查网络：**
```bash
# 进入容器测试
docker exec -it binance-monitor sh

# 测试币安API连接
wget -O- https://fapi.binance.com/fapi/v1/ping
```

### 4. 容器占用资源过高

**调整资源限制：**

编辑 `docker-compose.yml`：

```yaml
deploy:
  resources:
    limits:
      cpus: '0.5'      # 降低CPU限制
      memory: 256M     # 降低内存限制
```

### 5. 查看容器内部状态

```bash
# 进入容器
docker exec -it binance-monitor sh

# 查看进程
ps aux

# 查看网络连接
netstat -an | grep ESTABLISHED
```

## 日志管理

### 查看日志

```bash
# 实时日志
docker-compose logs -f

# 最近100行
docker-compose logs --tail 100

# 特定时间范围
docker-compose logs --since 2024-01-01T00:00:00
```

### 日志配置

日志已在 `docker-compose.yml` 中配置：

```yaml
logging:
  driver: "json-file"
  options:
    max-size: "10m"    # 单个日志文件最大10MB
    max-file: "3"      # 最多保留3个日志文件
```

### 清理日志

```bash
# 清理所有容器日志
docker system prune -a --volumes
```

## 更新部署

### 更新代码

```bash
# 1. 拉取最新代码
git pull

# 2. 重新构建并启动
make deploy
```

### 更新镜像

```bash
# 从 Docker Hub 更新
docker pull yourname/binance-monitor:latest
docker-compose up -d
```

## 备份与恢复

### 备份配置

```bash
# 备份环境变量文件
cp .env.docker .env.docker.backup.$(date +%Y%m%d)
```

### 恢复配置

```bash
# 恢复环境变量
cp .env.docker.backup.20240101 .env.docker
docker-compose restart
```

## 性能优化

### 1. 调整并发数

编辑 `binance/rest.go`：

```go
concurrency: 20,  // 根据服务器性能调整
```

### 2. 调整通道缓冲

编辑 `main.go`：

```go
klineDataCh := make(chan models.KlineData, 1000)  // 增加缓冲
oiDataCh := make(chan models.OIData, 1000)
```

### 3. 使用轻量级基础镜像

已使用 `alpine:latest`，镜像大小约 15-20MB。

## 安全建议

1. **不要暴露敏感信息**
   - 不要将 `.env.docker` 提交到 Git
   - 使用 `.gitignore` 排除敏感文件

2. **使用非 root 用户运行**
   - Dockerfile 已配置 `appuser` 用户
   - 容器内进程以非特权用户运行

3. **限制资源使用**
   - 已配置 CPU 和内存限制
   - 防止容器占用过多资源

4. **定期更新镜像**
   ```bash
   docker pull alpine:latest
   docker-compose build --no-cache
   ```

## 监控与告警

### 监控容器状态

```bash
# 使用 docker stats
docker stats binance-monitor

# 输出示例：
# CONTAINER ID   CPU %   MEM USAGE / LIMIT   MEM %   NET I/O
# xxxxx          0.50%   128MiB / 512MiB     25%     1.2kB / 0B
```

### 自动重启策略

已配置 `restart: unless-stopped`，容器会在以下情况自动重启：
- 程序崩溃
- 服务器重启
- Docker 服务重启

### 健康检查

Dockerfile 已包含健康检查：

```dockerfile
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD pgrep binance-monitor || exit 1
```

## 卸载

### 完全卸载

```bash
# 停止并删除容器
docker-compose down

# 删除镜像
docker rmi binance-monitor

# 删除项目文件
cd ..
rm -rf binance-monitor
```

### 保留配置卸载

```bash
# 只停止容器
docker-compose down

# 备份配置
cp .env.docker ~/.env.docker.backup
```

## 技术支持

遇到问题？

1. 查看日志：`make logs`
2. 检查配置：`docker-compose config`
3. 查看容器状态：`docker-compose ps`
4. 提交 Issue：包含日志和配置信息（隐藏敏感信息）
