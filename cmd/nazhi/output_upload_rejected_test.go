package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// TestMapSentinelToHTTPCode_UploadRejected 回归：ErrUploadRejected 必须映射为
// 422（业务拒绝档 exit1）而非默认 500（exit2）。上传被服务端拒绝（文件类型不收、
// 风控拦截）是「请求本身的问题」，脚本需要据此换文件重传；归入 5xx「服务端故障」
// 档会让脚本误判为可重试的瞬时故障。
func TestMapSentinelToHTTPCode_UploadRejected(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "裸哨兵",
			err:  client.ErrUploadRejected,
			want: 422,
		},
		{
			name: "包装链中含哨兵",
			err:  fmt.Errorf("上传文件失败: %w", fmt.Errorf("%w: code=0", client.ErrUploadRejected)),
			want: 422,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mapSentinelToHTTPCode(tt.err); got != tt.want {
				t.Errorf("mapSentinelToHTTPCode(ErrUploadRejected) = %d, 期望 %d", got, tt.want)
			}
		})
	}
	// 兜底确认：errors.Is 穿透包装链
	if !errors.Is(tests[1].err, client.ErrUploadRejected) {
		t.Fatal("测试夹具错误：包装链应可被 errors.Is 命中")
	}
}
