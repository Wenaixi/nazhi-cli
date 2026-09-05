package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/logx"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// MaxAttachmentSize 是非图片附件直传的上限（20MB，SDK 有意放宽：前端镜像文案 20MB，
// 服务端实测无 2MB 硬限、真实上限约 46.86MiB，见 CLAUDE.md 规范 #26）。
// 图片走压缩路径，上限为 MaxImageSize（5MB，SDK 放宽）；两者分开校验。
const MaxAttachmentSize = 20 * 1024 * 1024

// maxImageDecodePreflight 是图片分支进入解码管线前的体积预检线（200MB）。
// 高压缩比畸形大图可在 image.Decode 阶段把内存放大数十倍；与附件分支的 Stat 预检
// 对称防御。合法相机原图/全景图远低于此线，超线直接拒绝不做解码尝试。
const maxImageDecodePreflight = 200 * 1024 * 1024

var directUploadExtensions = map[string]struct{}{
	".pdf":  {},
	".mp4":  {},
	".txt":  {},
	".doc":  {},
	".docx": {},
	".wps":  {},
	".rar":  {},
	".zip":  {},
}

// multipartBufPool 复用 multipart 构造过程的字节缓冲，避免每次 UploadFile 都分配大块缓冲。
var multipartBufPool = sync.Pool{
	New: func() any {
		b := &bytes.Buffer{}
		// 预分配 MaxImageSize+1KB（约 5MB），匹配压缩后图片上限，避免 multipart 构造时多次扩容。
		b.Grow(MaxImageSize + 1024)
		return b
	},
}

// isDirectUploadAttachment 判断文件是否属于前端允许原样上传的非图片附件。
func isDirectUploadAttachment(filePath string) bool {
	_, ok := directUploadExtensions[strings.ToLower(filepath.Ext(filePath))]
	return ok
}

// UploadFile 上传图片或前端允许的非图片附件，返回附件 ID。
//
// 关键约束：本方法不发送任何 Token / Cookie / Authorization 头。
// 文件服务器（doc.nazhisoft.com）是独立公共服务，不需要业务域鉴权。
// SDK 内部使用独立的 clean http.Client（无 cookie jar），全程不携带任何鉴权头。
//
// 域隔离约束：syncCookieToken 只在 c.baseURL 域写入 X-Auth-Token cookie，
// 而 UploadFile 走 c.uploadURL 域（独立文件服务器）。
// 若 c.uploadURL 与 c.baseURL 指向同一主机（自定义部署场景），
// 则 syncCookieToken 写入的 cookie 在上传请求中不会泄漏（newCleanClient 无 cookie jar）。
// 但调用方应注意不要在业务 Client 的 baseURL 域上传敏感文件。
//
// 上传前自动预处理：
//   - 图片：任意格式 → JPG + 透明合成 + 压缩至 ≤ 5MB（MaxImageSize，SDK 放宽）
//   - 非图片附件（.mp4/.txt/.doc/.docx/.wps/.rar/.zip 等前端允许格式）：原样直传，上限 20MB（MaxAttachmentSize，SDK 有意放宽；前端镜像文案 20MB，服务端真实上限约 46.86MiB）
//
// 统一说明：图片压缩后 5MB（SDK 放宽），非图片附件 20MB（SDK 有意放宽，服务端实测支撑）
// 全部在内存中完成，不写盘、不修改原文件。
func (c *Client) UploadFile(ctx context.Context, filePath string) (*types.UploadFileResult, error) {
	// 1. 准备上传字节。图片继续走预处理；非图片附件按前端允许格式原样直传。
	var (
		fileData []byte
		mimeType string
		formName string
		err      error
	)
	if isDirectUploadAttachment(filePath) {
		// 先 Stat 预检大小再读入内存，避免超大文件整体吃进内存后才被拒绝
		if st, statErr := os.Stat(filePath); statErr == nil && st.Size() > MaxAttachmentSize {
			return nil, fmt.Errorf("附件超过 %d 字节: %w", MaxAttachmentSize, ErrFileTooLarge)
		}
		fileData, err = os.ReadFile(filePath)
		if err != nil {
			// FILE-1：本地 IO 错误（文件不存在/无权限）是调用方可控输入问题，
			// 包 ErrInvalidPayload 让 CLI 漏斗归 400/exit3，而非 500/exit2 被脚本无限重试。
			return nil, fmt.Errorf("读取附件失败: %w", errors.Join(ErrInvalidPayload, err))
		}
		if len(fileData) > MaxAttachmentSize {
			return nil, fmt.Errorf("附件超过 %d 字节: %w", MaxAttachmentSize, ErrFileTooLarge)
		}
		formName = filepath.Base(filePath)
		mimeType = mime.TypeByExtension(filepath.Ext(formName))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		c.logDebugCtx(ctx, "非图片附件原样上传: %s → %d bytes (mime=%s)", filePath, len(fileData), mimeType)
	} else {
		// 与附件分支对称的解码前体积预检：畸形大图在 image.Decode 全量读入并放大内存之前拒绝
		if st, statErr := os.Stat(filePath); statErr == nil && st.Size() > maxImageDecodePreflight {
			return nil, fmt.Errorf("图片超过 %d 字节，拒绝进入解码管线: %w", maxImageDecodePreflight, ErrFileTooLarge)
		}
		fileData, mimeType, err = c.prepareImageForUpload(filePath)
		if err != nil {
			// 仅当根因确为「压缩后仍超限」时把 ErrFileTooLarge 并入错误链。
			// 路径不存在 / 解码失败等不应 errors.Is(ErrFileTooLarge)，否则调用方会误判。
			// 真正过大时 image_prep 返回 ErrImageTooLarge（或下方 len 兜底 Join）。
			if errors.Is(err, ErrImageTooLarge) {
				return nil, fmt.Errorf("图片预处理失败: %w", errors.Join(ErrFileTooLarge, err))
			}
			return nil, fmt.Errorf("图片预处理失败: %w", err)
		}
		if len(fileData) > MaxImageSize {
			// 与压缩主路径的 sentinel 行为保持一致：
			// 兜底路径也用 errors.Join 包含 ErrImageTooLarge。
			return nil, fmt.Errorf("压缩后仍达 %d 字节: %w", len(fileData),
				errors.Join(ErrFileTooLarge, ErrImageTooLarge))
		}
		formName = strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath)) + ".jpg"
		c.logDebugCtx(ctx, "图片预处理完成: %s → %d bytes (mime=%s)", filePath, len(fileData), mimeType)
	}

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
	writer := multipart.NewWriter(buf)

	// formName 仅使用 basename，图片已在上方统一为 .jpg，非图片保留原扩展名。
	// 禁止把本地绝对路径塞进 Content-Disposition，避免路径泄露与服务端解析异常。
	part, err := writer.CreateFormFile("file", formName)
	if err != nil {
		return nil, fmt.Errorf("创建 multipart form 失败: %w", err)
	}
	if _, err := part.Write(fileData); err != nil {
		return nil, fmt.Errorf("写入图片到 multipart 失败: %w", err)
	}

	// 显式 Close：在 NewRequest 之前写入终结边界到 buf。
	// 注意：不保留 defer writer.Close()——显式 Close 在上方已经执行，
	// writer 在 NewRequest 前已完成终结边界的写入。CreateFormFile 和
	// part.Write 在 Close 前已返回，CreateFormFile/Write 路径无另存早退点。
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("关闭 multipart writer 失败: %w", err)
	}

	// ⚠️ 关键顺序：先拷贝完整 body bytes，再回收 buf。HTTP transport 的
	// writeLoop goroutine 异步读取请求体（io.Reader），若 buf 被复用归还 pool，
	// writeLoop 可能读到已重置的缓冲区 → data race。
	// 解法：拷贝到独立切片，立即回收 buf 到 pool。
	bodyData := make([]byte, buf.Len())
	copy(bodyData, buf.Bytes())
	// 回收 buf：bodyData 已持有完整 payload，buf 不再被引用。
	if buf.Cap() > 8*1024*1024 {
		buf = &bytes.Buffer{}
	} else {
		buf.Reset()
	}
	multipartBufPool.Put(buf)

	// 3. 构造请求
	//
	// 走共享 buildRequest helper，消除手工 NewRequestWithContext
	// 特例路径。与 doRequest/doBizGet 等其他 SDK 方法统一，享受 buildRequest
	// 的演进（如 debug 日志脱敏、req body 校验等无需在此同步）。
	//
	// multipart 场景下 Content-Type 必填（含 boundary），由 writer.FormDataContentType()
	// 提供；body 传入 bytes.NewReader(bodyData)（独立只读副本，无并发安全风险）。
	uploadURL := c.uploadURL + "/common/upload/uploadImage?bussinessType=12&groupName=other"
	req, err := c.buildRequest(ctx, http.MethodPost, uploadURL, bytes.NewReader(bodyData), map[string]string{
		"Accept":     "application/json, text/plain, */*",
		"User-Agent": defaultUserAgent,
	}, writer.FormDataContentType())
	if err != nil {
		return nil, fmt.Errorf("%w: 创建上传请求失败: %w", ErrNetwork, err)
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
		return nil, fmt.Errorf("%w: 上传请求失败: %w", ErrNetwork, err)
	}
	defer drainAndClose(resp.Body)

	// 先判 status code 再读 body。非 200 时只读 64KB 用于错误消息，
	// 避免大 HTTP 错误响应的 body 全部读入内存（服务端 502/503 有时带完整 HTML 堆栈）。
	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		// 复用 request.go 的 classifyHTTPStatus 统一 sentinel 分类。
		sentinel := classifyHTTPStatus(resp.StatusCode, ErrUploadRejected)
		return nil, fmt.Errorf("%w: status=%d body=%s", sentinel, resp.StatusCode, logx.RedactBodyThenTruncate(errBody, 100))
	}

	// P2-1：上传成功路径响应体同样封顶 1MB（对齐 request.go httpDo 的 HTTP-2 双守卫）。
	// 正常上传响应为几百字节 JSON（HAR 实证），超限仅防异常/被劫持服务端内存放大。
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize+1))
	if err != nil {
		// 读取失败时包装为 ErrNetwork 哨兵，供 errors.Is 识别；
		// 不吞错误避免后续解码报误导性 EOF。
		return nil, fmt.Errorf("%w: 读取上传响应体失败: %w", ErrNetwork, err)
	}
	if len(bodyBytes) > maxResponseBodySize {
		// P2-3（19 轮审计）：超限分支直 Close 放弃 keep-alive，不再经 defer drainAndClose
		// 无上限续读剩余 body——对齐 httpDo:377-381 的 2356484 修复纪律。
		// 恶意无限流下旧实现会 drain 到 newCleanClient 超时（主 Client 无超时兜底 5 分钟）。
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%w: 上传响应体超过 %d 字节上限", ErrInvalidResponse, maxResponseBodySize)
	}

	// 5. 解析响应。200+非 JSON（WAF 挑战页/维护页 HTML）与主管线同口径归 ErrInvalidResponse，
	// 保证 errors.Is 判定全覆盖、退出码漏斗不再落 default。
	unified, err := types.DecodeResponse(bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("解析上传响应失败: %w: %w", ErrInvalidResponse, err)
	}

	// 故意不走 types.CheckCode，统一响应码 ≠ 1 仍用
	// ErrUploadRejected 包装。语义边界：
	//   - ErrUploadRejected: 上传文件域业务错误（独立公共服务，无 cookie 鉴权），
	//     SDK 用户应单独判定（如限制文件类型、重试上传）
	//   - ErrBusinessRejected: 业务 API 域拒绝（session 过期、参数错），
	//     SDK 用户按 docs/README.md 推荐 errors.Is(ErrBusinessRejected) 重激活
	// 两者不可合并——上传服务与业务 API 是独立服务域，错误处理路径完全不同。
	if unified.Code != 1 {
		return nil, fmt.Errorf("%w: code=%d", ErrUploadRejected, unified.Code)
	}

	// 6. 从 returnData 提取 id
	if unified.ReturnData == nil {
		return nil, fmt.Errorf("%w: 响应中缺少 returnData", ErrUploadRejected)
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
		return nil, fmt.Errorf("解析 returnData 失败: %w", err)
	}

	// 先判字段是否存在再断言类型，区分『缺少 id』与『类型不匹配』两种根因。
	rawID, exists := result["id"]
	if !exists {
		return nil, fmt.Errorf("%w: returnData 中缺少 id 字段", ErrUploadRejected)
	}
	// decode returnData 采用 UseNumber 一致地解析 json.Number，
	// 但 float64 断言也要兼容——json.Number 需通过 Float64() 转换。
	var idInt int64
	var nameStr string
	switch v := rawID.(type) {
	case nil:
		return nil, fmt.Errorf("%w: returnData.id 字段为 null", ErrUploadRejected)
	case float64:
		idInt = int64(v)
	case json.Number:
		idInt, err = v.Int64()
		if err != nil {
			var f float64
			f, err = v.Float64()
			if err != nil {
				return nil, fmt.Errorf("%w: returnData.id 不是合法数字: %w", ErrUploadRejected, err)
			}
			idInt = int64(f)
		}
	default:
		return nil, fmt.Errorf("%w: returnData.id 类型不匹配, 期望 float64 或 json.Number 实际 %T", ErrUploadRejected, rawID)
	}

	// 尝试读取 name 字段（可能不存在）
	if rawName, exists := result["name"]; exists {
		if s, ok := rawName.(string); ok {
			nameStr = s
		}
	}

	return &types.UploadFileResult{AttachmentID: idInt, AttachmentName: nameStr}, nil
}

// DownloadFile 按附件 ID 从公开文件服务器下载图片到本地 dst。
//
// 流程：
//  1. GET {ssoBaseURL}/common/attachment/getImg?id={attachmentID}
//  2. 服务端 302 重定向到 doc.nazhisoft.com/other/M00/...（FastDFS 真实存储）
//  3. 跟随重定向（最多 maxDownloadRedirects 次）→ 取真实图片
//  4. 写入本地 dst
//
// 安全约束（与 UploadFile 对称）：
//   - 不发送任何 Token / Cookie / Authorization 头（公开服务）
//   - 重定向仅跟随到 nazhisoft.com / doc.nazhisoft.com 同主机域，
//     防止恶意 Location 跳转到第三方主机
//   - 重定向次数上限 5，防止恶意循环
//
// 错误契约：
//   - HTTP 429 → ErrRateLimited；5xx → ErrServiceUnavailable；
//     其余非 2xx（403/404 等服务端明确拒绝）→ ErrInvalidResponse
//     （与 request.go httpDo/doBizGet、session.go doGetMenu 同口径；4xx 不是网络故障，
//     归 ErrNetwork 会让脚本按可重试语义对永久失败无限重试）
//   - 写入失败 → fmt.Errorf("写入文件失败: %w", err)
//   - 重定向超过上限 → fmt.Errorf("重定向次数超过 %d 次", maxDownloadRedirects)
//   - 跨域重定向 → fmt.Errorf("拒绝跨域重定向到 %s", host)
func (c *Client) DownloadFile(ctx context.Context, attachmentID int64, dst string) error {
	// 1. 入口 URL 拼接：ssoBaseURL 域下 /common/attachment/getImg?id=X
	//    用 strconv.FormatInt 而非 fmt.Sprintf("%d") 避免 % 字符注入风险
	entryURL := c.ssoBaseURL + "/common/attachment/getImg?id=" + strconv.FormatInt(attachmentID, 10)

	// 2. 构造独立请求（不共享 c.http cookie jar）
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, entryURL, nil)
	if err != nil {
		return fmt.Errorf("%w: 创建下载请求失败: %w", ErrNetwork, err)
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", defaultUserAgent)

	// 3. 用 newCleanClient + CheckRedirect 校验同域 + 上限
	//    复用 newCleanClient 的 Transport 共享 + 无 cookie jar 特性，
	//    仅覆写 CheckRedirect 让 transport 自动跟随同域重定向。
	client := newCleanClient(c)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		// 上限校验：via 长度 = 已跟随次数，下次跟随时 via 增长 1
		if len(via) >= maxDownloadRedirects {
			return fmt.Errorf("重定向次数超过 %d 次", maxDownloadRedirects)
		}
		// 同域校验：上一跳 host → 下一跳 host 都必须在 nazhisoft.com 域
		if len(via) > 0 {
			last := via[len(via)-1].URL
			next := req.URL
			if !isSameTrustedHost(hostOf(last), hostOf(next)) {
				return fmt.Errorf("拒绝跨域重定向 %s → %s", hostOf(last), hostOf(next))
			}
		}
		return nil
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: 下载请求失败: %w", ErrNetwork, err)
	}
	defer drainAndClose(resp.Body)

	// 4. 状态码分类
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		sentinel := classifyHTTPStatus(resp.StatusCode, ErrInvalidResponse)
		return fmt.Errorf("%w: status=%d body=%s", sentinel, resp.StatusCode, logx.RedactBodyThenTruncate(errBody, 100))
	}

	// 5. 流式写入（ctx 感知：ctx 取消时立即中断，删除半成品）
	if err := writeDownloadToFile(ctx, resp.Body, dst); err != nil {
		return err
	}
	c.logDebugCtx(ctx, "DownloadFile 完成: id=%d → %s (host=%s)", attachmentID, dst, hostOf(resp.Request.URL))
	return nil
}

// maxDownloadRedirects 限制 DownloadFile 重定向跟随次数，防止恶意 Location 循环。
const maxDownloadRedirects = 5

// trustedHostSuffixes 是 DownloadFile 允许重定向到的 host 后缀白名单。
// 包级 var 让测试能临时覆盖（无需导出 API）。
// 生产默认白名单：nazhisoft.com 域（含 www/doc 子域）。
// 设计动机：恶意 Location 头跳转到 evil.com 会泄露 referer + 触发风控，
// 白名单把风险面收紧到平台自身子域。
var trustedHostSuffixes = []string{".nazhisoft.com", "nazhisoft.com"}

// hostOf 安全提取 URL 的 hostname（不含端口，nil 时返回空字符串）。
func hostOf(u *url.URL) string {
	if u == nil {
		return ""
	}
	return u.Hostname()
}

// isSameTrustedHost 判断两个 host 是否都在受信任域名集合内。
// 不接受 IP 字面量 host 匹配纯后缀规则——但 trustedHostSuffixes
// 可被测试覆盖为 "127.0.0.1" 等用于单元测试。
func isSameTrustedHost(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	for _, suffix := range trustedHostSuffixes {
		if hasHostSuffix(a, suffix) && hasHostSuffix(b, suffix) {
			return true
		}
	}
	return false
}

// hasHostSuffix 判断 host 是否受信：exact 匹配，或以 "."+suffix 结尾
// （suffix 自身以 "." 开头时直接以 suffix 结尾）。
// 禁止 evilnazhisoft.com 这种无点号前缀的后缀碰撞。
func hasHostSuffix(host, suffix string) bool {
	if host == "" || suffix == "" {
		return false
	}
	if host == suffix {
		return true
	}
	// suffix 已带前导点（如 .nazhisoft.com）：要求 host 以该后缀结尾即可
	if strings.HasPrefix(suffix, ".") {
		return strings.HasSuffix(host, suffix) && len(host) > len(suffix)
	}
	// suffix 无前导点（如 nazhisoft.com）：要求 exact 或以 "."+suffix 结尾
	return strings.HasSuffix(host, "."+suffix)
}

// writeDownloadToFile 把 src 流式写入 dst，ctx 取消可中断。
// 写入 0 字节或失败时关闭文件句柄后删除半成品（不留垃圾）。
// Windows 注意：必须先 f.Close() 再 os.Remove()——持有 open handle 时 Remove
// 在 Windows 上静默失败，测试会看到半成品残留。
func writeDownloadToFile(ctx context.Context, src io.Reader, dst string) error {
	f, err := osCreate(dst)
	if err != nil {
		// FILE-1：目标路径不可写/不存在是调用方输入问题，包 ErrInvalidPayload → 400/exit3。
		return fmt.Errorf("创建目标文件失败: %w", errors.Join(ErrInvalidPayload, err))
	}

	written, copyErr := copyCtx(ctx, src, f)
	// 显式 Close 在 osRemove 之前（Windows 文件句柄锁问题）
	closeErr := f.Close()
	if copyErr != nil {
		_ = osRemove(dst)
		// 中途传输失败（连接重置 / 意外 EOF）必须包 ErrNetwork 哨兵，
		// 让 SDK 调用方按 errors.Is(err, ErrNetwork) 判可重试；
		// 用户主动 ctx 取消不归类为网络故障，避免自动重试误触发。
		if errors.Is(copyErr, context.Canceled) || errors.Is(copyErr, context.DeadlineExceeded) {
			return fmt.Errorf("写入文件失败: %w", copyErr)
		}
		return fmt.Errorf("%w: 写入文件失败: %w", ErrNetwork, copyErr)
	}
	if closeErr != nil {
		_ = osRemove(dst)
		return fmt.Errorf("%w: 关闭目标文件失败: %w", ErrNetwork, closeErr)
	}
	if written == 0 {
		_ = osRemove(dst)
		return fmt.Errorf("%w: 服务端返回 0 字节", ErrNetwork)
	}
	return nil
}

// copyCtx 是 io.Copy 的 ctx 感知版本：ctx 取消时立刻终止复制。
func copyCtx(ctx context.Context, src io.Reader, dst io.Writer) (int64, error) {
	// 32KB buffer，与 io.Copy 默认一致
	buf := make([]byte, 32*1024)
	var written int64
	for {
		select {
		case <-ctx.Done():
			return written, ctx.Err()
		default:
		}
		n, rErr := src.Read(buf)
		if n > 0 {
			w, wErr := dst.Write(buf[:n])
			if wErr != nil {
				return written, wErr
			}
			written += int64(w)
			if w < n {
				return written, io.ErrShortWrite
			}
		}
		if rErr == io.EOF {
			return written, nil
		}
		if rErr != nil {
			return written, rErr
		}
	}
}

// osCreate / osRemove 单独函数封装便于测试 mock。
var (
	osCreate = func(dst string) (writeCloser, error) {
		return os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	}
	osRemove = os.Remove
)

// writeCloser 是 *os.File 的最小接口（Write + Close），便于测试。
type writeCloser interface {
	io.Writer
	Close() error
}

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
//
// 自定义 RoundTripper 隔离：非标准 *http.Transport 无法安全 Clone，文件通道回退到
// 无状态的 http.DefaultTransport，避免认证拦截器或其他状态把业务凭据带到公共上传域。
// WithHTTPClient 仍可控制业务请求；上传通道不继承其自定义 RoundTripper。
//
// 超时下限：上传/下载通道超时下限 30s（小于 30s 静默上浮并 WARN，防大文件被过短
// 超时截断；clean_client_cache_test 锁定）；主 Client 无超时时兜底 5 分钟，
// 有超时时上限沿用 c.http.Timeout。WithTimeout 注释不重复此细节，SDK 调用方
// 若需要更短超时预算请注意此下限。
func newCleanClient(c *Client) *http.Client {
	var transport http.RoundTripper
	switch t := c.http.Transport.(type) {
	case *http.Transport:
		// 每次现场 Clone，不缓存到 Client 字段。
		// Clone 成本 O(1) struct copy + 重置 idle conn pool，
		// 远低于一次 TLS 握手。运行时 Transport 变更即时感知。
		transport = t.Clone()
	default:
		// nil 或无法安全 Clone 的自定义 RoundTripper 均回退到无状态默认传输器。
		// 文件上传不能继承调用方的认证拦截器或状态，否则可能把业务凭据带到公共上传域。
		transport = http.DefaultTransport
	}
	timeout := c.http.Timeout
	if timeout == 0 {
		// 主 Client 无超时时，给文件上传一个合理兜底（5 分钟），
		// 避免大文件上传永久挂起。
		timeout = 5 * time.Minute
	} else if timeout < 30*time.Second {
		if c.logger != nil {
			c.logger.Warn("newCleanClient: 用户超时小于 30s，已覆盖为 30s（文件上传兜底）",
				"原超时", timeout, "覆盖后", 30*time.Second)
		}
		timeout = 30 * time.Second
	}
	return &http.Client{
		Transport:     transport,
		Timeout:       timeout,
		CheckRedirect: noRedirect,
	}
}
