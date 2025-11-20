.PHONY: help build run stop logs clean restart

help: ## 显示帮助信息
	@echo "币安加密货币市场监控程序 - Makefile 命令"
	@echo ""
	@echo "使用方法: make [命令]"
	@echo ""
	@echo "可用命令:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: ## 构建 Docker 镜像
	docker-compose build --no-cache

run: ## 启动服务
	docker-compose --env-file .env.docker up -d

stop: ## 停止服务
	docker-compose down

logs: ## 查看日志
	docker-compose logs -f

clean: ## 清理容器和镜像
	docker-compose down -v --rmi all

restart: ## 重启服务
	docker-compose restart

status: ## 查看容器状态
	docker-compose ps

deploy: ## 完整部署（构建+启动）
	@make build
	@make run
	@echo "部署完成！使用 'make logs' 查看日志"
