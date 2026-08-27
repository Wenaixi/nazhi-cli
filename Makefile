.PHONY: build build-linux build-darwin build-windows test test-verbose test-integration lint lint-fix vet fmt clean release install help ci-local test-coverage tidy-check

# ─── 版本 ───

VERSION := $(shell grep -E '^\s*var\s+Version\s*=' internal/version/version.go | sed 's/.*"\(.*\)"/\1/')
LDFLAGS := -ldflags="-s -w"

# ─── 构建 ───
# SDK 内置 nazhi-captcha-sdk 本地验证码识别器；纯 Go 构建，无 CGO 依赖，login 零配置。

build: clean-bin
	go build $(LDFLAGS) -o bin/nazhi.exe ./cmd/nazhi
	@echo "构建完成: bin/nazhi.exe（纯 Go；login 内置本地识别器零配置）"

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

lint-fix:
	"$$(go env GOPATH)/bin/golangci-lint" run --fix ./...
	@echo "lint 自动修复完成"

vet:
	go vet ./...
	@echo "vet 通过"

fmt:
	gofmt -l -s -w .
	@echo "gofmt 完成"

# ─── CI 本地复现 ───

# go.mod / go.sum 整洁校验（CI gate 1）
tidy-check:
	go mod tidy
	git diff --exit-code go.mod go.sum

# 单元测试 + 覆盖率汇总（只统计 pkg/，与 CI 单测范围一致）
test-coverage:
	go test -count=1 -race -coverprofile=coverage.out ./pkg/...
	go tool cover -func=coverage.out | tail -1

# 本地一键跑完 CI 的核心 gate（tidy → lint → vet → 单测 → 集成测试）
ci-local: tidy-check lint vet test test-integration
	@echo "ci-local 全绿"

# ─── 安装 ───

install:
	go install $(LDFLAGS) ./cmd/nazhi
	@echo "已安装到 GOBIN: nazhi"

# ─── 发布 ───

release: test vet build build-linux build-darwin build-windows
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
	@echo "  make build        编译 CLI（纯 Go，内置本地验证码识别） → bin/nazhi.exe"
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
# ─── E2E 究极大测试（读真写模拟，可随时开真） ───────────────────────
# 默认读真写模拟；无 NAZHI_USERNAME 时自动 Skip 真读，离线仍绿。
# NAZHI_E2E_LIVE_WRITE=1 时写走真域；文件上传默认真域（无鉴权安全）。
.PHONY: e2e e2e-mixed e2e-live

e2e: ## 一键 E2E（mixed：读真写模拟）
	go test -count=1 -race -v ./test/e2e/...
	@echo "e2e 完成（有 NAZHI_USERNAME 时含真读，无则仅 mock）"

e2e-mixed: e2e ## 别名：读真写模拟

# e2e-live 已移除：写操作永远 mock，不支持全真（防污染线上数据）
