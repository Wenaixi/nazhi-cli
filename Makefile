.PHONY: build build-linux build-darwin build-windows test test-verbose test-integration lint vet fmt clean release install help

# ─── 版本 ───

VERSION := $(shell grep 'Version' internal/version/version.go | head -1 | sed 's/.*"\(.*\)"/\1/')
LDFLAGS := -ldflags="-s -w"

# ─── 构建 ───

build: clean-bin
	go build $(LDFLAGS) -o bin/nazhi.exe ./cmd/nazhi
	@echo "构建完成: bin/nazhi.exe"

build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/nazhi-linux-amd64 ./cmd/nazhi
	@echo "Linux amd64: bin/nazhi-linux-amd64"

build-darwin:
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/nazhi-darwin-arm64 ./cmd/nazhi
	@echo "macOS arm64: bin/nazhi-darwin-arm64"

build-windows:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/nazhi-windows-amd64.exe ./cmd/nazhi
	@echo "Windows amd64: bin/nazhi-windows-amd64.exe"

# ─── 测试 ───

test:
	go test -count=1 -race ./...
	@echo "测试全通过"

test-verbose:
	go test -count=1 -race -v ./...
	@echo "测试完成"

test-integration:
	@if [ -f .env ]; then echo "加载 .env"; export $$(grep -v '^#' .env | xargs); fi; \
	NAZHI_USERNAME="$${NAZHI_USERNAME:-}" NAZHI_PASSWORD="$${NAZHI_PASSWORD:-}" \
	go test -count=1 -tags=integration -race -v ./test/integration/...
	@echo "集成测试完成（未设置 NAZHI_USERNAME/NAZHI_PASSWORD 时自动跳过）"

# ─── 代码质量 ───

lint:
	golangci-lint run ./...
	@echo "lint 通过"

vet:
	go vet ./...
	@echo "vet 通过"

fmt:
	gofmt -l -s -w .
	@echo "gofmt 完成"

# ─── 安装 ───

install:
	go install $(LDFLAGS) ./cmd/nazhi
	@echo "已安装到 GOBIN: nazhi"

# ─── 发布 ───

release: test vet build-linux build-darwin build-windows
	@echo ""
	@echo "═══════════════════════════"
	@echo "  nazhi-cli v$(VERSION) 跨平台构建完成"
	@echo "═══════════════════════════"
	ls -lh bin/

# ─── 清理 ───

clean-bin:
	@rm -rf bin/

clean:
	rm -rf bin/
	@echo "已清理"

# ─── 帮助 ───

help:
	@echo "nazhi-cli v$(VERSION) — 构建命令"
	@echo "═══════════════════════════════════════"
	@echo "  make build        编译 CLI → bin/nazhi.exe"
	@echo "  make build-linux  交叉编译 Linux amd64"
	@echo "  make build-darwin 交叉编译 macOS arm64"
	@echo "  make build-windows 交叉编译 Windows amd64"
	@echo "  make test         全量测试（race 检测）"
	@echo "  make vet          go vet 静态分析"
	@echo "  make lint         golangci-lint 检查"
	@echo "  make fmt          gofmt 格式化"
	@echo "  make install      安装到 GOBIN"
	@echo "  make release      发布全平台构建"
	@echo "  make clean        清理构建产物"
	@echo "  make help         显示此帮助"
