package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/spf13/cobra"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/Wenaixi/nazhi-cli/pkg/logx"
)

// pendingExitCode 追踪本进程退出码。三分退出码：0 成功 / 1 partial/业务 / 2 服务端 / 3 参数。
// printError 不再调 os.Exit（否则绕过 main 中 defer closeAllClients()）。
// 这里用 atomic.Int32 让 printEnvelope/printError 标记、main 读取，保证
//   - 退出码语义保持原样（出过错则非 0）
//   - defer closeAllClients() 仍能跑（os.Exit 只在 main 最后调一次）
var pendingExitCode atomic.Int32

// maxCLILimit 是四任务命令 --limit 参数的上界。对齐 SDK maxSubmittedRecords
// （pkg/client/submitted.go:138，10 万条单次任务合理上限）：offset+limit 派生
// endPage 不超过服务端 maxTotalPage，避免 SDK 静默返回首页快照。
const maxCLILimit = 100_000

// printErrorDepth 防止递归兜底路径无限递归。
// 当 stderr 本身也无法 JSON 编码时（如 fd 已关），递归兜底会无限递归。
// depth>1 时降级为直写 fmt.Fprintf，避免 stack overflow。
var printErrorDepth atomic.Int32

// markError 标记本进程遇到错误（默认退出码 1）。
// 若需要更精细的退出码（业务错误 1 / 服务端 2 / 参数 3），应直接调
// pendingExitCode.Store(...) 或 printEnvelope 让 envelope.ExitCode() 接管。
func markError() {
	pendingExitCode.Store(1)
}

// printEnvelope 序列化 envelope 到 stdout 并按 ExitCode 标记退出码。
// 这是 CLI 所有 Run 回调的统一出口。
func printEnvelope(e *envelope.Envelope) {
	if e == nil {
		return
	}
	// 统一脱敏（C2）：Message 可能直拼底层错误链（login.go 等的 err.Error() 包含
	// SDK 已部分脱敏文本；若未来新增未脱敏错误片段，stdout 通道不得静默泄露）。
	// RedactBody 幂等（已有值再脱敏不影响），保持与 printError 的 stderr 通道同口径。
	if e.Message != "" {
		e.Message = logx.RedactBody(e.Message)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(e); err != nil {
		markError()
		if !quiet {
			printError(fmt.Errorf("序列化 envelope 失败: %w", err))
		}
		return
	}
	if code := e.ExitCode(); code != 0 {
		pendingExitCode.Store(int32(code))
	}
}

// mapSentinelToHTTPCode 按错误链中的哨兵归类 HTTP 码：
// 参数类（含本地文件超限 ErrFileTooLarge）→ 400（exit 3）；业务拒绝/服务端明确 4xx → 422（exit 1）；
// 限流/会话冷却 → 429（exit 1，客户端已知应等待的确定性状态）；可重试取消 → 503；
// 网络/超时/5xx → 502（exit 2）；未识别保持 500。
// 免验证码端点不再使用 OCR 哨兵；遗留注释已清理。
func mapSentinelToHTTPCode(err error) int {
	switch {
	case errors.Is(err, client.ErrInvalidPayload),
		errors.Is(err, client.ErrFileTooLarge):
		return 400
	// 修订：本地文件系统错误（上传附件不存在/下载路径不可写）由 SDK 侧包
	// ErrInvalidPayload 哨兵归 400/exit3（见 file.go/image_prep.go）。
	// 不再在漏斗层匹配 *fs.PathError——stdin 读错误（关闭句柄）也是 PathError 链，
	// 且既有契约锁定其为 exit 2（管道场景可瞬时恢复，不应与永久性参数错误混淆）。
	// 网络/限流/超时/5xx 类哨兵必须先于 ErrBusinessRejected 判定。
	// FetchTasks/FetchTasksJSON 全维度失败汇总时外层包 ErrBusinessRejected、内层
	// errors.Join 的子错误已带网络类哨兵（%w 链）；若先命中 422 分支，服务端整体
	// 宕机会被误报为「业务拒绝」——脚本按 exit1 无退避重放，违反「网络/限流/超时
	// 用稳定哨兵、HTTP 映射 502/503/429」契约。顺序即优先级：确定性服务端/网络类
	// 在前，业务拒绝兜底在后。
	case errors.Is(err, client.ErrRateLimited),
		errors.Is(err, client.ErrSessionBackoff):
		return 429
	case errors.Is(err, client.ErrRetryable):
		return 503
	case errors.Is(err, client.ErrNetwork),
		errors.Is(err, client.ErrTimeout),
		errors.Is(err, client.ErrServiceUnavailable):
		return 502
	// ErrLoginRejected 优先于泛化业务拒绝 422——「登录取证失败」
	// 是明确的认证拒绝，与 login.go 专属中文 401 分支语义对齐（exit 恒 1，
	// 但 HTTP 码契约应收敛为 401：认证失败不是 422 未处理实体）。
	case errors.Is(err, client.ErrLoginRejected):
		return 401
	case errors.Is(err, client.ErrBusinessRejected),
		errors.Is(err, client.ErrInvalidResponse),
		errors.Is(err, client.ErrUploadRejected):
		return 422
	// 空成功链路与配置侧异常：ErrEmptyUserInfo 是 getMyInfo 成功却无用户数据，
	// ErrAllDecodersFailed 是所有解码器都未命中，ErrCookieSyncFailed 是登录成功
	// 但 token 同步到 cookie jar 失败（客户端配置问题，如 WithHTTPClient 传了
	// 非 *cookiejar.Jar）。三者都属服务端/配置侧异常，此前落 default 500。
	case errors.Is(err, client.ErrEmptyUserInfo),
		errors.Is(err, client.ErrAllDecodersFailed),
		errors.Is(err, client.ErrCookieSyncFailed):
		return 502
	// 不应报为服务端内部错误。与 ErrRetryable 同档——SDK 侧该哨兵的
	// 注释即定义为「ctx cancel 引发的可重试语义标记」。脚本据此决定重试。
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return 503
	default:
		return 500
	}
}

// printError 输出错误信封到 stderr：按哨兵映射 HTTP 码决定退出码档位。
// 注意：此函数**不**调用 os.Exit。退出由 main 在 rootCmd.Execute() 之后
// 统一处理。原因：os.Exit 不执行 defer，直接退出会导致 main 的
// defer closeAllClients() 永远不运行，HTTP 连接池等资源全部泄漏。
// 退出码契约
//   - printError 仅写 stderr + 设 pendingExitCode（按 envelope.ExitCode）
//   - 调用方（cobra Run 回调）保持原样 `printError(err); return`
//   - main 在 Execute 返回非 nil 或 pendingExitCode!=0 时统一 os.Exit
//
// 参数错误（缺 token、payload 读/解析失败）请用 printParamError（400→exit 3）。
func printError(err error) {
	printErrorWithCode(err, mapSentinelToHTTPCode(err))
}

// printParamError 输出参数错误 envelope.Error(400) 到 stderr，退出码 3。
// 用于 buildBizClient 失败（缺 token）、payload 读取/JSON 解析失败等
// 调用方可控的输入问题，与服务端/网络错误（printError → 500/exit 2）区分。
func printParamError(err error) {
	printErrorWithCode(err, 400)
}

// rejectLoneOffset 校验 --offset 合法性：单独 --offset（无 --limit）、负值或
// --limit 超过上界 maxCLILimit 时输出参数错误信封并返回 true。offset>0 而
// limit<=0 会被 SDK 全量路径静默忽略；offset<0 在 limit 模式下等效归零、全量
// 模式下整体失效；limit 过大（>maxCLILimit）会让 SDK 分页派生 endPage 超服务端
// maxTotalPage，SDK 静默返回首页快照——分页脚本拿首页当 top-N
// 而不知情。四命令统一拒绝以防静默错误数据。
//
// 调用次序（见 CLAUDE.md「D. CLI 契约」rejectLoneOffset 披露）：本函数允许在 buildBizClient 之后调用（task_teacher/
// task_public/task_submitted/task_withdrawn 四命令均如此），四命令已把 rejectLoneOffset
// 前移到 onlyCount 分支前——--count --limit 5 不再绕过校验；honor
// delete / typical-case delete 等先校后建派的双参数缺失时首报消息与 stdout/stderr
// 通道漂移（退出码恒 3 无损）。四任务命令的校验块已在 onlyCount 前（task_teacher
// 等文件同步位置），此处仅保留函数本体供四命令/未来调用方复用。
func rejectLoneOffset(cmd *cobra.Command) bool {
	offset, _ := cmd.Flags().GetInt("offset")
	limit, _ := cmd.Flags().GetInt("limit")
	// --count 与 --limit/--offset 语义互斥——count 模式只输总数，
	// 静默忽略 limit/offset 会让脚本拿错形状不自知。四命令统一拒绝。
	if onlyCount, _ := cmd.Flags().GetBool("count"); onlyCount && (limit > 0 || offset > 0) {
		printEnvelope(envelope.Error(400, "--count 不能与 --limit/--offset 同用（count 只输出记录总数）"))
		return true
	}
	if (offset > 0 && limit <= 0) || offset < 0 || limit < 0 {
		// 违规参数可能是 --limit 负值而非 --offset——文案必须同时点名两个
		// 参数，避免用户只看到 "--offset" 却摸不着为什么 --limit -1 也被拒。
		printEnvelope(envelope.Error(400, "--offset/--limit 需为非负数且 offset 仅配合 --limit 使用（非法取值会被忽略或归零，拒绝静默返回错误数据）"))
		return true
	}
	if limit > maxCLILimit {
		// limit 超上界 → SDK endPage 超 maxTotalPage 静默只翻首页，
		// 脚本拿截断数据不自知。参数错误拒绝（对齐 400/exit3 半套纪律：≤0 已拒、
		// 超上界同族拒绝）。上界与 SDK maxSubmittedRecords 对齐（submitted.go:138，
		// 10 万条单次任务合理上限，offset+limit 分页不会触发首页截断）。
		printEnvelope(envelope.Error(400, fmt.Sprintf("--limit 不能超过 %d（避免分页派生 endPage 触发服务端首页截断）", maxCLILimit)))
		return true
	}
	return false
}

// redactErrorMessage 统一处理 CLI 错误信封中的底层错误文本，避免 URL 查询参数中的
// token、userName 等敏感值绕过请求层脱敏后直接回显。
func redactErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	return logx.RedactBody(err.Error())
}

// printErrorWithCode 是 printError / printParamError 的共享实现。
func printErrorWithCode(err error, httpCode int) {
	if err == nil {
		return
	}
	// depth 守卫：递归调用只在 depth==0 时触发，避免 stderr fd 关闭时死循环。
	if printErrorDepth.Add(1) > 1 {
		// 二次调用（兜底路径又失败）→ 直接降级为 fmt.Fprintf，不再递归
		if !quiet {
			_, _ = fmt.Fprintf(os.Stderr, "ERROR: %s\n", redactErrorMessage(err))
		}
		printErrorDepth.Add(-1)
		return
	}
	defer printErrorDepth.Add(-1)

	// 把 error 包成 envelope，按 ExitCode 设退出码。
	e := envelope.Error(httpCode, redactErrorMessage(err))

	// quiet 模式下只标记退出码，不写 stderr
	if !quiet {
		enc := json.NewEncoder(os.Stderr)
		enc.SetIndent("", "  ")
		if enc.Encode(e) != nil {
			// stderr 写入失败时无法输出信封，但退出码仍必须反映原始错误。
			pendingExitCode.Store(int32(e.ExitCode()))
			printErrorWithCode(fmt.Errorf("printError JSON 编码失败: %w", err), httpCode)
			return
		}
	}
	if code := e.ExitCode(); code != 0 {
		pendingExitCode.Store(int32(code))
	}
}

// printVerbose 输出日志到 stderr（仅在 verbose 模式下且非 quiet）。
// 加 [verbose] 前缀，与 printError JSON envelope 区分
// 避免 verbose 日志被错误接收方误解析为 JSON 错误。
func printVerbose(format string, args ...any) {
	if verbose && !quiet {
		fmt.Fprintf(os.Stderr, "[verbose] "+format+"\n", args...)
	}
}

// printPrompt 向 stderr 写入交互提示。不受 verbose 守卫，但受 quiet 与终端检测双重守卫。
// 用途：self-eval submit 等从 stdin 读取输入的命令，需要在用户终端看到
// "请输入 xxx: " 提示符才能知道要敲字。走 printVerbose 用户没加 -v 看不到提示；
// 走 printError 会以 JSON envelope 污染 stderr 错误流。
// 守卫：
//   - 仅在 isTerminalStdin()==true 时输出（CI / 管道环境下无意义）
//   - quiet 模式不输出（用户显式要求静默）
func printPrompt(prompt string) {
	if quiet {
		return
	}
	if !isTerminalStdin() {
		return
	}
	fmt.Fprint(os.Stderr, prompt)
}
