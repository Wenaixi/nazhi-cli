package main

import (
	"errors"
	"io"
	"sync"
	"testing"
)

// countingCloser 记录 Close 被调用次数，用于验证「同一资源不被重复关闭」。
type countingCloser struct {
	mu    sync.Mutex
	count int
	err   error
}

func (c *countingCloser) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count++
	return c.err
}

func (c *countingCloser) calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count
}

// TestProcessScope_CloseLogFiles_ClosesEachFileOnce 锁定资源所有权的不变量：
// 同一资源在同一 Scope 中只应被关闭一次。
//
// 背景（2026-09-26 架构核实）：ProcessScope 引入前，lifecycle.go 的
// trackClient/trackLogFile 同时写入 Scope 与 legacy 两张包级表，
// 导致 closeAllClients 必须用 seen map 去重，否则同一 Client 被 Close 两次。
// 去重是当时唯一的防线，却没有任何测试覆盖。
//
// 本测试直接锁定「一次登记、一次关闭」，让重复登记不可能悄悄发生。
func TestProcessScope_CloseLogFiles_ClosesEachFileOnce(t *testing.T) {
	s := NewProcessScope()
	f := &countingCloser{}

	s.TrackLogFile(f)
	if err := s.CloseLogFiles(); err != nil {
		t.Fatalf("CloseLogFiles 失败: %v", err)
	}
	if got := f.calls(); got != 1 {
		t.Errorf("同一日志文件应只关闭 1 次，实际 %d 次", got)
	}
}

// TestProcessScope_CloseLogFiles_DuplicateRegistrationStillClosesOnce 锁定
// 重复登记同一资源时不得重复关闭——这正是双写时期 seen map 的职责。
// 去重后该逻辑应由 Scope 自身承担，而不是靠调用方拼两张表。
func TestProcessScope_CloseLogFiles_DuplicateRegistrationStillClosesOnce(t *testing.T) {
	s := NewProcessScope()
	f := &countingCloser{}

	s.TrackLogFile(f)
	s.TrackLogFile(f) // 重复登记

	if err := s.CloseLogFiles(); err != nil {
		t.Fatalf("CloseLogFiles 失败: %v", err)
	}
	if got := f.calls(); got != 1 {
		t.Errorf("重复登记同一资源后应只关闭 1 次，实际 %d 次", got)
	}
}

// TestProcessScope_CloseLogFiles_IsIdempotent 锁定二次关闭是 no-op：
// closeAllClients 内部会把列表置 nil，main 的 defer 与 os.Exit 前
// 可能各调用一次清理，二次调用必须无副作用。
func TestProcessScope_CloseLogFiles_IsIdempotent(t *testing.T) {
	s := NewProcessScope()
	f := &countingCloser{}
	s.TrackLogFile(f)

	if err := s.CloseLogFiles(); err != nil {
		t.Fatalf("首次 CloseLogFiles 失败: %v", err)
	}
	if err := s.CloseLogFiles(); err != nil {
		t.Fatalf("二次 CloseLogFiles 失败: %v", err)
	}
	if got := f.calls(); got != 1 {
		t.Errorf("二次关闭后累计应仍为 1 次，实际 %d 次", got)
	}
}

// TestProcessScope_CloseLogFiles_LIFOOrder 锁定关闭顺序为后进先出：
// 与 Go 资源栈语义一致，最后打开的日志文件最先关闭。
func TestProcessScope_CloseLogFiles_LIFOOrder(t *testing.T) {
	s := NewProcessScope()
	var order []int
	closers := make([]*orderedCloser, 3)
	for i := range closers {
		closers[i] = &orderedCloser{id: i + 1, order: &order}
	}
	for _, c := range closers {
		s.TrackLogFile(c)
	}

	if err := s.CloseLogFiles(); err != nil {
		t.Fatalf("CloseLogFiles 失败: %v", err)
	}

	want := []int{3, 2, 1}
	if len(order) != len(want) {
		t.Fatalf("期望关闭 %d 个资源，实际 %d", len(want), len(order))
	}
	for i, id := range want {
		if order[i] != id {
			t.Errorf("关闭顺序错位：期望 %v，实际 %v", want, order)
			break
		}
	}
}

// TestProcessScope_CloseLogFiles_AggregatesErrors 锁定错误聚合：
// 多个资源关闭失败时，返回值必须仍能让调用方感知（errors.Join）。
func TestProcessScope_CloseLogFiles_AggregatesErrors(t *testing.T) {
	s := NewProcessScope()
	boom1 := &countingCloser{err: errors.New("第一个失败")}
	boom2 := &countingCloser{err: errors.New("第二个失败")}
	ok := &countingCloser{}

	s.TrackLogFile(boom1)
	s.TrackLogFile(boom2)
	s.TrackLogFile(ok)

	err := s.CloseLogFiles()
	if err == nil {
		t.Fatal("存在失败资源时 CloseLogFiles 应返回错误")
	}
	// 即使有失败，其余资源也必须被尝试关闭
	if ok.calls() != 1 {
		t.Errorf("前序资源失败不应阻断后续关闭，实际调用 %d 次", ok.calls())
	}
}

// orderedCloser 记录自身被关闭的顺序。
type orderedCloser struct {
	id    int
	order *[]int
}

func (c *orderedCloser) Close() error {
	*c.order = append(*c.order, c.id)
	return nil
}

var (
	_ io.Closer = (*countingCloser)(nil)
	_ io.Closer = (*orderedCloser)(nil)
)
