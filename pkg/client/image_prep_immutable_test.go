// Package client 内部白盒测试：G4 getQualitySteps/getScaleFactors 不可变验证。
package client

import (
	"reflect"
	"testing"
)

// TestGetQualitySteps_ReturnsNewSlice 验证每次调用都返回不同副本。
func TestGetQualitySteps_ReturnsNewSlice(t *testing.T) {
	a := getQualitySteps()
	b := getQualitySteps()

	if len(a) != 3 || a[0] != 80 || a[1] != 60 || a[2] != 40 {
		t.Errorf("getQualitySteps() 返回意外的值: %v", a)
	}

	// 修改 a 不应影响 b
	if len(b) > 0 {
		a[0] = 999
		if b[0] == 999 {
			t.Errorf("G4 回归：修改 getQualitySteps() 的返回副本影响了其他调用方，"+
				"a[0]=%d, b[0]=%d (期望 b[0] 保持 80)", a[0], b[0])
		}
	}

	ha := reflect.ValueOf(a).Pointer()
	hb := reflect.ValueOf(b).Pointer()
	if ha == hb {
		t.Error("getQualitySteps() 两次调用返回了同一底层数组")
	}
}

// TestGetScaleFactors_ReturnsNewSlice 验证每次调用都返回不同副本。
func TestGetScaleFactors_ReturnsNewSlice(t *testing.T) {
	a := getScaleFactors()
	b := getScaleFactors()

	if len(a) != 7 {
		t.Errorf("getScaleFactors() 长度应为 7，实际 %d", len(a))
	}

	a[0] = 0.5
	if b[0] == 0.5 {
		t.Errorf("G4 回归：修改 getScaleFactors() 的返回副本影响了其他调用方，"+
			"a[0]=%.1f, b[0]=%.1f (期望 b[0] 保持 0.7)", a[0], b[0])
	}

	ha := reflect.ValueOf(a).Pointer()
	hb := reflect.ValueOf(b).Pointer()
	if ha == hb {
		t.Error("getScaleFactors() 两次调用返回了同一底层数组")
	}
}

// TestGetQualitySteps_Values 验证 getQualitySteps 返回值正确。
func TestGetQualitySteps_Values(t *testing.T) {
	steps := getQualitySteps()
	expected := []int{80, 60, 40}
	if len(steps) != len(expected) {
		t.Fatalf("长度: 期望 %d, 实际 %d", len(expected), len(steps))
	}
	for i, v := range steps {
		if v != expected[i] {
			t.Errorf("步骤 %d: 期望 %d, 实际 %d", i, expected[i], v)
		}
	}
}

// TestGetScaleFactors_Values 验证 getScaleFactors 返回值正确。
func TestGetScaleFactors_Values(t *testing.T) {
	factors := getScaleFactors()
	if len(factors) != 7 {
		t.Fatalf("长度: 期望 7, 实际 %d", len(factors))
	}
	for i, v := range factors {
		if v != 0.7 {
			t.Errorf("因子 %d: 期望 0.7, 实际 %.1f", i, v)
		}
	}
}
