package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/spf13/cobra"
)

// pendingClients 跟踪本次进程内构造的所有 Client，main 退出前统一 Close()。
// 解决 "Client 包装了 *ocr.Pool 但不暴露 Close() → 临时目录泄漏" 的问题
// （组 C 修复，merge 保留）。
var (
	pendingClientsMu sync.Mutex
	pendingClients   []*client.Client
)

// trackClient 把 Client 加入待清理列表。
// 由 buildClient / buildBizClient 内部调用，业务侧无需感知。
func trackClient(c *client.Client) {
	pendingClientsMu.Lock()
	pendingClients = append(pendingClients, c)
	pendingClientsMu.Unlock()
}

// closeAllClients 关闭所有待清理 Client，返回聚合错误。
// 在 main 函数退出前调用一次 (defer)，保证 ONNX session + 临时目录 + keep-alive 连接全部释放。
func closeAllClients() error {
	pendingClientsMu.Lock()
	clients := pendingClients
	pendingClients = nil
	pendingClientsMu.Unlock()

	// C9 修复：收集所有 Close 错误而非只保留第一个。
	var firstErr error
	for _, c := range clients {
		if err := c.Close(); err != nil {
			firstErr = errors.Join(firstErr, err)
		}
	}
	return firstErr
}

// newClientWithOpts 是 buildClient / buildBizClient 共享的 Client 构造辅助（C3 修复）。
//
// 消除两处重复的 `if err != nil { if c != nil { c.Close() }; return nil, err }` 模式，
// 统一处理 New() 失败时的资源清理。
//
// 注意：调用方仍需自行调用 trackClient() 注册到 pendingClients，因为
// buildBizClient 需要先返回 token 再由调用方决定是否 track。
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

// buildClient 从 cobra 命令标志构建通用 Client，处理 sso-base / base-url /
// upload-url / timeout 的 env fallback 与 opts 拼接。**不**做 token 必填校验——
// token 必填是业务 API 命令（whoami/task/self-eval/session activate）的
// 约束，SSO 命令（login/school）不需要（组 E 拆分）。
//
// login/school 等 SSO 命令直接调用。
// 业务命令应调 buildBizClient（基于 buildClientOpts + token 必填校验）。
//
// C1+C2 修复（group-H round-4）：新增 urlType 参数让调用方指定 URL 来源——
//   - urlType="sso": 从 cmd 读 --sso-base flag + NAZHI_SSO_BASE env（login/school）
//   - urlType="base": 从 cmd 读 --base-url flag + NAZHI_BASE_URL env（业务 API 命令）
//   - urlType="upload": 从 cmd 读 --upload-url flag + NAZHI_UPLOAD_URL env（file upload）
//
// urlKey 是 timeout env key（默认 NAZHI_TIMEOUT），不同命令可覆盖默认值。
// school 用默认 15s，file upload 用 30s——通过 urlKey 注入对应 env。
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

// buildBizClient 从 cobra 命令标志构建业务 API Client，自动处理 env fallback。
// 基于 buildClientOpts + token 必填校验（组 E 拆分）。
//
// 必填标志：token。
// 可选标志：base-url, timeout, sso-base。
//
// 返回 (client, token)。
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

// buildClientOpts 构造 client.Option 列表，是 buildClient 与 buildBizClient
// 共享的核心实现（组 E 提取）。
//
// 参数：
//   - cmd: cobra 命令（含已注册的 flag）
//   - urlType: "sso" / "base" / "upload" — 决定读哪个 URL flag + env
//   - timeoutEnv: env key（如 "NAZHI_TIMEOUT"，file_upload 复用同一 key 但默认 30s）
//   - requireToken: true 时若 token 解析为空则返回 error
//
// 所有 env fallback 在这里统一处理。
func buildClientOpts(cmd *cobra.Command, urlType string, timeoutEnv string, requireToken bool) ([]client.Option, string, error) {
	// F16 修复（round-7）：token 读取下沉到 urlType 分支，upload/sso 短路。
	//
	// 原代码无条件读 --token flag + NAZHI_TOKEN env，即使 file upload 命令
	// 显式不提供 --token flag，NAZHI_TOKEN 仍会被注入到 pendingToken →
	// syncCookieToken 写 cookie jar 到 sso/api 域，违反 fileUploadCmd 文档
	// 契约「本命令不接受 --token 参数」。
	//
	// 新语义（按 urlType 分流）：
	//   - urlType=="base"   读 --token flag + NAZHI_TOKEN env（业务 API 命令）
	//   - urlType=="upload" 跳过 token 读取（file upload 命令契约无 token）
	//   - urlType=="sso"    跳过 token 读取（SSO 命令不需要预置 token，
	//                       由 Login 流程获取并同步）
	//
	// requireToken 参数对 upload/sso 仍兼容——它们不传 true，所以
	// 即使短路也不会因 requireToken 报错。
	var token string
	switch urlType {
	case "base":
		// F7 修复（group-F round-8）：用 flagChanged() 守卫 token 读取，
		// 避免用户显式传 --token "" 时 env fallback 静默覆盖。
		//
		// 原代码无 Changed 检查：cmd.Flags().GetString("token") 显式空字符串
		// 与未传 flag 行为完全一致（都返回 ""），env fallback 无法区分这两种意图。
		// 哨兵默认 + env 覆盖是反模式——用户期望"显式空字符串"被尊重。
		//
		// 设计契约：
		//   - Changed("token")=true → 用户显式传过 flag，flag 值生效（含显式空字符串）
		//   - Changed("token")=false → 未传 flag，走 env fallback
		//
		// 与 round-4 C2 timeout 修复保持对称（cmd/nazhi/env.go:45 flagChanged）。
		if flagChanged(cmd, "token") {
			token, _ = cmd.Flags().GetString("token")
		} else {
			token = envString("NAZHI_TOKEN", "")
		}
	}
	if requireToken && token == "" {
		return nil, "", fmt.Errorf("--token 为必填（也可通过 NAZHI_TOKEN 环境变量设置）")
	}

	var urlVal string
	switch urlType {
	case "sso":
		// B12 修复：用 flagChanged() 守卫 sso-base 读取，
		// 避免用户显式传 --sso-base "" 时被 NAZHI_SSO_BASE 环境变量覆盖。
		if flagChanged(cmd, "sso-base") {
			urlVal, _ = cmd.Flags().GetString("sso-base")
		} else {
			urlVal = envString("NAZHI_SSO_BASE", "")
		}
	case "base":
		// B12 修复：用 flagChanged() 守卫 base-url 读取。
		if flagChanged(cmd, "base-url") {
			urlVal, _ = cmd.Flags().GetString("base-url")
		} else {
			urlVal = envString("NAZHI_BASE_URL", "")
		}
	case "upload":
		// B12 修复：用 flagChanged() 守卫 upload-url 读取。
		if flagChanged(cmd, "upload-url") {
			urlVal, _ = cmd.Flags().GetString("upload-url")
		} else {
			urlVal = envString("NAZHI_UPLOAD_URL", "")
		}
	default:
		return nil, "", fmt.Errorf("buildClientOpts: 未知 urlType %q（期望 sso/base/upload）", urlType)
	}

	timeoutSec, _ := cmd.Flags().GetInt("timeout")
	if !flagChanged(cmd, "timeout") {
		timeoutSec = envInt(timeoutEnv, timeoutSec)
	}

	opts := []client.Option{client.WithTimeout(time.Duration(timeoutSec) * time.Second)}
	if token != "" {
		opts = append(opts, client.WithToken(token))
	}
	// G2 修复：--verbose 时让 SDK logger 输出 Debug 级别日志，
	// 否则 c.logDebug 被 slog LevelWarn 过滤，用户看不到 SDK 内部细节。
	if verbose {
		opts = append(opts, client.WithLogger(
			slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})),
		))
	}
	switch urlType {
	case "sso":
		if urlVal != "" {
			opts = append(opts, client.WithSSOBase(urlVal))
		}
	case "base":
		if urlVal != "" {
			opts = append(opts, client.WithBaseURL(urlVal))
		}
	case "upload":
		if urlVal != "" {
			opts = append(opts, client.WithUploadURL(urlVal))
		}
	}
	return opts, token, nil
}
