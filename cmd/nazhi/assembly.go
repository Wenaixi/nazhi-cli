package main

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/logx"
	"github.com/spf13/cobra"
)

// ProcessScope 显式拥有进程级待清理资源，替代原 lifecycle.go 的两套包级全局。
// 原先 pendingClients / pendingLogFiles 各自一把锁、散落在包全局，
// 既是隐式 Seam 又没有 Module 拥有它；现在由一个 Scope 集中管理，
// main 显式持有 defaultScope，测试可构造独立 Scope 隔离。
type ProcessScope struct {
	clientsMu sync.Mutex
	clients   []*client.Client
	filesMu   sync.Mutex
	files     []io.Closer
}

// NewProcessScope 创建独立的 Scope，便于测试隔离。
func NewProcessScope() *ProcessScope { return &ProcessScope{} }

func (s *ProcessScope) TrackClient(c *client.Client) {
	if c == nil || s == nil {
		return
	}
	s.clientsMu.Lock()
	s.clients = append(s.clients, c)
	s.clientsMu.Unlock()
}

func (s *ProcessScope) TrackLogFile(f io.Closer) {
	if f == nil || s == nil {
		return
	}
	s.filesMu.Lock()
	s.files = append(s.files, f)
	s.filesMu.Unlock()
}

func (s *ProcessScope) CloseLogFiles() error {
	if s == nil {
		return nil
	}
	s.filesMu.Lock()
	files := s.files
	s.files = nil
	s.filesMu.Unlock()
	return closeInLIFO(files, func(f io.Closer) error { return f.Close() })
}

// closeInLIFO 按后进先出关闭资源，同一资源只关闭一次。
//
// 去重的由来：ProcessScope 引入前，lifecycle.go 的 trackLogFile 同时写入
// Scope 与 legacy 两张包级表，同一 writer 因此在两处各存一份指针，
// 关闭时若不去重就会被 Close 两次。删除 legacy 双写层后该风险消失，
// 但「重复登记不应重复关闭」本身是有价值的不变量——它让去重成为
// Scope 自身的职责，而不是依赖调用方不去重复登记。
//
// 接口比较用的元素：*os.File、*bufio.Writer 等指针类型可直接比较；
// 若将来登记不可比较的接口实现，此处需改为按身份而非按值去重。
func closeInLIFO[T comparable](items []T, closeOne func(T) error) error {
	seen := make(map[T]struct{}, len(items))
	var firstErr error
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		if err := closeOne(item); err != nil {
			firstErr = errors.Join(firstErr, err)
		}
	}
	return firstErr
}

func (s *ProcessScope) CloseAllClients() error {
	if s == nil {
		return nil
	}
	s.clientsMu.Lock()
	clients := s.clients
	s.clients = nil
	s.clientsMu.Unlock()
	return closeInLIFO(clients, func(c *client.Client) error { return c.Close() })
}

// TrackedClientCount 返回当前已登记的 Client 数量，仅供测试观测。
// 生产代码不需要该信息，因此不作为业务能力暴露给调用方。
func (s *ProcessScope) TrackedClientCount() int {
	if s == nil {
		return 0
	}
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	return len(s.clients)
}

// TrackedLogFileCount 返回当前已登记的日志文件数量，仅供测试观测。
func (s *ProcessScope) TrackedLogFileCount() int {
	if s == nil {
		return 0
	}
	s.filesMu.Lock()
	defer s.filesMu.Unlock()
	return len(s.files)
}

// defaultScope 是进程默认 Scope，供包级 helper（lifecycle.go）使用。
// 新代码应显式传递 Scope。
var defaultScope = NewProcessScope()

// urlOptDef 描述一种 URL 类型对应的 flag/env/Option 元组。
type urlOptDef struct {
	flagName string
	envKey   string
	optFn    func(string) client.Option
}

// urlOptMap 将 urlType 映射到 flag/env/Option，消除 switch 重复。
var urlOptMap = map[string]urlOptDef{
	"sso":    {"sso-base", "NAZHI_SSO_BASE", client.WithSSOBase},
	"base":   {"base-url", "NAZHI_BASE_URL", client.WithBaseURL},
	"upload": {"upload-url", "NAZHI_UPLOAD_URL", client.WithUploadURL},
}

// maxPageSize 是 CLI 分页命令 --page-size 参数的上界（服务端单页上限）。
// 对齐 pkg/client 的 defaultSubmittedPageSize=500（实测服务端 pageSize 上限 500），
// 超限以参数错误拒绝而非透传让服务端静默截断。honor list / typical-case list /
// circle images 三处跨域引用共用此常量（定义在组装层公共设施区，避免
// 荣誉域命令文件承载全 CLI 分页知识）。
const maxPageSize = 500

// warnToStderr 配置类告警统一出口：--quiet 承诺「关闭所有 stderr 输出」，
// 此前三处 fmt.Fprintf(os.Stderr) 直写绕过了该承诺（timeout/log-level/log-format），
// CI 以 stderr 非空为异常信号会误判。
func warnToStderr(msg string) {
	if quiet {
		return
	}
	fmt.Fprint(os.Stderr, msg)
}

// resolveTimeoutSec 解析最终生效的 HTTP 超时秒数：flag 显式值优先，
// 未显式时回落 NAZHI_TIMEOUT 等环境变量，均非法（≤0）时回退注册默认 15 秒并告警。
// 三通道同因同果：flag 非法与 env 非法产出一致（此前 30 秒兜底与 15 秒默认分叉）。
func resolveTimeoutSec(cmd *cobra.Command, envKey string) int {
	const defaultTimeout = 15
	timeoutSec, _ := cmd.Flags().GetInt("timeout")
	if !flagChanged(cmd, "timeout") {
		timeoutSec = envInt(envKey, timeoutSec)
	}
	if timeoutSec <= 0 {
		warnToStderr(fmt.Sprintf("warn: timeout 值 %d 无效（flag 或环境变量），使用默认 %d 秒超时\n", timeoutSec, defaultTimeout))
		return defaultTimeout
	}
	return timeoutSec
}

// buildClientOpts 构造 client.Option 列表，是 buildClient 与 buildBizClient 共享核心。
func buildClientOpts(cmd *cobra.Command, urlType string, timeoutEnv string, requireToken bool) ([]client.Option, string, error) {
	var token string
	if urlType == "base" {
		token = applyURLFlag(cmd, "token", "NAZHI_TOKEN")
	}
	if requireToken && token == "" {
		return nil, "", fmt.Errorf("--token 为必填（也可通过 NAZHI_TOKEN 环境变量设置）")
	}

	timeoutSec := resolveTimeoutSec(cmd, timeoutEnv)
	opts := []client.Option{client.WithTimeout(time.Duration(timeoutSec) * time.Second)}

	if token != "" {
		opts = append(opts, client.WithToken(token))
	}

	if def, ok := urlOptMap[urlType]; ok {
		if v := applyURLFlag(cmd, def.flagName, def.envKey); v != "" {
			opts = append(opts, def.optFn(v))
		}
	} else {
		return nil, "", fmt.Errorf("buildClientOpts: 未知 urlType %q（期望 sso/base/upload）", urlType)
	}

	// 日志级别、格式与落盘路径：flag 大于 env 大于默认；--verbose 兼容为 debug
	levelStr := strings.TrimSpace(cliLogLevel)
	if levelStr == "" {
		levelStr = strings.TrimSpace(os.Getenv("NAZHI_LOG_LEVEL"))
	}
	if verbose && levelStr == "" {
		levelStr = "debug"
	}
	if levelStr == "" {
		levelStr = "warn"
	}
	lvl, err := logx.ParseLevel(levelStr)
	if err != nil {
		warnToStderr(fmt.Sprintf("warn: %v，使用 warn 级别\n", err))
		lvl = slog.LevelWarn
	}
	formatStr := strings.TrimSpace(cliLogFormat)
	if formatStr == "" {
		formatStr = strings.TrimSpace(os.Getenv("NAZHI_LOG_FORMAT"))
	}
	if formatStr == "" {
		formatStr = "text"
	}
	if _, err := logx.ParseFormat(formatStr); err != nil {
		warnToStderr(fmt.Sprintf("warn: %v，使用 text 格式\n", err))
		formatStr = "text"
	}
	filePath := strings.TrimSpace(cliLogFile)
	if filePath == "" {
		filePath = strings.TrimSpace(os.Getenv("NAZHI_LOG_FILE"))
	}
	var writers []io.Writer
	if !quiet {
		writers = append(writers, os.Stderr)
	}
	if filePath != "" {
		if fw, ferr := logx.NewFileWriter(filePath); ferr == nil {
			writers = append(writers, fw)
			trackLogFile(fw)
		} else {
			// 走 warnToStderr（--quiet 关闭所有 stderr 的契约：log-file 打开
			// 失败是配置类告警，与 timeout/log-level 同族）。
			warnToStderr(fmt.Sprintf("warn: 无法打开 log-file %q: %v\n", filePath, ferr))
		}
	}
	if len(writers) == 0 {
		writers = []io.Writer{io.Discard}
	}
	lg := logx.NewLogger(lvl, formatStr, writers...)
	opts = append(opts, client.WithLogger(lg))
	return opts, token, nil
}

func newClientWithOpts(opts ...client.Option) (*client.Client, error) {
	c, err := client.New(opts...)
	if err != nil {
		if c != nil {
			c.Close()
		}
		return nil, err
	}
	return c, nil
}

func registerBizFlags(cmd *cobra.Command) {
	cmd.Flags().String("token", "", "X-Auth-Token（必填，也可通过 NAZHI_TOKEN 环境变量设置）")
	cmd.Flags().String("base-url", "", "业务 API 根地址（默认 http://139.159.205.146:8280，也可通过 NAZHI_BASE_URL 环境变量设置）")
	cmd.Flags().Int("timeout", 15, "HTTP 超时（秒，也可通过 NAZHI_TIMEOUT 环境变量设置）")
}

func buildClient(cmd *cobra.Command, urlType string, timeoutEnv string) (*client.Client, error) {
	opts, _, err := buildClientOpts(cmd, urlType, timeoutEnv, false)
	if err != nil {
		return nil, err
	}
	c, err := newClientWithOpts(opts...)
	if err != nil {
		return nil, err
	}
	trackClient(c)
	return c, nil
}

func buildBizClient(cmd *cobra.Command) (*client.Client, string, error) {
	opts, token, err := buildClientOpts(cmd, "base", "NAZHI_TIMEOUT", true)
	if err != nil {
		return nil, "", err
	}
	c, err := newClientWithOpts(opts...)
	if err != nil {
		return nil, "", err
	}
	trackClient(c)
	return c, token, nil
}
