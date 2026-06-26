// Package ocr 内部白盒测试：B5 platformLibName 跨平台命名验证。
package ocr

import (
	"runtime"
	"strings"
	"testing"
)

// TestPlatformLibNameFor_Darwin 验证 darwin 平台返回 .dylib 结尾的文件名。
func TestPlatformLibNameFor_Darwin(t *testing.T) {
	name := platformLibNameFor("darwin")
	if !strings.HasSuffix(name, ".dylib") {
		t.Errorf("darwin 平台应返回 .dylib 结尾的文件名，实际: %q", name)
	}
	if name != "libonnxruntime.dylib" {
		t.Errorf("darwin 平台应返回 libonnxruntime.dylib，实际: %q", name)
	}
}

// TestPlatformLibNameFor_Windows 验证 windows 平台返回 .dll 结尾的文件名。
func TestPlatformLibNameFor_Windows(t *testing.T) {
	name := platformLibNameFor("windows")
	if !strings.HasSuffix(name, ".dll") {
		t.Errorf("windows 平台应返回 .dll 结尾的文件名，实际: %q", name)
	}
	if name != "onnxruntime.dll" {
		t.Errorf("windows 平台应返回 onnxruntime.dll，实际: %q", name)
	}
}

// TestPlatformLibNameFor_Linux 验证 linux 平台返回 .so 结尾的文件名。
func TestPlatformLibNameFor_Linux(t *testing.T) {
	name := platformLibNameFor("linux")
	if !strings.HasSuffix(name, ".so") {
		t.Errorf("linux 平台应返回 .so 结尾的文件名，实际: %q", name)
	}
	if name != "libonnxruntime.so" {
		t.Errorf("linux 平台应返回 libonnxruntime.so，实际: %q", name)
	}
}

// TestPlatformLibNameFor_Default 验证未知平台返回无扩展名 "onnxruntime"。
func TestPlatformLibNameFor_Default(t *testing.T) {
	name := platformLibNameFor("freebsd")
	if name != "onnxruntime" {
		t.Errorf("未知平台应返回 onnxruntime，实际: %q", name)
	}
}

// TestPlatformLibName_Live 在当前运行的 OS 上验证 platformLibName 行为。
func TestPlatformLibName_Live(t *testing.T) {
	name := platformLibName()
	if name == "" {
		t.Fatal("platformLibName() 不应返回空字符串")
	}
	t.Logf("runtime.GOOS=%s -> platformLibName()=%s", runtime.GOOS, name)
}
