package internal

import (
	"testing"
	"time"
)

func TestMapTaskStatus(t *testing.T) {
	cases := map[string]bool{
		"上传期 已提交": true,
		"未开始 未提交": false,
		"上传期 未提交": false,
		"":            false,
	}
	for in, want := range cases {
		if got := MapTaskStatus(in); got != want {
			t.Errorf("MapTaskStatus(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestMapSchoolID(t *testing.T) {
	if got, err := MapSchoolID("173"); err != nil || got != 173 {
		t.Errorf("MapSchoolID(\"173\") = %d, %v; want 173, nil", got, err)
	}
	if _, err := MapSchoolID("abc"); err == nil {
		t.Error("MapSchoolID(\"abc\") 应返回 error")
	}
}

func TestParseServerDate(t *testing.T) {
	got, err := ParseServerDate("2026-07-08")
	if err != nil {
		t.Fatalf("ParseServerDate 失败: %v", err)
	}
	if got.Year() != 2026 || got.Month() != 7 || got.Day() != 8 {
		t.Errorf("ParseServerDate 解析错: %v", got)
	}
	if _, err := ParseServerDate("not-a-date"); err == nil {
		t.Error("ParseServerDate(\"not-a-date\") 应返回 error")
	}
}

func TestMapCircleApproved(t *testing.T) {
	if MapCircleApproved(0) {
		t.Error("status=0 应为 false")
	}
	if !MapCircleApproved(1) {
		t.Error("status=1 应为 true")
	}
	_ = time.Now() // keep time import if unused
}