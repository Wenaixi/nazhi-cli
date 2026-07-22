package types

import (
	"encoding/json"
	"testing"
)

func TestFlexBool_UnmarshalVariants(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{`true`, true},
		{`false`, false},
		{`1`, true},
		{`0`, false},
		{`1.0`, true},
		{`0.0`, false},
		{`"true"`, true},
		{`"false"`, false},
		{`"1"`, true},
		{`"0"`, false},
		{`null`, false},
	}
	for _, c := range cases {
		var b FlexBool
		if err := json.Unmarshal([]byte(c.raw), &b); err != nil {
			t.Errorf("raw=%s err=%v", c.raw, err)
			continue
		}
		if b.Bool() != c.want {
			t.Errorf("raw=%s got %v want %v", c.raw, b.Bool(), c.want)
		}
	}
}

func TestFlexBool_Marshal(t *testing.T) {
	raw, err := json.Marshal(FlexBool(true))
	if err != nil || string(raw) != "true" {
		t.Fatalf("marshal true: %s %v", raw, err)
	}
	raw, err = json.Marshal(FlexBool(false))
	if err != nil || string(raw) != "false" {
		t.Fatalf("marshal false: %s %v", raw, err)
	}
}

// TestCircleRecord_LikeStatusNumber 平台若返回 likeStatus:1 不得整页失败。
func TestCircleRecord_LikeStatusNumber(t *testing.T) {
	raw := `{"id":1,"name":"x","content":"c","type_name":"t","likeStatus":1,"approved":0,"status":1}`
	var rec CircleRecord
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !rec.LikeStatus.Bool() {
		t.Error("likeStatus:1 期望 true")
	}
	if rec.Approved.Bool() {
		t.Error("approved:0 期望 false")
	}
}

// TestHonorRecord_ApprovedNumber 荣誉列表若带 approved:0/1 不得失败。
func TestHonorRecord_ApprovedNumber(t *testing.T) {
	raw := `{"id":1,"type_name":"a","level_name":"校","level":5,"approved":1,"status":1,"statusName":"通过","get_date":"2026-05-25","score":4.0}`
	var hr HonorRecord
	if err := json.Unmarshal([]byte(raw), &hr); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !hr.Approved.Bool() {
		t.Error("approved:1 期望 true")
	}
}
