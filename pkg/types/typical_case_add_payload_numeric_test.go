package types

import (
	"encoding/json"
	"testing"
)

func TestAddTypicalCasePayload_TypeRoleLevel_NumericFlex(t *testing.T) {
	cases := []struct {
		json      string
		wantType  string
		wantRole  string
		wantLevel string
	}{
		{`{"title":"t","type":1,"role":2,"level":3}`, "1", "2", "3"},
		{`{"title":"t","type":"1","role":"2","level":"3"}`, "1", "2", "3"},
		{`{"title":"t","type":2.0,"role":1.0,"level":5.0}`, "2", "1", "5"},
	}
	for i, tc := range cases {
		var p AddTypicalCasePayload
		if err := json.Unmarshal([]byte(tc.json), &p); err != nil {
			t.Fatalf("case %d unmarshal failed: %v json=%s", i, err, tc.json)
		}
		if p.Type != tc.wantType || p.Role != tc.wantRole || p.Level != tc.wantLevel {
			t.Fatalf("case %d want %s/%s/%s got %s/%s/%s", i, tc.wantType, tc.wantRole, tc.wantLevel, p.Type, p.Role, p.Level)
		}
	}
}

func TestAddTypicalCasePayload_TypeRoleLevel_NumericRejectFractional(t *testing.T) {
	var p AddTypicalCasePayload
	if err := json.Unmarshal([]byte(`{"title":"t","type":1.5}`), &p); err == nil {
		t.Fatal("fractional type should be rejected")
	}
}
