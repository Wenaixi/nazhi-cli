package client

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// fakeReadCloser 模拟 http.Response.Body：
// - Read() 从 data 读取
// - Close() 增加计数器
// - drained 用于检测 drainAndClose 真的读了所有数据
type fakeReadCloser struct {
	data     io.Reader
	closeCnt int
	drained  bool // 记录是否被 io.Copy 完整读到底
}

func (f *fakeReadCloser) Read(p []byte) (int, error) {
	n, err := f.data.Read(p)
	if err == io.EOF {
		f.drained = true
	}
	return n, err
}

func (f *fakeReadCloser) Close() error {
	f.closeCnt++
	return nil
}

// 真实场景测试：drainAndClose 必须先 drain body 再 Close()，
// 让 net/http 把连接归还 keep-alive 池（F1 修复的同类 bug 防御）。
func TestDrainAndClose_DrainsBeforeClose(t *testing.T) {
	body := &fakeReadCloser{data: strings.NewReader("hello world")}
	drainAndClose(body)

	if body.closeCnt != 1 {
		t.Errorf("Close 应被调用 1 次，实际 %d", body.closeCnt)
	}
	if !body.drained {
		t.Error("drainAndClose 应完整读完 body 后再 Close（drained=true）")
	}
}

// nil 安全：某些边缘路径（如响应 body 为 nil）不应 panic。
func TestDrainAndClose_NilSafe(t *testing.T) {
	drainAndClose(nil) // 不应 panic
}

// http.NoBody 等 ReadCloser 实现也必须正确 drain+close。
func TestDrainAndClose_NoBody(t *testing.T) {
	body := http.NoBody
	drainAndClose(body) // http.NoBody 的 Close 返回 nil，drain 是空操作
}
