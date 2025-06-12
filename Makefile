# Makefile for sync-image project

# 变量定义
BINARY_NAME=sync-image
MAIN_PATH=cmd/sync/main.go
BUILD_DIR=build
VERSION?=dev
COMMIT?=$(shell git rev-parse --short HEAD)
BUILD_TIME?=$(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Go 相关变量
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# 构建标志
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)"

# 默认目标
.PHONY: all
all: clean deps build

# 安装依赖
.PHONY: deps
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# 构建
.PHONY: build
build:
	mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)

# 构建多平台版本
.PHONY: build-all
build-all: clean deps
	mkdir -p $(BUILD_DIR)
	# Linux AMD64
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	# Linux ARM64
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PATH)
	# Windows AMD64
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	# macOS AMD64
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	# macOS ARM64
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)

# 运行测试
.PHONY: test
test:
	$(GOTEST) -v ./...

# 运行测试并生成覆盖率报告
.PHONY: test-coverage
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# 代码格式化
.PHONY: fmt
fmt:
	$(GOCMD) fmt ./...

# 代码检查
.PHONY: vet
vet:
	$(GOCMD) vet ./...

# 静态分析
.PHONY: lint
lint:
	golangci-lint run

# 清理
.PHONY: clean
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# 安装到系统
.PHONY: install
install: build
	cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/

# 卸载
.PHONY: uninstall
uninstall:
	rm -f /usr/local/bin/$(BINARY_NAME)

# 运行程序（开发模式）
.PHONY: run
run:
	$(GOCMD) run $(MAIN_PATH) --debug --log.level=debug

# 运行程序（生产模式）
.PHONY: run-prod
run-prod: build
	./$(BUILD_DIR)/$(BINARY_NAME)

# Docker 构建
.PHONY: docker-build
docker-build:
	docker build -t $(BINARY_NAME):$(VERSION) .

# Docker 构建 CI 版本
.PHONY: docker-build-ci
docker-build-ci:
	docker build -f Dockerfile.ci -t $(BINARY_NAME):$(VERSION)-ci .

# Docker 多架构构建
.PHONY: docker-buildx
docker-buildx:
	docker buildx build --platform linux/amd64,linux/arm64 -t $(BINARY_NAME):$(VERSION) .

# Docker 构建并推送到 GitHub Container Registry
.PHONY: docker-push-ghcr
docker-push-ghcr:
	docker buildx build --platform linux/amd64,linux/arm64 \
		-t ghcr.io/$(shell echo $(shell git config --get remote.origin.url) | sed 's/.*github.com[:/]\([^.]*\).*/\1/' | tr '[:upper:]' '[:lower:]')/$(BINARY_NAME):$(VERSION) \
		--push .

# 显示帮助
.PHONY: help
help:
	@echo "可用的 make 目标："
	@echo "  all          - 清理、安装依赖并构建"
	@echo "  deps         - 安装 Go 依赖"
	@echo "  build        - 构建二进制文件"
	@echo "  build-all    - 构建所有平台的二进制文件"
	@echo "  test         - 运行测试"
	@echo "  test-coverage- 运行测试并生成覆盖率报告"
	@echo "  fmt          - 格式化代码"
	@echo "  vet          - 运行 go vet"
	@echo "  lint         - 运行静态分析"
	@echo "  clean        - 清理构建文件"
	@echo "  install      - 安装到系统"
	@echo "  uninstall    - 从系统卸载"
	@echo "  run          - 运行程序（开发模式）"
	@echo "  run-prod     - 运行程序（生产模式）"
	@echo "  docker-build - 构建 Docker 镜像"
	@echo "  help         - 显示此帮助信息"
