package types

import "testing"

func TestTaskLevelName_Exhaustive(t *testing.T) {
	cases := []struct{ code, want string }{
		{TaskLevelNational, "国家"},
		{TaskLevelProvince, "省"},
		{TaskLevelCity, "地区/市"},
		{TaskLevelCounty, "区/县/街道/社区"},
		{TaskLevelSchool, "校"},
		{TaskLevelGrade, "年段"},
		{"", ""},
		{"0", ""},
		{"7", ""},
		{"99", ""},
	}
	for _, tc := range cases {
		got := TaskLevelName(tc.code)
		if got != tc.want {
			t.Fatalf("TaskLevelName(%q) want %q got %q", tc.code, tc.want, got)
		}
	}
	if TaskLevelNational != "1" || TaskLevelGrade != "6" {
		t.Fatal("level constants drift")
	}
}

func TestCheckResultName_Exhaustive(t *testing.T) {
	cases := []struct{ code, want string }{
		{CheckResultExcellent, "优秀"},
		{CheckResultGood, "良"},
		{CheckResultPass, "合格"},
		{CheckResultPoor, "差"},
		{"", ""},
		{"0", ""},
		{"5", ""},
	}
	for _, tc := range cases {
		got := CheckResultName(tc.code)
		if got != tc.want {
			t.Fatalf("CheckResultName(%q) want %q got %q", tc.code, tc.want, got)
		}
	}
}
