package types

import (
	"encoding/json"
	"testing"
)

// TestFlexInt_DecodeMatrix 回归测试：FlexInt 必须兼容平台可能出现的
// number、浮点字面量（4.0）、数字字符串与 null 四类形状。
//
// 背景（十三域审计 P2-A）：UserInfo 的 Seat/YouthLeagueFlag/Nation/IDType
// 曾用裸 int，encoding/json 拒绝 4.0→int 导致 DecodeReturnData 整页失败，
// 被 GetMyInfo fallback 链吞成 ErrEmptyUserInfo（与 HonorRecord.score=4.0
// 同款已踩坑）。FlexInt 与 FlexFloat/IntList 同族归一。
func TestFlexInt_DecodeMatrix(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"整数number", `5`, 5},
		{"浮点整值", `4.0`, 4},
		{"数字字符串", `"45"`, 45},
		{"null", `null`, 0},
	}
	for _, tc := range cases {
		var v FlexInt
		if err := json.Unmarshal([]byte(tc.in), &v); err != nil {
			t.Fatalf("%s 解码失败: %v", tc.name, err)
		}
		if int(v) != tc.want {
			t.Errorf("%s 期望 %d，实际 %d", tc.name, tc.want, int(v))
		}
	}

	// 非整数浮点必须报错而非静默截断
	var v FlexInt
	if err := json.Unmarshal([]byte("2.5"), &v); err == nil {
		t.Error("非整数浮点应报错防静默截断")
	}
}

// TestUserInfo_IntegerFieldsTolerateFloatLiterals 回归测试：
// getMyInfo 响应中四个曾为裸 int 的展示字段遇浮点字面量时不得炸整页。
func TestUserInfo_IntegerFieldsTolerateFloatLiterals(t *testing.T) {
	raw := `{"name":"张三","seat":45.0,"youthLeagueFlag":1.0,"nation":1.0,"idType":1.0}`
	var u UserInfo
	if err := json.Unmarshal([]byte(raw), &u); err != nil {
		t.Fatalf("浮点字面量不应导致解码失败: %v", err)
	}
	if u.Seat != 45 || u.YouthLeagueFlag != 1 || u.Nation != 1 || u.IDType != 1 {
		t.Errorf("字段值不符: seat=%d youth=%d nation=%d idType=%d", u.Seat, u.YouthLeagueFlag, u.Nation, u.IDType)
	}
}
