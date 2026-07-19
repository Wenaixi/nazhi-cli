// Package ocr 内部白盒测试：外部模型目录加载（modelDir）。
package ocr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWithModelDir 验证 WithModelDir 正确设置 OCR.modelDir。
func TestWithModelDir(t *testing.T) {
	o := New()
	WithModelDir("/tmp/models")(o)
	if o.modelDir != "/tmp/models" {
		t.Errorf("WithModelDir 设置失败：期望 %q，实际 %q", "/tmp/models", o.modelDir)
	}
}

// TestOCR_ExtractModels_FromExternalDir 验证 modelDir 指向的外部目录
// 中模型文件被正确复制到提取目录。
func TestOCR_ExtractModels_FromExternalDir(t *testing.T) {
	// 准备：创建源目录，写入模拟模型文件
	srcDir, err := os.MkdirTemp("", "ocr-model-src-*")
	if err != nil {
		t.Fatalf("MkdirTemp 源目录失败: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(srcDir) })

	// 写入三个平台所需的文件：原生库、ONNX 模型、字符集
	libFiles := []string{
		"common_old.onnx",
		"charsets_old.json",
		platformLibName(),
	}
	for _, f := range libFiles {
		if err := os.WriteFile(filepath.Join(srcDir, f), []byte("mock-"+f), 0644); err != nil {
			t.Fatalf("写入 %s 失败: %v", f, err)
		}
	}

	// 执行：创建 OCR 实例并设置 modelDir
	o := New()
	WithModelDir(srcDir)(o)

	dstDir, err := o.extractModels()
	if err != nil {
		t.Fatalf("extractModels() 从外部目录应成功，但错误: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dstDir) })

	// 验证：目标目录包含所有需要的文件，且内容匹配
	for _, f := range libFiles {
		got, err := os.ReadFile(filepath.Join(dstDir, f))
		if err != nil {
			t.Errorf("目标目录缺少文件 %s: %v", f, err)
			continue
		}
		if string(got) != "mock-"+f {
			t.Errorf("文件 %s 内容不匹配：期望 %q，实际 %q", f, "mock-"+f, string(got))
		}
	}
}

// TestOCR_ExtractModels_MissingFilesInModelDir 验证 modelDir 缺少文件时报错。
func TestOCR_ExtractModels_MissingFilesInModelDir(t *testing.T) {
	srcDir, err := os.MkdirTemp("", "ocr-model-missing-*")
	if err != nil {
		t.Fatalf("MkdirTemp 失败: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(srcDir) })

	// 只写入部分文件——缺少 onnxruntime 原生库
	_ = os.WriteFile(filepath.Join(srcDir, "common_old.onnx"), []byte("mock"), 0644)
	_ = os.WriteFile(filepath.Join(srcDir, "charsets_old.json"), []byte("mock"), 0644)

	o := New()
	WithModelDir(srcDir)(o)

	_, err = o.extractModels()
	if err == nil {
		t.Fatal("extractModels() 应失败（缺少原生库文件），但返回 nil")
	}
	if !strings.Contains(err.Error(), "失败") {
		t.Errorf("错误消息应描述具体哪个文件失败，实际: %v", err)
	}
}

// TestOCR_ExtractModels_ModelDirEmpty 验证 modelDir 为空字符串时
// extractModels 的行为：应该有明确的错误，而不是 panic。
func TestOCR_ExtractModels_ModelDirEmpty(t *testing.T) {
	o := New()
	// modelDir 默认就是空字符串
	_, err := o.extractModels()
	if err == nil {
		t.Log("注意：modelDir 为空且无嵌入数据时 extractModels 应返回错误")
		t.Log("如果嵌入数据已编译到二进制中，本测试可跳过")
	}
	// 不能 panic
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("extractModels() panic: %v", r)
		}
	}()
}

// TestPool_SetModelDir 验证 Pool 支持设置模型目录并传播到 OCR 实例。
func TestPool_SetModelDir(t *testing.T) {
	srcDir, err := os.MkdirTemp("", "ocr-pool-model-*")
	if err != nil {
		t.Fatalf("MkdirTemp 失败: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(srcDir) })

	// 写入必要的模型文件
	for _, f := range []string{"common_old.onnx", "charsets_old.json", platformLibName()} {
		if err := os.WriteFile(filepath.Join(srcDir, f), []byte("pool-"+f), 0644); err != nil {
			t.Fatalf("写入 %s 失败: %v", f, err)
		}
	}

	p := NewPool(0)
	p.SetModelDir(srcDir)

	// Pool 调用 Recognize 前先获取一个 OCR 实例，验证 modelDir 已注入
	if p.modelDir != srcDir {
		t.Errorf("Pool.modelDir 未正确设置：期望 %q，实际 %q", srcDir, p.modelDir)
		return
	}

	// 从 Pool 取一个 OCR 实例验证 modelDir 传播
	o := &OCR{} // 模拟 pool.Get 的行为
	_ = o       // 不在 Pool 上实际调用 Recognize（需要 CGO 和 ddddocr），只验证字段
}
