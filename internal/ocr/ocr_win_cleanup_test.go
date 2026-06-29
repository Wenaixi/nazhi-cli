// Package ocr 内部白盒测试：Windows 上 onnxruntime.dll 被进程内 LoadLibrary
// 持锁的清理降级。
//
// 历史 bug：在 Windows 上执行 nazhi login（带 -tags=ddddocr 构建）后，
// Pool.Close → OCR.Close 会按顺序 o.ocr.Close() + os.RemoveAll(tempDir)。
// 当 OnnxRuntime DLL 仍被 CGO LoadLibrary 持锁时，Windows 在进程退出前
// 不会释放该 DLL 的 mmap 文件句柄，os.RemoveAll 删到 onnxruntime.dll 时
// 返回 ERROR_ACCESS_DENIED(5) / ERROR_SHARING_VIOLATION(32)，
// Close() 把这类 OS 级「被占用」错误并入返回值，污染 stderr。
//
// 修复：把「删除临时目录」抽成 cleanupTempDir helper，
// 对 OS 级「文件被占用」类错误降级（返回 nil），其他错误照常上报。
// 「不静默吞错」铁律保留：仅 DLL/原生库持锁导致的 errno 才降级，
// 其他 errno（权限拒绝、磁盘满等）照常返回。
//
// 测试用 removeDirFn 函数变量注入删除行为，避免依赖真实 Windows 持锁。
package ocr

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 注意：errnoAccessDeniedWin / errnoSharingViolationWin 常量定义在 ocr.go
// （生产代码与测试共享同一包，直接引用即可，避免重复声明）。

// TestCleanupTempDir_BusyDLL_DowngradesToNil Windows 上 onnxruntime DLL
// 被进程持锁的场景：os.RemoveAll 内的 syscall 会返回 ERROR_ACCESS_DENIED
// 或 ERROR_SHARING_VIOLATION。helper 必须识别这类「OS 级文件占用」错误
// 并降级为 nil（不让 Close() 返回污染 stderr）。
func TestCleanupTempDir_BusyDLL_DowngradesToNil(t *testing.T) {
	// mock 让 cleanupTempDir 内部那次调用「看起来像」Windows 删除被锁的 DLL
	orig := removeDirFn
	defer func() { removeDirFn = orig }()

	removeDirFn = func(path string) error {
		return &os.PathError{
			Op:   "RemoveAll",
			Path: path,
			Err:  errnoAccessDeniedWin,
		}
	}

	if err := cleanupTempDir(`C:\fake\path\nazhi-cli-ocr-busy`); err != nil {
		t.Fatalf("access-denied 应降级为 nil，实际返回 %v", err)
	}
}

// TestCleanupTempDir_SharingViolation_DowngradesToNil 同上，覆盖
// ERROR_SHARING_VIOLATION（另一个 Windows 文件占用 errno）。
func TestCleanupTempDir_SharingViolation_DowngradesToNil(t *testing.T) {
	orig := removeDirFn
	defer func() { removeDirFn = orig }()

	removeDirFn = func(path string) error {
		return &os.PathError{
			Op:   "RemoveAll",
			Path: path,
			Err:  errnoSharingViolationWin,
		}
	}

	if err := cleanupTempDir(`C:\fake\path\shared-dll`); err != nil {
		t.Fatalf("sharing-violation 应降级为 nil，实际返回 %v", err)
	}
}

// TestCleanupTempDir_OtherError_Propagates 「不静默吞错」铁律：
// 除 access-denied/sharing-violation 之外的真实清理错误必须照常返回，
// 否则 Linux 上的权限拒绝、磁盘满、只读卷等都会变成 silent failure。
func TestCleanupTempDir_OtherError_Propagates(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"plain error", errors.New("disk full while removing")},
		{"wrapped plain error", &os.PathError{
			Op:   "RemoveAll",
			Path: "/tmp/x",
			Err:  errors.New("read-only file system"),
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orig := removeDirFn
			defer func() { removeDirFn = orig }()

			removeDirFn = func(path string) error { return tc.err }

			err := cleanupTempDir("/tmp/test")
			if err == nil {
				t.Fatalf("非 DLL 占用错误应被透传，但返回 nil")
			}
			if !errors.Is(err, tc.err) {
				t.Errorf("错误应保留原 error 链（errors.Is），实际 %v 不包含 %v", err, tc.err)
			}
		})
	}
}

// TestCleanupTempDir_NilError_DeletesNormally 没有错误时直接删除且返回 nil。
// 这是基线测试，验证 helper 不是「永远返回 nil」的空壳。
func TestCleanupTempDir_NilError_DeletesNormally(t *testing.T) {
	dir := t.TempDir()
	// 在 dir 下再放一个子目录，让 RemoveAll 真的有事可做
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatalf("Mkdir 失败: %v", err)
	}

	orig := removeDirFn
	defer func() { removeDirFn = orig }()
	removeDirFn = os.RemoveAll

	if err := cleanupTempDir(dir); err != nil {
		t.Fatalf("正常删除应返回 nil，实际 %v", err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("temp dir 应被删除，实际：err=%v", err)
	}
}

// TestOCR_CloseWindowsBusyDLL_NoStderrPollution 端到端语义：OCR.Close() 在
// Windows DLL 持锁场景必须成功（返回 nil），不让 stderr 被 OS 占用错误污染。
//
// 与 ocr_close_test.go 中 TestOCR_CloseReturnsRemoveAllError 互补：
// 老测试用 null byte 路径让 RemoveAll 必失败，覆盖「非 DLL 占用错误照常返回」，
// 本测试覆盖「DLL 占用应降级」。
func TestOCR_CloseWindowsBusyDLL_NoStderrPollution(t *testing.T) {
	orig := removeDirFn
	defer func() { removeDirFn = orig }()

	// 模拟 Windows onnxruntime.dll 被进程持锁 → access denied
	removeDirFn = func(path string) error {
		return &os.PathError{
			Op:   "RemoveAll",
			Path: path,
			Err:  errnoAccessDeniedWin,
		}
	}

	o := &OCR{tempDir: `C:\fake\nazhi-cli-ocr-12345`}
	// o.ocr 留 nil：跳过 ddddocr.Close 路径，测试只关心 RemoveAll 降级
	if err := o.Close(); err != nil {
		t.Fatalf("Close 应对 DLL 占用降级返回 nil，实际：%v", err)
	}
}

// TestCleanupTempDir_LinuxEIO_Propagates 平台守卫 RED 测试：非 Windows 平台
// 上 syscall.Errno 数值（5=EIO、32=EPIPE）也会被 Windows 错误码命中。
// 若 isPlatformLibBusy 不加 GOOS 守卫，Linux 上 os.RemoveAll 因磁盘 I/O
// 故障返回裸 EIO/EPIPE 会被误判为「DLL 占用」而吞掉，违反「不静默吞错」铁律。
//
// 本测试：goosFn 注入 "linux" + 注入 errnoAccessDeniedWin(5)，断言 cleanupTempDir
// 必须返回 wrap 错误（不能降级为 nil）。
func TestCleanupTempDir_LinuxEIO_Propagates(t *testing.T) {
	origGoos := goosFn
	defer func() { goosFn = origGoos }()
	goosFn = func() string { return "linux" }

	origRm := removeDirFn
	defer func() { removeDirFn = origRm }()
	removeDirFn = func(path string) error {
		return &os.PathError{
			Op:   "RemoveAll",
			Path: path,
			Err:  errnoAccessDeniedWin, // 数值 = 5，与 Linux EIO 同值
		}
	}

	err := cleanupTempDir("/tmp/nazhi-cli-ocr-fake")
	if err == nil {
		t.Fatalf("Linux + errno=5(EIO) 必须透传错误，不能降级为 nil（违反「不静默吞错」铁律）")
	}
	// 进一步断言：必须是 wrap 后的业务错误，而非 raw syscall error
	if !strings.Contains(err.Error(), "清理临时目录") {
		t.Errorf("应保留 wrap 中文错误信息，实际：%v", err)
	}
}

// TestCleanupTempDir_LinuxEPIPE_Propagates 同上场景，覆盖 errno=32(EPIPE)。
func TestCleanupTempDir_LinuxEPIPE_Propagates(t *testing.T) {
	origGoos := goosFn
	defer func() { goosFn = origGoos }()
	goosFn = func() string { return "linux" }

	origRm := removeDirFn
	defer func() { removeDirFn = origRm }()
	removeDirFn = func(path string) error {
		return &os.PathError{
			Op:   "RemoveAll",
			Path: path,
			Err:  errnoSharingViolationWin, // 数值 = 32，与 Linux EPIPE 同值
		}
	}

	err := cleanupTempDir("/tmp/nazhi-cli-ocr-fake")
	if err == nil {
		t.Fatalf("Linux + errno=32(EPIPE) 必须透传错误，不能降级为 nil")
	}
}

// TestCleanupTempDir_WindowsAccessDenied_StillDowngrades 回归护栏：补 GOOS
// 守卫后，Windows 路径上的降级行为不能退化。确保 RED→GREEN 修复不破坏
// Windows DLL 持锁的核心降级语义。
func TestCleanupTempDir_WindowsAccessDenied_StillDowngrades(t *testing.T) {
	origGoos := goosFn
	defer func() { goosFn = origGoos }()
	goosFn = func() string { return "windows" }

	origRm := removeDirFn
	defer func() { removeDirFn = origRm }()
	removeDirFn = func(path string) error {
		return &os.PathError{
			Op:   "RemoveAll",
			Path: path,
			Err:  errnoAccessDeniedWin,
		}
	}

	if err := cleanupTempDir(`C:\fake\nazhi-cli-ocr-busy`); err != nil {
		t.Fatalf("Windows + errno=5 应降级为 nil，实际：%v", err)
	}
}

// TestCleanupTempDir_DarwinAccessDenied_Propagates 覆盖 darwin：注释说
// 「非 Windows 平台永远 false」，GOOS 守卫应不止排除 linux，也排除 macOS。
func TestCleanupTempDir_DarwinAccessDenied_Propagates(t *testing.T) {
	origGoos := goosFn
	defer func() { goosFn = origGoos }()
	goosFn = func() string { return "darwin" }

	origRm := removeDirFn
	defer func() { removeDirFn = origRm }()
	removeDirFn = func(path string) error {
		return &os.PathError{
			Op:   "RemoveAll",
			Path: path,
			Err:  errnoAccessDeniedWin,
		}
	}

	if err := cleanupTempDir("/tmp/nazhi-cli-ocr-darwin"); err == nil {
		t.Fatal("darwin + errno=5 必须透传，不应降级")
	}
}
