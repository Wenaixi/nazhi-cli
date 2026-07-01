package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"testing"
)

// timeoutError 是一个模拟超时的 error，实现 Timeout() bool 接口。
// url.Error.Timeout() 和 net.OpError.Timeout() 通过类型断言检查 Err 字段
// 是否实现 Timeout() bool，因此本类型可以让 url.Error/nested OpError 也被
// isTimeoutError 识别为超时错误。
type timeoutError struct{}

func (timeoutError) Error() string { return "i/o timeout" }
func (timeoutError) Timeout() bool { return true }

// ─── ClassifyError 基本分类测试 ───

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want ErrorCategory
	}{
		{"context.Canceled", context.Canceled, CategoryContextCancel},
		{"wrapped context.Canceled", fmt.Errorf("wrap: %w", context.Canceled), CategoryContextCancel},
		{"context.DeadlineExceeded", context.DeadlineExceeded, CategoryContextTimeout},
		{"wrapped DeadlineExceeded", fmt.Errorf("wrap: %w", context.DeadlineExceeded), CategoryContextTimeout},
		{"nil error", nil, CategoryUnknown},
		{"other error", errors.New("some error"), CategoryUnknown},
		{"plain ErrBusinessRejected", ErrBusinessRejected, CategoryBusinessError},
		{"wrapped ErrBusinessRejected", fmt.Errorf("op: %w", ErrBusinessRejected), CategoryBusinessError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyError(tt.err)
			if got != tt.want {
				t.Errorf("ClassifyError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// ─── 网络超时分类测试 ───

func TestClassifyError_NetworkTimeout(t *testing.T) {
	t.Run("url.Error with timeout underlying", func(t *testing.T) {
		urlErr := &url.Error{Op: "Get", URL: "http://example.com", Err: timeoutError{}}
		if got := ClassifyError(urlErr); got != CategoryNetworkTimeout {
			t.Errorf("ClassifyError(url.Error with timeout) = %v, want NetworkTimeout", got)
		}
	})

	t.Run("net.OpError with timeout underlying", func(t *testing.T) {
		netErr := &net.OpError{Op: "dial", Net: "tcp", Err: timeoutError{}}
		if got := ClassifyError(netErr); got != CategoryNetworkTimeout {
			t.Errorf("ClassifyError(net.OpError with timeout) = %v, want NetworkTimeout", got)
		}
	})

	t.Run("url.Error non-timeout", func(t *testing.T) {
		urlErr := &url.Error{Op: "Get", URL: "http://example.com", Err: errors.New("connection refused")}
		if got := ClassifyError(urlErr); got == CategoryNetworkTimeout {
			t.Error("non-timeout url.Error should not be NetworkTimeout")
		}
	})
}

// ─── 分类优先级测试 ───

func TestClassifyError_Priority(t *testing.T) {
	t.Run("context.Canceled outranks network timeout", func(t *testing.T) {
		// url.Error wrapping context.Canceled:
		//   errors.Is(urlErr, context.Canceled) 应先于 urlErr.Timeout() 检查
		urlErr := &url.Error{
			Op:  "Get",
			URL: "http://example.com",
			Err: fmt.Errorf("wrapped: %w", context.Canceled),
		}
		if got := ClassifyError(urlErr); got != CategoryContextCancel {
			t.Errorf("期望 ContextCancel，得到 %v", got)
		}
	})

	t.Run("context.DeadlineExceeded outranks network timeout", func(t *testing.T) {
		urlErr := &url.Error{
			Op:  "Get",
			URL: "http://example.com",
			Err: fmt.Errorf("wrapped: %w", context.DeadlineExceeded),
		}
		if got := ClassifyError(urlErr); got != CategoryContextTimeout {
			t.Errorf("期望 ContextTimeout，得到 %v", got)
		}
	})
}
