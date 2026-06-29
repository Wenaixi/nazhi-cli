// Package ocr 内部白盒测试：OCR 启动时清扫遗留的临时目录。
//
// 背景：每次 OCR 初始化会在 %TEMP% 下用 os.MkdirTemp("", "nazhi-cli-ocr-*")
// 建一个唯一新目录。Windows 上 onnxruntime.dll 被 CGO LoadLibrary 占用，
// Close 时本进程的目录删不掉；进程退出后句柄释放，旧目录才能被下次进程清掉。
//
// 业界惯例（Chrome/VSCode 等）：在新建临时目录时顺手 best-effort 清扫
// 之前进程遗留的同前缀目录。这层清扫是「锦上添花」——删不掉的静默跳过，
// 不会影响主流程。
//
// 测试策略：通过包级变量注入 tempDirFn / readDirFn / removeDirFn，
// 模拟「%TEMP% 下的目录列表」与「删除行为」，
// 不依赖真实文件系统，保证跨 OS runner 行为一致。
package ocr

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// fakeDirEntry 构造一个最小可用的 fs.DirEntry 用于 readDirFn mock。
// 真实 os.ReadDir 返回 *os.DirEntry 类型的 fs.DirEntry 实现；
// 这里用 fs.DirEntry 仅作类型断言满足 os.ReadDir 的返回值签名。
type fakeDirEntry struct {
	name  string
	isDir bool
}

func (f fakeDirEntry) Name() string { return f.name }
func (f fakeDirEntry) IsDir() bool  { return f.isDir }
func (f fakeDirEntry) Type() os.FileMode {
	if f.isDir {
		return os.ModeDir
	}
	return 0
}
func (f fakeDirEntry) Info() (os.FileInfo, error) { return nil, errors.New("not implemented") }

// toReadDirResult 把 []fakeDirEntry 转成 []os.DirEntry 切片，
// 供 readDirFn mock 返回值用。
func toReadDirResult(t *testing.T, entries []fakeDirEntry) []os.DirEntry {
	t.Helper()
	out := make([]os.DirEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, e)
	}
	return out
}

// withSweepMocks 一次性注入 tempDirFn / readDirFn / removeDirFn 的 mock，
// 返回 restore 函数让测试结束时还原原始函数。
func withSweepMocks(t *testing.T, tempDir string, entries []fakeDirEntry, remove func(string) error) func() {
	t.Helper()
	origTemp := tempDirFn
	origRead := readDirFn
	origRm := removeDirFn
	tempDirFn = func() string { return tempDir }
	readDirFn = func(_ string) ([]os.DirEntry, error) { return toReadDirResult(t, entries), nil }
	removeDirFn = remove
	return func() {
		tempDirFn = origTemp
		readDirFn = origRead
		removeDirFn = origRm
	}
}

// TestSweepStaleTempDirs_SkipsCurrentAndRemovesOthers 场景 (a)：
// %TEMP% 下有多个 nazhi-cli-ocr-* 目录，包含「本次新建的」和「上次遗留的」，
// 清扫必须只删非当前的、保留当前目录。
func TestSweepStaleTempDirs_SkipsCurrentAndRemovesOthers(t *testing.T) {
	currentDir := filepath.Join(os.TempDir(), "nazhi-cli-ocr-current")
	stale1 := filepath.Join(os.TempDir(), "nazhi-cli-ocr-stale1")
	stale2 := filepath.Join(os.TempDir(), "nazhi-cli-ocr-stale2")

	var removed []string
	restore := withSweepMocks(t, os.TempDir(), []fakeDirEntry{
		{name: "nazhi-cli-ocr-current", isDir: true},
		{name: "nazhi-cli-ocr-stale1", isDir: true},
		{name: "nazhi-cli-ocr-stale2", isDir: true},
	}, func(p string) error {
		removed = append(removed, p)
		return nil
	})
	defer restore()

	if err := sweepStaleTempDirs(currentDir); err != nil {
		t.Fatalf("sweepStaleTempDirs 应静默成功，实际返回 %v", err)
	}
	sort.Strings(removed)
	want := []string{stale1, stale2}
	if len(removed) != len(want) {
		t.Fatalf("删除数量不对：got %v want %v", removed, want)
	}
	for i := range want {
		if removed[i] != want[i] {
			t.Errorf("删除列表第 %d 项：got %q want %q", i, removed[i], want[i])
		}
	}
}

// TestSweepStaleTempDirs_NeverTouchesForeignEntries 场景 (b)：
// %TEMP% 混入「名字以 nazhi-cli-ocr- 开头但不是本进程创建」之外的东西
// （其它程序目录、文件、其它前缀），必须一律不碰，防误删。
func TestSweepStaleTempDirs_NeverTouchesForeignEntries(t *testing.T) {
	currentDir := filepath.Join(os.TempDir(), "nazhi-cli-ocr-current")
	stale := filepath.Join(os.TempDir(), "nazhi-cli-ocr-stale")

	var removed []string
	restore := withSweepMocks(t, os.TempDir(), []fakeDirEntry{
		{name: "nazhi-cli-ocr-current", isDir: true},
		{name: "nazhi-cli-ocr-stale", isDir: true},
		// 同前缀但文件名更复杂（仍属于本工具域，必须删）
		{name: "nazhi-cli-ocr-20240101-xyz", isDir: true},
		// 完全无关：其它程序、其它前缀、文件而非目录
		{name: "chrome-temp", isDir: true},
		{name: "vscode-update", isDir: true},
		{name: "go-build-12345", isDir: true},
		{name: "random.txt", isDir: false},
		{name: "log", isDir: false},
	}, func(p string) error {
		removed = append(removed, p)
		return nil
	})
	defer restore()

	if err := sweepStaleTempDirs(currentDir); err != nil {
		t.Fatalf("sweepStaleTempDirs 应静默成功，实际返回 %v", err)
	}
	sort.Strings(removed)
	want := []string{
		filepath.Join(os.TempDir(), "nazhi-cli-ocr-20240101-xyz"),
		stale,
	}
	sort.Strings(want)
	if len(removed) != len(want) {
		t.Fatalf("删除数量不对：got %v want %v（绝不能误删 chrome/vscode/go-build/random.txt/log）", removed, want)
	}
	for i := range want {
		if removed[i] != want[i] {
			t.Errorf("删除列表第 %d 项：got %q want %q", i, removed[i], want[i])
		}
	}
	// 二次断言（更严格）：结果中绝不能出现非 nazhi-cli-ocr- 前缀
	for _, p := range removed {
		base := filepath.Base(p)
		if !strings.HasPrefix(base, "nazhi-cli-ocr-") {
			t.Errorf("误删非本工具前缀目录：%q", p)
		}
	}
}

// TestSweepStaleTempDirs_RemoveFailureDoesNotAbort 场景 (c)：
// 某个 stale 目录删除失败（mock 返回 error），不能 panic、中断、整体报错；
// 必须继续尝试删其它 stale 目录，并整体不返回致命错误。
func TestSweepStaleTempDirs_RemoveFailureDoesNotAbort(t *testing.T) {
	currentDir := filepath.Join(os.TempDir(), "nazhi-cli-ocr-current")
	stale1 := filepath.Join(os.TempDir(), "nazhi-cli-ocr-stale1")
	stale2 := filepath.Join(os.TempDir(), "nazhi-cli-ocr-stale2")
	stale3 := filepath.Join(os.TempDir(), "nazhi-cli-ocr-stale3")

	var removed []string
	restore := withSweepMocks(t, os.TempDir(), []fakeDirEntry{
		{name: "nazhi-cli-ocr-current", isDir: true},
		{name: "nazhi-cli-ocr-stale1", isDir: true},
		{name: "nazhi-cli-ocr-stale2", isDir: true},
		{name: "nazhi-cli-ocr-stale3", isDir: true},
	}, func(p string) error {
		removed = append(removed, p)
		// stale2 模拟「正在被其他实例占用，删不掉」——必须被静默跳过
		if p == stale2 {
			return errors.New("sharing violation")
		}
		return nil
	})
	defer restore()

	if err := sweepStaleTempDirs(currentDir); err != nil {
		t.Fatalf("单个 stale 删失败不应冒泡为致命错误，实际：%v", err)
	}
	sort.Strings(removed)
	want := []string{stale1, stale2, stale3}
	if len(removed) != len(want) {
		t.Fatalf("即使 stale2 删失败，其它 stale 也必须被尝试：got %v want %v", removed, want)
	}
}

// TestSweepStaleTempDirs_ReadDirFailure_ReturnsSilently 场景 (d)：
// ReadDir 本身失败（权限拒绝、临时目录不存在等），sweep 必须静默返回，
// 不影响调用方主流程。
func TestSweepStaleTempDirs_ReadDirFailure_ReturnsSilently(t *testing.T) {
	origTemp := tempDirFn
	origRead := readDirFn
	origRm := removeDirFn
	defer func() {
		tempDirFn = origTemp
		readDirFn = origRead
		removeDirFn = origRm
	}()
	tempDirFn = func() string { return "/no/such/dir" }
	readDirFn = func(_ string) ([]os.DirEntry, error) {
		return nil, errors.New("access denied")
	}
	// removeDirFn 不应被调用
	called := false
	removeDirFn = func(p string) error {
		called = true
		return nil
	}

	if err := sweepStaleTempDirs("/whatever/current"); err != nil {
		t.Fatalf("ReadDir 失败时应静默返回 nil，实际：%v", err)
	}
	if called {
		t.Fatal("ReadDir 失败时不应尝试任何删除")
	}
}

// TestExtractModels_CallsSweepAfterMkdirTemp 集成断言：extractModels 在
// os.MkdirTemp 成功建出当前目录后，必须调用 sweepStaleTempDirs 并把
// 刚建的 dir 传进去作为 currentDir。
//
// 为什么必须有这个测试：4 个 sweepStaleTempDirs 单元测试只覆盖了 helper 自身的
// 4 种行为（场景 a/b/c/d），但如果忘记在 extractModels 末尾接入 sweep，整套机制
// 形同虚设——helper 不会因测试失败而自动接入主流程。这里拦截 sweep 调用，断言
// 调用发生过且 currentDir 正确。
func TestExtractModels_CallsSweepAfterMkdirTemp(t *testing.T) {
	var sweepCalledWith string
	var sweepCallCount int
	origSweep := sweepFn
	defer func() { sweepFn = origSweep }()
	sweepFn = func(currentDir string) error {
		sweepCalledWith = currentDir
		sweepCallCount++
		return nil
	}

	o := &OCR{}
	dir, err := o.extractModels()
	if err != nil {
		t.Fatalf("extractModels 失败：%v", err)
	}
	defer os.RemoveAll(dir) // 测试收尾：清掉本次 extractModels 真的建出的目录

	if sweepCallCount != 1 {
		t.Fatalf("extractModels 成功后应调用 sweepFn 恰好 1 次，实际 %d 次", sweepCallCount)
	}
	if sweepCalledWith != dir {
		t.Errorf("sweepFn 应传入本次新建的 dir 作 currentDir：got %q want %q", sweepCalledWith, dir)
	}
}
