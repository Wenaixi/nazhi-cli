package main

import (
	"fmt"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/spf13/cobra"
)

// buildClient 从 cobra 命令标志构建通用 Client，处理 sso-base / base-url /
// timeout 的 env fallback 与 opts 拼接。**不**做 token 必填校验——
// token 必填是业务 API 命令（whoami/task/self-eval/session activate）的
// 约束，SSO 命令（login/school）不需要。
//
// login/school 等 SSO 命令直接调用。
// 业务命令应调 buildBizClient（基于 buildClient + token 必填校验）。
func buildClient(cmd *cobra.Command) (*client.Client, error) {
	opts, _, err := buildClientOpts(cmd, false)
	if err != nil {
		return nil, err
	}
	return client.New(opts...), nil
}

// buildBizClient 从 cobra 命令标志构建业务 API Client，自动处理 env fallback。
// 基于 buildClient 之上额外做 token 必填校验。
//
// 必填标志：token。
// 可选标志：base-url, timeout, sso-base。
//
// 返回 (client, token)。
func buildBizClient(cmd *cobra.Command) (*client.Client, string, error) {
	opts, token, err := buildClientOpts(cmd, true)
	if err != nil {
		return nil, "", err
	}
	return client.New(opts...), token, nil
}

// buildClientOpts 构造 client.Option 列表，是 buildClient 与 buildBizClient
// 共享的核心实现。
//
// requireToken=true 时若 token 解析为空则返回 error；否则 token 留空即可。
// 所有 env fallback 在这里统一处理。
func buildClientOpts(cmd *cobra.Command, requireToken bool) ([]client.Option, string, error) {
	token, _ := cmd.Flags().GetString("token")
	if token == "" {
		token = envString("NAZHI_TOKEN", "")
	}
	if requireToken && token == "" {
		return nil, "", fmt.Errorf("--token 为必填（也可通过 NAZHI_TOKEN 环境变量设置）")
	}

	ssoBase, _ := cmd.Flags().GetString("sso-base")
	baseURL, _ := cmd.Flags().GetString("base-url")
	timeoutSec, _ := cmd.Flags().GetInt("timeout")

	if ssoBase == "" {
		ssoBase = envString("NAZHI_SSO_BASE", "")
	}
	if baseURL == "" {
		baseURL = envString("NAZHI_BASE_URL", "")
	}
	if !flagChanged(cmd, "timeout") {
		timeoutSec = envInt("NAZHI_TIMEOUT", 15)
	}

	opts := []client.Option{client.WithTimeout(time.Duration(timeoutSec) * time.Second)}
	if token != "" {
		opts = append(opts, client.WithToken(token))
	}
	if ssoBase != "" {
		opts = append(opts, client.WithSSOBase(ssoBase))
	}
	if baseURL != "" {
		opts = append(opts, client.WithBaseURL(baseURL))
	}
	return opts, token, nil
}
