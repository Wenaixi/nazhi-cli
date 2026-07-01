package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"sync"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// multipartBufPool 复用 multipart 构造过程的字节缓冲，避免每次 UploadFile
// 都分配 5MB+ 的 bytes.Buffer。F8.3 优化。
var multipartBufPool = sync.Pool{
	New: func() any {
		b := &bytes.Buffer{}
		b.Grow(5*1024 + 1024) // 预分配 5MB+1KB 匹配原 Grow 语义
		return b
	},
}

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
	fileData, mimeType, err := c.prepareImageForUpload(ctx, filePath)
	if err != nil {
		// F3 修复：errors.Join(ErrFileTooLarge, err) 让 ErrFileTooLarge 进错误链，
		// 调用方 errors.Is(err, ErrFileTooLarge) 单一识别所有「文件过大」路径——
		// 不论根因是 image_prep.go L122 的 ErrImageTooLarge（缩放级联到底仍超限）
		// 还是下方的 len(fileData) > MaxImageSize 兜底，二者都通过同一个 sentinel。
		//
		// 注：errors.Is(err, ErrImageTooLarge) 仍命中（pre-existing 行为保留），
		// 只是额外让 ErrFileTooLarge 也进入链。
		return 0, fmt.Errorf("图片预处理失败: %w", errors.Join(ErrFileTooLarge, err))
	}
	if len(fileData) > MaxImageSize {
		// A3 修复：让两条"图片过大"路径的 sentinel 行为一致。
		// 兜底路径也用 errors.Join 包含 ErrImageTooLarge。
		return 0, fmt.Errorf("压缩后仍达 %d 字节: %w", len(fileData),
			errors.Join(ErrFileTooLarge, ErrImageTooLarge))
	}
	c.logDebug("图片预处理完成: %s → %d bytes (mime=%s)", filePath, len(fileData), mimeType)

	// 2. 构造 multipart 请求体
	//
	// 必须显式 writer.Close()，不能在 http.NewRequestWithContext
	// 之后。multipart writer 的终结边界 `--{boundary}--\r\n` 只在 Close() 时追加，
	// 若只 defer Close()，则 wire 上发出去的 body 缺终止边界，server 端 multipart
	// parser 报 EOF 错误，100% 上传失败。
	//
	var buf *bytes.Buffer
	bufObj := multipartBufPool.Get()
	buf, ok := bufObj.(*bytes.Buffer)
	if !ok || buf == nil {
		buf = &bytes.Buffer{}
		// pool 返回了错误类型或 nil，分包新建 buf 时预分配
		// fileData+1KB 空间以避免 multipart 构造时多次扩容
		buf.Grow(len(fileData) + 1024)
	}
	defer func() {
		// pool 缩容：上次上传超大文件后 buf cap 可能 >8MB，
		// 重建小 buf 避免常驻大内存泄漏（PLAUSIBLE: 只有上传极大文件时触发）。
		if buf.Cap() > 8*1024*1024 {
			buf = &bytes.Buffer{}
		} else {
			buf.Reset()
		}
		multipartBufPool.Put(buf)
	}()
	writer := multipart.NewWriter(buf)

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
	uploadURL := c.uploadURL + "/common/upload/uploadImage?bussinessType=12&groupName=other"
	req, err := c.buildRequest(ctx, http.MethodPost, uploadURL, buf, map[string]string{
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
	// 共享 Transport 让连接池/TLS 握手/代理配置复用，批量上传 N 张图时
	// 只需 1 次 DNS+TCP+TLS 握手，后续 keep-alive 复用。
	resp, err := newCleanClient(c).Do(req)
	if err != nil {
		return 0, fmt.Errorf("%w: 上传请求失败: %w", ErrNetwork, err)
	}
	defer drainAndClose(resp.Body)

	// F8.3 优化：先判 status code 再读 body。非 200 时只读 64KB 用于错误消息，
	// 避免大 HTTP 错误响应的 body 全部读入内存（服务端 502/503 有时带完整 HTML 堆栈）。
	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		// A2 修复：复用 request.go 的 classifyHTTPStatus 统一 sentinel 分类。
		sentinel := classifyHTTPStatus(resp.StatusCode, ErrUploadRejected)
		return 0, fmt.Errorf("%w: status=%d body=%s", sentinel, resp.StatusCode, logSafeBody(errBody))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		// 关键修复: 之前用 `bodyBytes, _ := io.ReadAll(resp.Body)` 吞噬错误,
		// 当服务端断网 (connection reset / unexpected EOF) 时 bodyBytes 为空,
		// 后续 json.Unmarshal([]) 返回 EOF, 报「解析上传响应失败: EOF」丢失根因。
		// 现在包装为 ErrNetwork 哨兵, 上层可 errors.Is 识别。
		return 0, fmt.Errorf("%w: 读取上传响应体失败: %w", ErrNetwork, err)
	}

	// 5. 解析响应
	unified, err := types.DecodeResponse(bodyBytes)
	if err != nil {
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
	// 但与 tokenparse.ExtractFromReturnData 保持一致更安全。
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
	var idInt int64
	switch v := rawID.(type) {
	case nil:
		return 0, fmt.Errorf("%w: returnData.id 字段为 null", ErrUploadRejected)
	case float64:
		idInt = int64(v)
	case json.Number:
		// priority: Int64() for integer IDs (>2^53 仍精确), Float64 fallback for decimals.
		// PLAUSIBLE: 仅当 ID >2^53 时 float64 精度损失触发 (json.Number.Float64 → +Inf)。
		idInt, err = v.Int64()
		if err != nil {
			var f float64
			f, err = v.Float64()
			if err != nil {
				return 0, fmt.Errorf("%w: returnData.id 不是合法数字: %w", ErrUploadRejected, err)
			}
			idInt = int64(f)
		}
	default:
		return 0, fmt.Errorf("%w: returnData.id 类型不匹配, 期望 float64 或 json.Number 实际 %T", ErrUploadRejected, rawID)
	}

	return idInt, nil
}

// newCleanClient 构造"无 cookie"的安全 http.Client 供 UploadFile 使用。
//
// 安全保证：独立 http.Client（不共享 c.http.Jar），不发送任何 Cookie /
// Authorization 头，杜绝业务域鉴权信息泄露到文件上传公共服务。
//
// 性能优化：
//   - 每次调用现场 Clone c.http.Transport（type assertion + t.Clone()），
//     共享 Dialer/TLSConfig/代理配置，但 idle 连接池独立。Client.Close() 的
//     CloseIdleConnections 只关闭本次 clean client 自己的 idle 池，不殃及业务
//     Client 到 sso/api 主机的 keep-alive 连接。
//   - 批量上传场景（N 张图）下，每张图产生独立 clean idle 池：keep-alive 复用
//     限于本张图生命周期内的多次重定向/分块下载；图与图之间不会泄露 idle conn
//     到不同 doc server host（若有自定义路由）。Clone 成本是 O(1) struct copy +
//     重置 idle pool，远低于一次完整 DNS+TCP+TLS 握手。
//
// 同时禁用自动重定向（与 SSO 流程策略一致），防止 302 跳转到第三方主机
// 时附带请求头。
//
// 注意：每个 newCleanClient 调用现场 Clone 一次（O(1) struct copy +
// 重置 idle conn pool），确保运行时 c.http.Transport 变更（如测试中
// mock RoundTripper）能被即时感知，不被任何缓存字段粘住。
func newCleanClient(c *Client) *http.Client {
	var transport http.RoundTripper
	switch t := c.http.Transport.(type) {
	case *http.Transport:
		// F5.6 修复：每次现场 Clone，不缓存到 Client 字段。
		// Clone 成本 O(1) struct copy + 重置 idle conn pool，
		// 远低于一次 TLS 握手。运行时 Transport 变更即时感知。
		transport = t.Clone()
	default:
		// nil (fallback http.DefaultTransport) 或自定义 RoundTripper 不 Clone
		//  - http.DefaultTransport 是进程单例，Clone 会创建额外 idle 池
		//  - 自定义 RT 无法 Clone
		if c.http.Transport == nil {
			transport = http.DefaultTransport
		} else {
			transport = c.http.Transport
		}
	}
	timeout := c.http.Timeout
	if timeout != 0 && timeout < 30*time.Second {
		timeout = 30 * time.Second // 文件上传的合理兜底超时，确保不继承主 Client 过短 timeout
	}
	return &http.Client{
		Transport:     transport,
		Timeout:       timeout,
		CheckRedirect: noRedirect,
	}
}
