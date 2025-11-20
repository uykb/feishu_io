#!/bin/bash
# Git 推送脚本

set -e

echo "=== 初始化 Git 仓库并推送到 GitHub ==="

# 添加所有文件
echo "1. 添加所有文件..."
git add .

# 提交
echo "2. 提交更改..."
git commit -m "Initial commit: Binance Market Monitor with Lark Integration"

# 设置主分支
echo "3. 设置主分支为 main..."
git branch -M main

# 添加远程仓库
echo "4. 添加远程仓库..."
git remote add origin https://github.com/uykb/Copycat-bot.git || echo "远程仓库已存在"

# 推送到 GitHub
echo "5. 推送到 GitHub..."
git push -u origin main

echo ""
echo "=== 推送完成 ==="
echo "仓库地址: https://github.com/uykb/Copycat-bot"
echo "镜像将自动构建: ghcr.io/uykb/copycat-bot:main"
echo ""
echo "下一步："
echo "1. 前往 GitHub 仓库设置"
echo "2. Actions 已自动触发，等待镜像构建完成"
echo "3. 使用命令拉取镜像: docker pull ghcr.io/uykb/copycat-bot:main"
