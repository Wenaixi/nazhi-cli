package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// UploadFile 上传图片到文件服务器，返回图片 ID。
// 不需要 Token，文件服务器独立。
func (c *Client) UploadFile(ctx context.Context, filePath string) (int64, error) {
	// 1. 检查文件是否存在且合法
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return 0, fmt.Errorf("文件不存在: %w", err)
	}

	// 检查文件大小（最大 5MB）
	const maxSize = 5 * 1024 * 1024
	if fileInfo.Size() > maxSize {
		return 0, fmt.Errorf("%w: 文件 %s 大小 %d 超出限制 %d", ErrFileTooLarge, filePath, fileInfo.Size(), maxSize)
	}

	// 2. 读取文件
	file, err := os.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	// 3. 构造 multipart 请求体
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// 添加 file 字段
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return 0, fmt.Errorf("创建 multipart form 失败: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return 0, fmt.Errorf("写入文件到 multipart 失败: %w", err)
	}
	writer.Close()

	// 4. 发送请求
	uploadURL := c.uploadServiceURL("/common/upload/uploadImage?bussinessType=12&groupName=other")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &buf)
	if err != nil {
		return 0, fmt.Errorf("创建上传请求失败: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	// 文件上传服务器独立，没有统一的认证头
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("%w: 上传请求失败: %w", ErrNetwork, err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

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

	// id 可能是 float64（JSON 数字）或 string
	id, ok := result["id"].(float64)
	if !ok {
		return 0, fmt.Errorf("%w: returnData 中缺少 id 字段", ErrUploadRejected)
	}

	return int64(id), nil
}
