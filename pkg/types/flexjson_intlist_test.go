package types

import (
	"encoding/json"
	"testing"
)

func TestIntList_UnmarshalNumberArray(t *testing.T) {
	var l IntList
	if err := json.Unmarshal([]byte(`[2025,9,1]`), &l); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(l) != 3 || l[0] != 2025 || l[1] != 9 || l[2] != 1 {
		t.Fatalf("got %v", l)
	}
}

func TestIntList_UnmarshalStringArray(t *testing.T) {
	var l IntList
	if err := json.Unmarshal([]byte(`["2025","9","1"]`), &l); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(l) != 3 || l[0] != 2025 {
		t.Fatalf("got %v", l)
	}
}

func TestIntList_UnmarshalNull(t *testing.T) {
	var l IntList
	if err := json.Unmarshal([]byte(`null`), &l); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if l != nil {
		t.Fatalf("want nil, got %v", l)
	}
}

func TestUserInfo_AdmissionDateNumberArray(t *testing.T) {
	// 真实 getMyInfo returnData 片段：admissionDate 为 number 数组
	raw := `{"id":1,"name":"测","studentNumber":"S1","studentId":1,"schoolId":173,"gradeId":1,"gradeName":"高一","classId":1,"className":"八班","seat":1,"admissionDate":[2025,9,1]}`
	var ui UserInfo
	if err := json.Unmarshal([]byte(raw), &ui); err != nil {
		t.Fatalf("UserInfo unmarshal: %v", err)
	}
	if len(ui.AdmissionDate) != 3 || ui.AdmissionDate[0] != 2025 {
		t.Fatalf("AdmissionDate = %v", ui.AdmissionDate)
	}
}
