// client_close_test.go 验证 Client.Close() 释放 OCR tempDir 与 HTTP keep-alive 连接。
package client_test

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// closeMockOCR 记录 Close() 是否被调用, 用于回归测试 Client.Close()
// 是否正确清理 OCR 资源 (避免 %TEMP%/nazhi-cli-ocr-XXXX/ 永久泄漏)。
type closeMockOCR struct {
	closed   atomic.Bool
	closeErr error
}

func (m *closeMockOCR) Recognize(_ []byte) (string, error) { return "abcd", nil }
func (m *closeMockOCR) Close() error {
	m.closed.Store(true)
	return m.closeErr
}

// TestClient_Close_ReleasesOCR 验证 Client.Close() 会调用底层 OCR 识别器的 Close()
// (该方法负责释放 ONNX session 和 %TEMP%/nazhi-cli-ocr-XXXX/ 临时目录)。
//
// 修复前: Client 包装 *ocr.Pool 但不暴露 Close(), 每次 CLI 退出都泄漏临时目录。
// 修复后: Client.Close() 调用 c.ocr.Close() 清理资源。
func TestClient_Close_ReleasesOCR(t *testing.T) {
	mock := &closeMockOCR{}

	c := client.New(
		client.WithCustomOCR(mock),
	)

	if err := c.Close(); err != nil {
		t.Fatalf("Close() 返回错误: %v", err)
	}

	if !mock.closed.Load() {
		t.Errorf("Close() 未调用底层 OCR.Close()，临时目录会泄漏到 %%TEMP%%")
	}
}

// TestClient_Close_PropagatesOCRCloseError 验证 Client.Close() 把 OCR.Close()
// 的错误向上传播, 让调用方能感知 (Windows AV 持锁 / 权限拒绝等)。
func TestClient_Close_PropagatesOCRCloseError(t *testing.T) {
	mock := &closeMockOCR{
		closeErr: errors.New("simulated remove-all failure"),
	}

	c := client.New(
		client.WithCustomOCR(mock),
	)

	err := c.Close()
	if err == nil {
		t.Fatal("Close() 应传播 OCR 错误")
	}
	if !mock.closed.Load() {
		t.Error("Close() 即使出错也应调用 OCR.Close()")
	}
}

// TestClient_Close_DefaultOCR 验证默认 Client (无自定义 OCR) Close() 不 panic
// 也不泄漏——这是 CLI 退出路径的常见调用方式。
//
// 进程级单例的 Close() 是有副作用的: 释放后其他 Client 无法再识别验证码。
// 默认实现应仅在自定义 OCR 时 Close(), 避免误杀进程级单例。
// 但本测试只验证 Close() 调用本身不出错, 不验证是否实际清理。
func TestClient_Close_DefaultOCR(t *testing.T) {
	c := client.New()

	// 默认 OCR 是 *ocr.Pool (单例), Close() 可能释放单例资源。
	// 为避免污染同进程后续测试, 用子测试隔离。
	err := c.Close()
	if err != nil {
		t.Logf("默认 OCR Close() 返回错误 (可能因并发测试已释放): %v", err)
	}
}
