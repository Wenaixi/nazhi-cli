// Package envelope 提供 CLI 统一响应 envelope。
//
// 设计动机：CLI 所有命令输出统一 JSON envelope，方便脚本 parse。
//   - Status 字段：success / partial / error 三态
//   - Code 字段：HTTP 风格状态码（200/4xx/5xx）
//   - Message 字段：人类可读提示
//   - Data 字段：业务负载（任意类型）
//
// 退出码三分契约（见 ExitCode 方法）：
//   - 0: 成功
//   - 1: partial / 业务错误 (4xx 非 400)
//   - 2: 网络/服务端错误 (5xx)
//   - 3: 参数错误 (400)
package envelope

// Status 是 envelope 的状态字段。
type Status string

const (
	// StatusSuccess 表示完全成功。
	StatusSuccess Status = "success"
	// StatusPartial 表示部分成功（有数据但也有错误）。
	StatusPartial Status = "partial"
	// StatusError 表示完全失败。
	StatusError Status = "error"
)

// Envelope 是 CLI 统一的响应结构。
type Envelope struct {
	Status  Status `json:"status"`
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// ExitCode 返回 CLI 退出码。
//
// 三分契约：
//   - 0  成功（status=success）
//   - 1  部分成功 或 业务错误 (4xx 非 400)
//   - 2  网络/服务端错误 (5xx)
//   - 3  参数错误 (400)
func (e *Envelope) ExitCode() int {
	switch e.Status {
	case StatusSuccess:
		return 0
	case StatusPartial:
		return 1
	case StatusError:
		switch {
		case e.Code == 400:
			return 3
		case e.Code >= 500:
			return 2
		case e.Code >= 400 && e.Code < 500:
			return 1
		}
	}
	// 兜底：未知状态视为失败
	return 1
}

// Success 构造成功 envelope。
func Success(data any) *Envelope {
	return &Envelope{Status: StatusSuccess, Code: 200, Data: data}
}

// Empty 构造空数据 envelope（HTTP 204 风格）。
func Empty(msg string) *Envelope {
	return &Envelope{Status: StatusSuccess, Code: 204, Message: msg, Data: nil}
}

// Partial 构造部分成功 envelope。
func Partial(code int, msg string, data any) *Envelope {
	return &Envelope{Status: StatusPartial, Code: code, Message: msg, Data: data}
}

// Error 构造错误 envelope。
func Error(code int, msg string) *Envelope {
	return &Envelope{Status: StatusError, Code: code, Message: msg, Data: nil}
}
