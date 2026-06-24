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
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filePath+".jpg")
	if err != nil {
		return 0, fmt.Errorf("创建 multipart form 失败: %w", err)
	}
	if _, err := part.Write(fileData); err != nil {
		return 0, fmt.Errorf("写入图片到 multipart 失败: %w", err)
	}
	writer.Close()

	// 3. 构造请求
	uploadURL := c.uploadServiceURL("/common/upload/uploadImage?bussinessType=12&groupName=other")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &buf)
	if err != nil {
		return 0, fmt.Errorf("创建上传请求失败: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36")

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
	defer func() {
		// 关键：先 drain body 再 close，让 net/http 把连接归还 keep-alive 池
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		// 关键修复: 之前用 `bodyBytes, _ := io.ReadAll(resp.Body)` 吞噬错误,
		// 当服务端断网 (connection reset / unexpected EOF) 时 bodyBytes 为空,
		// 后续 json.Unmarshal([]) 返回 EOF, 报「解析上传响应失败: EOF」丢失根因。
		// 现在包装为 ErrNetwork 哨兵, 上层可 errors.Is 识别。
		return 0, fmt.Errorf("%w: 读取上传响应体失败: %w", ErrNetwork, err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("%w: status=%d body=%s", ErrUploadRejected, resp.StatusCode, string(bodyBytes))
	}

	// 5. 解析响应
	var unified types.UnifiedResponse
	if err := json.Unmarshal(bodyBytes, &unified); err != nil {
		return 0, fmt.Errorf("解析上传响应失败: %w", err)
	}

	if unified.Code != 1 {
		return 0, fmt.Errorf("%w: code=%d", ErrUploadRejected, unified.Code)
	}

	// 6. 从 returnData 提取 id
	if unified.ReturnData == nil {
		return 0, fmt.Errorf("%w: 响应中缺少 returnData", ErrUploadRejected)
	}

	var result map[string]any
	if err := json.Unmarshal(*unified.ReturnData, &result); err != nil {
		return 0, fmt.Errorf("解析 returnData 失败: %w", err)
	}

	id, ok := result["id"].(float64)
	if !ok {
		return 0, fmt.Errorf("%w: returnData 中缺少 id 字段", ErrUploadRejected)
	}

	return int64(id), nil
}

// newCleanClient 构造"无 cookie"的安全 http.Client 供 UploadFile 使用。
//
// 安全保证：独立 http.Client（不共享 c.http.Jar），不发送任何 Cookie /
// Authorization 头，杜绝业务域鉴权信息泄露到文件上传公共服务。
//
// 性能优化：共享 c.http.Transport（连接池、TLS 配置、代理、Dialer），
// 批量上传 50 张图时仅需 1 次 DNS+TCP+TLS 握手，后续 keep-alive 复用。
//
// 同时禁用自动重定向（与 SSO 流程策略一致），防止 302 跳转到第三方主机
// 时附带请求头。
func newCleanClient(c *Client) *http.Client {
	transport := c.http.Transport
	if transport == nil {
		transport = http.DefaultTransport
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
