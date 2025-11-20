# Git 推送命令

请在项目根目录下按顺序执行以下命令：

## Windows PowerShell / CMD

```powershell
# 1. 进入项目目录
cd c:/Users/uykb/CodeBuddy/20251120111530

# 2. 添加所有文件到 Git
git add .

# 3. 提交更改
git commit -m "Initial commit: Binance Market Monitor with Lark Integration and Docker support"

# 4. 设置主分支
git branch -M main

# 5. 添加远程仓库（如果已添加会提示错误，可忽略）
git remote add origin https://github.com/uykb/Copycat-bot.git

# 6. 推送到 GitHub
git push -u origin main
```

## Linux / macOS / Git Bash

```bash
# 1. 进入项目目录
cd /c/Users/uykb/CodeBuddy/20251120111530

# 2. 添加所有文件到 Git
git add .

# 3. 提交更改
git commit -m "Initial commit: Binance Market Monitor with Lark Integration and Docker support"

# 4. 设置主分支
git branch -M main

# 5. 添加远程仓库
git remote add origin https://github.com/uykb/Copycat-bot.git

# 6. 推送到 GitHub
git push -u origin main
```

## 或使用脚本（Linux/macOS/Git Bash）

```bash
# 给脚本执行权限
chmod +x git-push.sh

# 运行脚本
./git-push.sh
```

---

## 推送后的操作

### 1. 验证推送成功

访问仓库地址确认代码已上传：
```
https://github.com/uykb/Copycat-bot
```

### 2. 检查 GitHub Actions

1. 访问 `https://github.com/uykb/Copycat-bot/actions`
2. 查看 "Build and Push Docker Image" 工作流
3. 等待构建完成（约 3-5 分钟）

### 3. 验证 Docker 镜像

构建完成后，镜像会发布到：
```
ghcr.io/uykb/copycat-bot:main
ghcr.io/uykb/copycat-bot:latest
```

### 4. 拉取并运行镜像

```bash
# 拉取镜像
docker pull ghcr.io/uykb/copycat-bot:main

# 运行容器
docker run -d \
  --name copycat-bot \
  --restart unless-stopped \
  -e LARK_WEBHOOK_URL="你的飞书Webhook" \
  ghcr.io/uykb/copycat-bot:main

# 查看日志
docker logs -f copycat-bot
```

---

## 常见问题

### Q1: 推送时提示权限错误

**解决方案：**
```bash
# 配置 Git 凭据
git config --global user.name "你的GitHub用户名"
git config --global user.email "你的GitHub邮箱"

# 使用 GitHub CLI 认证（推荐）
gh auth login

# 或使用 Personal Access Token
# 1. 访问 https://github.com/settings/tokens
# 2. 生成新 token，勾选 repo 权限
# 3. 推送时使用 token 作为密码
```

### Q2: 远程仓库已存在错误

```bash
# 删除现有远程仓库
git remote remove origin

# 重新添加
git remote add origin https://github.com/uykb/Copycat-bot.git
```

### Q3: 推送被拒绝（rejected）

```bash
# 强制推送（首次推送可用）
git push -u origin main --force
```

### Q4: GitHub Actions 构建失败

1. 检查 Dockerfile 语法
2. 查看 Actions 日志详细错误
3. 确保 go.mod 和 go.sum 存在且正确

---

## 镜像可见性设置

### 将镜像设为公开（推荐）

1. 访问 `https://github.com/uykb/Copycat-bot/pkgs/container/copycat-bot`
2. 点击 "Package settings"
3. 在 "Danger Zone" 中，将 Visibility 改为 "Public"

这样其他人也可以直接拉取镜像，无需认证。

---

## 自动化工作流说明

推送后，GitHub Actions 会自动：

1. ✅ 检出代码
2. ✅ 设置 Docker Buildx
3. ✅ 登录到 GHCR
4. ✅ 构建 Docker 镜像（支持 amd64 和 arm64）
5. ✅ 推送镜像到 ghcr.io
6. ✅ 生成多个标签：
   - `main` - 主分支最新版本
   - `latest` - 最新版本
   - `sha-xxxxxx` - 特定提交
   - `vX.X.X` - 版本标签（如果打 tag）

---

## 后续更新代码

当你需要更新代码时：

```bash
# 1. 修改代码后
git add .

# 2. 提交更改
git commit -m "你的更新说明"

# 3. 推送
git push

# GitHub Actions 会自动重新构建镜像
```

---

## 查看构建日志

```bash
# 使用 GitHub CLI
gh run list
gh run view <run-id>

# 或访问网页
https://github.com/uykb/Copycat-bot/actions
```
