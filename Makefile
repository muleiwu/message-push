.PHONY: demo-reset demo-run dev build test fmt docker-build docker-up docker-down

# 本地 SQLite 演示（仅供本机文档与界面体验）
demo-reset:
	go run ./cmd/demo reset --db .local/message-push-demo.sqlite --redis-db 15

demo-run:
	./scripts/ensure-demo-assets.sh
	APP_INSTALLED=true DATABASE_DRIVER=sqlite DATABASE_HOST=.local/message-push-demo.sqlite REDIS_DB=15 HTTP_LOAD_STATIC=true HTTP_STATIC_MODE=embed go run main.go start

# 开发运行
dev:
	go run main.go start

# 构建
build:
	go build -o bin/push-service main.go

# 带优化的构建
build-prod:
	CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o bin/push-service main.go

# 测试
test:
	go test ./... -v -cover

# 测试覆盖率
test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# 代码格式化
fmt:
	gofmt -w .
	go fmt ./...

# 代码检查
lint:
	golangci-lint run

# 依赖管理
deps:
	go mod tidy
	go mod download

# Docker
docker-build:
	docker build -t push-service:latest .

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f

# 清理
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# 帮助
help:
	@echo "可用命令:"
	@echo "  make demo-reset      - 原子重建 SQLite 演示库并填充假数据"
	@echo "  make demo-run        - 使用 SQLite 与 Redis DB 15 启动演示服务"
	@echo "  make dev             - 开发模式运行"
	@echo "  make build           - 构建二进制文件"
	@echo "  make test            - 运行测试"
	@echo "  make fmt             - 格式化代码"
	@echo "  make docker-build    - 构建Docker镜像"
	@echo "  make docker-up       - 启动Docker容器"
	@echo "  make clean           - 清理生成的文件"
