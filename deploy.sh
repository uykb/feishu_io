#!/bin/bash
# 快速部署脚本

set -e

echo "=== 币安加密货币市场监控程序 - Docker 部署 ==="

# 检查 Docker 是否安装
if ! command -v docker &> /dev/null; then
    echo "错误: Docker 未安装，请先安装 Docker"
    exit 1
fi

# 检查 docker-compose 是否安装
if ! command -v docker-compose &> /dev/null; then
    echo "错误: docker-compose 未安装，请先安装 docker-compose"
    exit 1
fi

# 检查环境变量文件
if [ ! -f .env.docker ]; then
    echo "错误: .env.docker 文件不存在"
    echo "请复制 .env.docker.example 并填写配置"
    exit 1
fi

# 检查 LARK_WEBHOOK_URL 是否配置
source .env.docker
if [ -z "$LARK_WEBHOOK_URL" ] || [ "$LARK_WEBHOOK_URL" == "https://open.feishu.cn/open-apis/bot/v2/hook/your_webhook_token_here" ]; then
    echo "错误: 请在 .env.docker 中配置正确的 LARK_WEBHOOK_URL"
    exit 1
fi

echo "1. 停止并删除旧容器..."
docker-compose down

echo "2. 构建 Docker 镜像..."
docker-compose build --no-cache

echo "3. 启动容器..."
docker-compose --env-file .env.docker up -d

echo "4. 检查容器状态..."
sleep 3
docker-compose ps

echo ""
echo "=== 部署完成 ==="
echo "查看日志: docker-compose logs -f"
echo "停止服务: docker-compose down"
echo "重启服务: docker-compose restart"
