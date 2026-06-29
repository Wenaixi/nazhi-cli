package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/spf13/cobra"
)

// ─── 环境变量约定 ───────────────────────────────────────────
// 所有 nazhi CLI 命令支持以下环境变量作为标志的默认值
// （命令行标志始终优先于环境变量）
//   NAZHI_USERNAME     — 学号（login、school）
//   NAZHI_PASSWORD     — 密码（login）
//   NAZHI_TOKEN        — X-Auth-Token（session/whoami/task/self-eval）
//   NAZHI_SSO_BASE     — SSO 根地址（login、school）
//   NAZHI_BASE_URL     — 业务 API 根地址（session/whoami/task/self-eval）
//   NAZHI_TIMEOUT      — HTTP 超时（秒，所有命令）
//   NAZHI_UPLOAD_URL   — 文件上传 API 根地址（file upload）
// 推荐在 CI/集成测试中通过 `.env` 文件或 secret 注入。

// buildClientOpts 构造 client.Option 列表，是 buildClient 与 buildBizClient
// 共享的核心实现（组 E 提取）。
// 参数
//   - cmd: cobra 命令（含已注册的 flag）
//   - urlType: "sso" / "base" / "upload" — 决定读哪个 URL flag + env
//   - timeoutEnv: env key（如 "NAZHI_TIMEOUT"，file_upload 复用同一 key 但默认 30s）
//   - requireToken: true 时若 token 解析为空则返回 error
//
// 所有 env fallback 在这里统一处理。
func buildClientOpts(cmd *cobra.Command, urlType string, timeoutEnv string, requireToken bool) ([]client.Option, string, error) {
	// token 读取下沉到 urlType 分支，upload/sso 短路。
	// 原代码无条件读 --token flag + NAZHI_TOKEN env，即使 file upload 命令
	// 显式不提供 --token flag，NAZHI_TOKEN 仍会被注入到 pendingToken →
	// syncCookieToken 写 cookie jar 到 sso/api 域，违反 fileUploadCmd 文档
	// 契约「本命令不接受 --token 参数」。
	// 新语义（按 urlType 分流）
	//   - urlType=="base"   读 --token flag + NAZHI_TOKEN env（业务 API 命令）
	//   - urlType=="upload" 跳过 token 读取（file upload 命令契约无 token）
	//   - urlType=="sso"    跳过 token 读取（SSO 命令不需要预置 token
	//                       由 Login 流程获取并同步）
	// requireToken 参数对 upload/sso 仍兼容——它们不传 true，所以
	// 即使短路也不会因 requireToken 报错。
	var token string
	switch urlType {
	case "base":
		// 改走 applyURLFlag helper
		// 消除 6 处重复的 flagChanged+GetString+envString 模板。
		// 语义不变：flag 显式传递 → 用 flag 值；未传 → env fallback。
		token = applyURLFlag(cmd, "token", "NAZHI_TOKEN")
	}
	if requireToken && token == "" {
		return nil, "", fmt.Errorf("--token 为必填（也可通过 NAZHI_TOKEN 环境变量设置）")
	}

	// 合并两个平行 switch（原 urlVal 赋值 + 原 opts 追加），消除中间变量 urlVal。
	// 原代码先 switch urlType 提取 urlVal，处理 timeout/token/verbose 后
	// 再 switch 同 urlType 组装 opts，中间隔了 ~20 行。合并后一个 switch 完成
	// url 提取和 opts 追加，减少 1 个局部变量 + 消除冗余的 default 分支。
	// 注意：token/verbose 在 switch 后处理，所以 opts 初始值只需 timeout，url 和
	// logger 在 switch 中追加。
	timeoutSec, _ := cmd.Flags().GetInt("timeout")
	if !flagChanged(cmd, "timeout") {
		timeoutSec = envInt(timeoutEnv, timeoutSec)
	}
	// cleanup-timeout: timeout <= 0 时使用默认值 30 秒。
	// GetInt("timeout") 在 flag 未注册或值为 0 时返回 0，导致无超时（阻塞）。
	// fileUploadCmd 默认 timeout=30，但 buildClientOpts 通用路径不感知默认。
	const defaultTimeout = 30
	if timeoutSec <= 0 {
		timeoutSec = defaultTimeout
	}

	opts := []client.Option{client.WithTimeout(time.Duration(timeoutSec) * time.Second)}

	// token 按 urlType 分流
	if token != "" {
		opts = append(opts, client.WithToken(token))
	}

	// url 相关 option 合并 switch
	switch urlType {
	case "sso":
		if v := applyURLFlag(cmd, "sso-base", "NAZHI_SSO_BASE"); v != "" {
			opts = append(opts, client.WithSSOBase(v))
		}
	case "base":
		if v := applyURLFlag(cmd, "base-url", "NAZHI_BASE_URL"); v != "" {
			opts = append(opts, client.WithBaseURL(v))
		}
	case "upload":
		if v := applyURLFlag(cmd, "upload-url", "NAZHI_UPLOAD_URL"); v != "" {
			opts = append(opts, client.WithUploadURL(v))
		}
	default:
		return nil, "", fmt.Errorf("buildClientOpts: 未知 urlType %q（期望 sso/base/upload）", urlType)
	}

	// --verbose 时让 SDK logger 输出 Debug 级别日志
	// 否则 c.logDebug 被 slog LevelWarn 过滤，用户看不到 SDK 内部细节。
	if verbose && !quiet {
		opts = append(opts, client.WithLogger(
			slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})),
		))
	}
	return opts, token, nil
}
