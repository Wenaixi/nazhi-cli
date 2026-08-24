// scan.go 病毒扫描注入点：UploadFile 在发出任何网络请求前对完整上传字节
// 做 ClamAV clamd INSTREAM 扫描。fail-closed——扫描出错视为不干净。
package client

import (
	"context"
	"errors"
	"fmt"

	"github.com/PyYoshi/go-clamav"
)

// 错误哨兵：调用方按类型分流处理。
var (
	// ErrVirusDetected 扫描检出恶意文件。上传被拒绝，字节不应触达平台。
	ErrVirusDetected = errors.New("virus detected by antivirus scan")
	// ErrScanUnavailable 扫描器不可用或扫描失败（连接断开、超时等）。
	// fail-closed 语义下同样拒绝上传；调用方应检查扫描服务健康状态。
	ErrScanUnavailable = errors.New("antivirus scan unavailable or failed")
)

// UploadScanner 是病毒扫描的最小抽象：对完整字节流返回判定或错误。
//
// SDK 不绑定具体实现——生产用 WithClamavScanner 注入 go-clamav 客户端，
// 测试注入内存 fake。错误一律视为「判定未知」，调用方必须拒绝。
type UploadScanner interface {
	ScanUpload(ctx context.Context, data []byte) error
}

// clamdScanner 把 go-clamav 的 ScanStream 适配为 UploadScanner。
type clamdScanner struct {
	client *clamav.Client
}

// ScanUpload 送检：INSTREAM 协议把字节切块发给 clamd，无临时文件。
// 判定映射：Infected → ErrVirusDetected；Clean → nil；
// 其余任何错误（连接失败、超时、超限）→ ErrScanUnavailable（fail-closed）。
func (s *clamdScanner) ScanUpload(ctx context.Context, data []byte) error {
	res, err := s.client.ScanBytes(ctx, data)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrScanUnavailable, err)
	}
	switch {
	case res.Infected():
		return fmt.Errorf("%w: %s", ErrVirusDetected, res.Signature)
	case res.Clean():
		return nil
	default:
		// VerdictUnknown 等其他终态：不能证明干净，拒绝。
		return fmt.Errorf("%w: verdict=%v", ErrScanUnavailable, res.Verdict)
	}
}

// WithClamavScanner 注入基于 clamd 的病毒扫描器。
//
// addr 支持 clamd 标准形式："tcp://host:port" 或 "unix:///path/to/clamd.sock"。
// opts 直接透传给 go-clamav 客户端构造（如 WithMaxStreamSize / WithTimeout）。
//
// 构造失败时 warn 并保持无扫描器状态（与 WithCustomOCR 的守卫模式一致）：
// 扫描器是可选加固项，部署环境没有 clamd 时上传链路仍可用。
// 注入成功后，UploadFile 对每次上传先扫描再发请求。
func WithClamavScanner(addr string, opts ...clamav.Option) Option {
	return func(c *Client) {
		if addr == "" {
			c.logger.Warn("WithClamavScanner: 空 addr 被拒绝，保持当前值")
			return
		}
		client, err := clamav.New(addr, opts...)
		if err != nil {
			c.logger.Warn("WithClamavScanner: clamd 客户端初始化失败，未启用病毒扫描",
				"addr", addr, "err", err.Error())
			return
		}
		c.uploadScanner = &clamdScanner{client: client}
	}
}

// WithUploadScanner 注入自定义扫描实现（测试用内存 fake 走这里）。
var WithUploadScanner = withNilGuard[UploadScanner]("WithUploadScanner", func(c *Client, v UploadScanner) {
	c.uploadScanner = v
})
