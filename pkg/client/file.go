package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// UploadFile 上传图片到文件服务器，返回图片 ID。
//
// ⚠️ 关键约束：本方法不发送任何 Token / Cookie / Authorization 头。
// 文件服务器（doc.nazhisoft.com）是独立公共服务，不需要业务域鉴权。
// SDK 内部使用独立的 clean http.Client（无 cookie jar），杜绝任何鉴权头泄露。
//
// ⚠️ 域隔离约束：syncCookieToken 只在 c.baseURL 域写入 X-Auth-Token cookie，
// 而 UploadFile 走 c.uploadURL 域（独立文件服务器）。
// 若 c.uploadURL 与 c.baseURL 指向同一主机（自定义部署场景），
// 则 syncCookieToken 写入的 cookie 在上传请求中不会泄漏（newCleanClient 无 cookie jar）。
// 但调用方应注意不要在业务 Client 的 baseURL 域上传敏感文件。
//
// 上传前自动预处理：任意格式 → JPG + 透明合成 + 压缩至 ≤ 5MB。
// 全部在内存中完成，不写盘、不修改原文件。
func (c *Client) UploadFile(ctx context.Context, filePath string) (int64, error) {
	// 1. 图片预处理
	fileData, mimeType, err := c.prepareImageForUpload(filePath)
	if err != nil {
		return 0, fmt.Errorf("图片预处理失败: %w", err)
	}
	if len(fileData) > MaxImageSize {
		return 0, fmt.Errorf("%w: 压缩后仍达 %d 字节（上限 %d）", ErrFileTooLarge, len(fileData), MaxImageSize)
	}
	c.logDebug("图片预处理完成: %s → %d bytes (mime=%s)", filePath, len(fileData), mimeType)

	// 2. 构造 multipart 请求体
	//
	// 必须显式 writer.Close()，不能在 http.NewRequestWithContext
	// 之后。multipart writer 的终结边界 `--{boundary}--\r\n` 只在 Close() 时追加，
	// 若只 defer Close()，则 wire 上发出去的 body 缺终止边界，server 端 multipart
	// parser 报 EOF 错误，100% 上传失败。
	//
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filePath+".jpg")
	if err != nil {
		return 0, fmt.Errorf("创建 multipart form 失败: %w", err)
	}
	if _, err := part.Write(fileData); err != nil {
		return 0, fmt.Errorf("写入图片到 multipart 失败: %w", err)
	}

	// 显式 Close：在 NewRequest 之前写入终结边界到 buf。
	// 注意：不保留 defer writer.Close()——显式 Close 在上方已经执行，
	// writer 在 NewRequest 前已完成终结边界的写入。CreateFormFile 和
	// part.Write 在 Close 前已返回，CreateFormFile/Write 路径无另存早退点。
	if err := writer.Close(); err != nil {
		return 0, fmt.Errorf("关闭 multipart writer 失败: %w", err)
	}

	// 3. 构造请求
	//
	// 走共享 buildRequest helper，消除手工 NewRequestWithContext
	// 特例路径。与 doRequest/doBizGet 等其他 SDK 方法统一，享受 buildRequest
	// 的演进（如 debug 日志脱敏、req body 校验等无需在此同步）。
	//
	// multipart 场景下 Content-Type 必填（含 boundary），由 writer.FormDataContentType()
	// 提供；body 传入 *bytes.Buffer（满足 io.Reader 接口），buildRequest 透传。
	uploadURL := c.uploadServiceURL("/common/upload/uploadImage?bussinessType=12&groupName=other")
	req, err := c.buildRequest(ctx, http.MethodPost, uploadURL, &buf, map[string]string{
		"Accept":     "application/json, text/plain, */*",
		"User-Agent": defaultUserAgent,
	}, writer.FormDataContentType())
	if err != nil {
		return 0, fmt.Errorf("%w: 创建上传请求失败: %w", ErrNetwork, err)
	}

	// 4. 关键安全措施：使用独立的 clean http.Client（无 cookie jar）
	//
	// 即使用户复用了已登录的 Client（cookie jar 里有 X-Auth-Token），
	// 这里也用全新的 client.Do() 发请求，确保不会泄露任何 Cookie。
	// 同时禁用自动重定向（CheckRedirect=ErrUseLastResponse），与 SSO 流程策略一致，
	// 防止 302 跳转到第三方主机时附带请求头。
	//
	// 共享 Transport 让连接池/TLS 握手/代理配置复用，批量上传 50 张图时
	// 只需 1 次 DNS+TCP+TLS 握手，后续 keep-alive 复用。
	resp, err := newCleanClient(c).Do(req)
	if err != nil {
		return 0, fmt.Errorf("%w: 上传请求失败: %w", ErrNetwork, err)
	}
	defer drainAndClose(resp.Body)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		// 关键修复: 之前用 `bodyBytes, _ := io.ReadAll(resp.Body)` 吞噬错误,
		// 当服务端断网 (connection reset / unexpected EOF) 时 bodyBytes 为空,
		// 后续 json.Unmarshal([]) 返回 EOF, 报「解析上传响应失败: EOF」丢失根因。
		// 现在包装为 ErrNetwork 哨兵, 上层可 errors.Is 识别。
		return 0, fmt.Errorf("%w: 读取上传响应体失败: %w", ErrNetwork, err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("%w: status=%d body=%s", ErrUploadRejected, resp.StatusCode, logSafeBody(bodyBytes))
	}

	// 5. 解析响应
	var unified types.UnifiedResponse
	if err := json.Unmarshal(bodyBytes, &unified); err != nil {
		return 0, fmt.Errorf("解析上传响应失败: %w", err)
	}

	// I3 修复：故意不走 types.CheckCode，统一响应码 ≠ 1 仍用
	// ErrUploadRejected 包装。语义边界：
	//   - ErrUploadRejected: 上传文件域业务错误（独立公共服务，无 cookie 鉴权），
	//     SDK 用户应单独判定（如限制文件类型、重试上传）
	//   - ErrBusinessRejected: 业务 API 域拒绝（session 过期、参数错），
	//     SDK 用户按 docs/sdk/README.md 推荐 errors.Is(ErrBusinessRejected) 重激活
	// 两者不可合并——上传服务与业务 API 是独立服务域，错误处理路径完全不同。
	if unified.Code != 1 {
		return 0, fmt.Errorf("%w: code=%d", ErrUploadRejected, unified.Code)
	}

	// 6. 从 returnData 提取 id
	if unified.ReturnData == nil {
		return 0, fmt.Errorf("%w: 响应中缺少 returnData", ErrUploadRejected)
	}

	var result map[string]any
	// 这里无法用 types.DecodeReturnData[map[string]any] 替代手写 decoder。
	//
	// DecodeReturnData 用 json.Unmarshal，默认将数字解为 float64。
	// 而当前代码用 json.NewDecoder + UseNumber 将数字解为 json.Number，
	// 避免文件 ID 在 >2^53 时的 float64 精度损失。虽然文件 ID 通常在此范围内，
	// 但与 auth.go extractTokenFromReturnData 保持一致更安全。
	//
	// 如果未来 DecodeReturnData 支持 UseNumber 模式，可以迁移。
	dec := json.NewDecoder(bytes.NewReader(*unified.ReturnData))
	dec.UseNumber()
	if err := dec.Decode(&result); err != nil {
		return 0, fmt.Errorf("解析 returnData 失败: %w", err)
	}

	// J2 修复：先区分字段是否存在，再做类型断言。
	// 修复前 `id, ok := result["id"].(float64); if !ok` 把「字段不存在」与
	// 「类型不匹配」两种根因合并成同一条「缺少 id 字段」，导致 type mismatch
	// 误导用户去检查协议而非数据类型。
	rawID, exists := result["id"]
	if !exists {
		return 0, fmt.Errorf("%w: returnData 中缺少 id 字段", ErrUploadRejected)
	}
	// decode returnData 采用 UseNumber 一致地解析 json.Number，
	// 但 float64 断言也要兼容——json.Number 需通过 Float64() 转换。
	var idFloat float64
	switch v := rawID.(type) {
	case nil:
		return 0, fmt.Errorf("%w: returnData.id 字段为 null", ErrUploadRejected)
	case float64:
		idFloat = v
	case json.Number:
		idFloat, err = v.Float64()
		if err != nil {
			return 0, fmt.Errorf("%w: returnData.id 不是合法数字: %v", ErrUploadRejected, err)
		}
	default:
		return 0, fmt.Errorf("%w: returnData.id 类型不匹配, 期望 float64 或 json.Number 实际 %T", ErrUploadRejected, rawID)
	}

	return int64(idFloat), nil
}

// newCleanClient 构造"无 cookie"的安全 http.Client 供 UploadFile 使用。
//
// 安全保证：独立 http.Client（不共享 c.http.Jar），不发送任何 Cookie /
// Authorization 头，杜绝业务域鉴权信息泄露到文件上传公共服务。
//
// 性能优化：
//   - Clone c.http.Transport 共享 Dialer/TLSConfig/代理配置，
//     但 idle 连接池独立。Client.Close() 的 CloseIdleConnections
//     只关闭 clean client 自己的 idle 池，不殃及业务 Client 到 sso/api 主机的
//     keep-alive 连接。
//   - B1：clonedTransport 缓存在 c.cleanTransport 字段，由 sync.Once 保护，
//     首次 Clone 后复用同一实例。修复前每次 UploadFile 都 t.Clone() 一次，
//     50 张图 = 50 次完整 DNS+TCP+TLS 握手（每次 Clone 都产生新 Transport 实例，
//     累加的 idle 连接池被丢弃，keep-alive 完全失效）。修复后 50 张图共享同一
//     clean idle 池，TLS 握手仅 1 次。
//
// 同时禁用自动重定向（与 SSO 流程策略一致），防止 302 跳转到第三方主机
// 时附带请求头。
//
// 注意：cleanTransport 通过 sync.Once 缓存一次后不再感知运行时
// c.http.Transport 的变更。典型场景：测试中第一次 UploadFile 后动态替换
// Transport（如 mock RoundTripper），新 Transport 不会生效。此限制是 B1
// 缓存设计的有意取舍——运行时 Transport 变更在业务实践中极罕见，且需重建
// Client（sync.Once 重置不可逆）。
func newCleanClient(c *Client) *http.Client {
	// B1：懒加载 cloned Transport，sync.Once 保证并发安全且只 Clone 一次
	c.cleanTransportInit.Do(func() {
		switch t := c.http.Transport.(type) {
		case *http.Transport:
			// Clone 出独立 Transport：配置共享但 idle 池独立
			c.cleanTransport = t.Clone()
		default:
			// nil (fallback http.DefaultTransport) 或自定义 RoundTripper 不缓存
			//  - http.DefaultTransport 是进程单例，多 Client 共享缓存会引入隐性耦合
			//  - 自定义 RT 无法 Clone
		}
	})

	var transport http.RoundTripper
	if c.cleanTransport != nil {
		transport = c.cleanTransport
	} else if c.http.Transport == nil {
		// 原 Client 没设置 Transport，回退到 http.DefaultTransport
		//（不 Clone DefaultTransport——它是全局共享进程单例）
		transport = http.DefaultTransport
	} else if t, ok := c.http.Transport.(*http.Transport); ok {
		// *http.Transport 但未进入缓存路径（如运行时替换 Transport），
		// 仍 Clone 出独立实例，不共享 idle 池。
		transport = t.Clone()
	} else {
		// 自定义 RoundTripper（如 mock 测试用）无法 Clone，直接透传
		// 此时不存在 idle 池共享问题（mock 通常不维护连接池）
		transport = c.http.Transport
	}

	timeout := c.http.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second // 文件上传的合理兜底超时
	}
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
