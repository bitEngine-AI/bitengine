COMPOSE := docker compose -f deploy/docker-compose.yml

dev:          ## 启动（一条命令）
	$(COMPOSE) up --build

dev-bg:       ## 后台启动
	$(COMPOSE) up --build -d
	@echo "✅ http://localhost:9000/api/v1/system/status"

stop:         ## 停止
	$(COMPOSE) down

test:         ## 测试
	docker exec be-core go test -v -count=1 ./internal/...

logs:         ## 日志
	docker logs -f be-core

status:       ## 状态
	@docker ps --filter "name=be-" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
	@echo "" && curl -sf http://localhost:9000/api/v1/system/status 2>/dev/null || echo "⚠️ 未响应"

models:       ## 拉取模型
	docker exec be-ollama ollama pull qwen3:4b
	docker exec be-ollama ollama pull phi4-mini
	@echo "✅ Models ready"

shell:        ## 进入容器
	docker exec -it be-core sh

db:           ## 进入数据库
	docker exec -it be-postgres psql -U bitengine

clean:        ## 清理
	$(COMPOSE) down -v

help:         ## 帮助
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "\033[36m%-12s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
.PHONY: dev dev-bg stop test logs status models shell db clean help
