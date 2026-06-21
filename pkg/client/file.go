package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// UploadFile 上传图片到文件服务器，返回图片 ID。
// 不需要 Token，文件服务器独立。
//
// 上传前自动预处理（对齐 v1 utils/image_convert.py）：
//  1. 任意格式 → JPG（PNG/GIF/WEBP 等）
//  2. 透明通道 → 合成白底
//  3. 压缩至 ≤ 5MB（质量级联 + 等比缩放）
//  4. 临时数据全部在内存中，不写盘、不修改原文件
func (c *Client) UploadFile(ctx context.Context, filePath string) (int64, error) {
	// 1. 预处理图片：任意格式 → JPG，压缩至 ≤ 1MB
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

	// 3. 发送请求
	uploadURL := c.uploadServiceURL("/common/upload/uploadImage?bussinessType=12&groupName=other")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &buf)
	if err != nil {
		return 0, fmt.Errorf("创建上传请求失败: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	// 文件上传服务器独立，没有统一的认证头
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("%w: 上传请求失败: %w", ErrNetwork, err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("%w: status=%d body=%s", ErrUploadRejected, resp.StatusCode, string(bodyBytes))
	}

	// 4. 解析响应
	var unified types.UnifiedResponse
	if err := json.Unmarshal(bodyBytes, &unified); err != nil {
		return 0, fmt.Errorf("解析上传响应失败: %w", err)
	}

	if unified.Code != 1 {
		return 0, fmt.Errorf("%w: code=%d", ErrUploadRejected, unified.Code)
	}

	// 5. 从 returnData 提取 id
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
