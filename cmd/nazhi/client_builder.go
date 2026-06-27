package main

import (
	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/spf13/cobra"
)

// newClientWithOpts 是 buildClient / buildBizClient 共享的 Client 构造辅助。
// 消除两处重复的 `if err != nil { if c != nil { c.Close() }; return nil, err }` 模式
// 统一处理 New() 失败时的资源清理。
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

// registerBizFlags 在业务命令中注册通用 flag（--token, --base-url, --timeout）。
// 消除 6 个命令 init() 中 18 行 flag 重复注册。
// 调用方仍需自行注册命令特有 flag（如 --payload, --comment）。
func registerBizFlags(cmd *cobra.Command) {
	cmd.Flags().String("token", "", "X-Auth-Token（必填，也可通过 NAZHI_TOKEN 环境变量设置）")
	cmd.Flags().String("base-url", "", "业务 API 根地址（默认 http://139.159.205.146:8280，也可通过 NAZHI_BASE_URL 环境变量设置）")
	cmd.Flags().Int("timeout", 15, "HTTP 超时（秒，也可通过 NAZHI_TIMEOUT 环境变量设置）")
}

// buildClient 从 cobra 命令标志构建通用 Client，处理 sso-base / base-url /
// upload-url / timeout 的 env fallback 与 opts 拼接。**不**做 token 必填校验——
// token 必填是业务 API 命令（whoami/task/self-eval/session activate）的
// 约束，SSO 命令（login/school）不需要（组 E 拆分）。
// login/school 等 SSO 命令直接调用。
// 业务命令应调 buildBizClient（基于 buildClientOpts + token 必填校验）。
// 新增 urlType 参数让调用方指定 URL 来源——
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
// 必填标志：token。
// 可选标志：base-url, timeout, sso-base。
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
