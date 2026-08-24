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
	var firstErr error
	for i := len(files) - 1; i >= 0; i-- {
		if err := files[i].Close(); err != nil {
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
	var firstErr error
	for i := len(clients) - 1; i >= 0; i-- {
		if err := clients[i].Close(); err != nil {
			firstErr = errors.Join(firstErr, err)
		}
	}
	return firstErr
}

// defaultScope 是进程默认 Scope，供原有全局 helper 兼容。
// 新代码应显式传递 Scope；旧全局函数保留为薄转发，避免一次性大范围改动。
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

// buildClientOpts 构造 client.Option 列表，是 buildClient 与 buildBizClient 共享核心。
func buildClientOpts(cmd *cobra.Command, urlType string, timeoutEnv string, requireToken bool) ([]client.Option, string, error) {
	var token string
	if urlType == "base" {
		token = applyURLFlag(cmd, "token", "NAZHI_TOKEN")
	}
	if requireToken && token == "" {
		return nil, "", fmt.Errorf("--token 为必填（也可通过 NAZHI_TOKEN 环境变量设置）")
	}

	timeoutSec, _ := cmd.Flags().GetInt("timeout")
	if !flagChanged(cmd, "timeout") {
		timeoutSec = envInt(timeoutEnv, timeoutSec)
	}
	const defaultTimeout = 30
	if timeoutSec <= 0 {
		if flagChanged(cmd, "timeout") {
			fmt.Fprintf(os.Stderr, "warn: --timeout 0 无效，使用默认 %d 秒超时\n", defaultTimeout)
		}
		timeoutSec = defaultTimeout
	}

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
		fmt.Fprintf(os.Stderr, "warn: %v，使用 warn 级别\n", err)
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
		fmt.Fprintf(os.Stderr, "warn: %v，使用 text 格式\n", err)
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
			fmt.Fprintf(os.Stderr, "warn: 无法打开 log-file %q: %v\n", filePath, ferr)
		}
	}
	if len(writers) == 0 {
		writers = []io.Writer{io.Discard}
	}
	lg := logx.NewLogger(lvl, formatStr, writers...)
	opts = append(opts, client.WithLogger(lg))
	if ocr := omniOCRFromEnv(); ocr != nil {
		opts = append(opts, client.WithCustomOCR(ocr))
	}
	return opts, token, nil
}

// omniOCRFromEnv 读取硅基流动 Qwen3-Omni 配置。
func omniOCRFromEnv() *omniOCR {
	key := strings.TrimSpace(os.Getenv("NAZHI_SILICONFLOW_API_KEY"))
	if key == "" {
		key = strings.TrimSpace(os.Getenv("NAZHI_OCR_API_KEY"))
	}
	if key == "" {
		key = strings.TrimSpace(os.Getenv("SILICONFLOW_API_KEY"))
	}
	if key == "" {
		return nil
	}
	return newOmniOCR(key, os.Getenv("NAZHI_OCR_BASE_URL"), os.Getenv("NAZHI_OCR_MODEL"))
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
