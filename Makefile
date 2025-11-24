.PHONY: migrate-up migrate-down migrate-fresh migrate-seed dev build test fmt docker-build docker-up docker-down

# 数据库迁移
migrate-up:
	go run cmd/migrate/main.go -action=up

migrate-down:
	go run cmd/migrate/main.go -action=down

migrate-fresh:
	go run cmd/migrate/main.go -action=fresh

migrate-seed:
	go run cmd/migrate/main.go -action=seed

# 开发运行
dev:
	go run main.go

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
	@echo "  make migrate-up      - 执行数据库迁移"
	@echo "  make migrate-down    - 回滚数据库迁移"
	@echo "  make migrate-fresh   - 清空并重新迁移数据库"
	@echo "  make migrate-seed    - 填充测试数据"
	@echo "  make dev             - 开发模式运行"
	@echo "  make build           - 构建二进制文件"
	@echo "  make test            - 运行测试"
	@echo "  make fmt             - 格式化代码"
	@echo "  make docker-build    - 构建Docker镜像"
	@echo "  make docker-up       - 启动Docker容器"
	@echo "  make clean           - 清理生成的文件"
