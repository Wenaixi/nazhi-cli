package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// helpers
func mockBizWithMetaAndCapture(t *testing.T, metaJSON string, capture *types.TaskAddCirclePayload) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"张三","schoolName":"测试中学"}}`))
		case "/api/studentCircleNew/getCircleTypeByTaskId":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(metaJSON))
		case "/api/studentCircleNew/addCircle":
			_ = json.NewDecoder(r.Body).Decode(capture)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"提交成功"}`))
		case "/api/studentCircleNew/editCircle":
			_ = json.NewDecoder(r.Body).Decode(capture)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"修改成功"}`))
		default:
			t.Errorf("意外路径: %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
}

func metaForTask(hours float64, typeVal int) string {
	// hours>0 => hoursStatus=true; hours<=0 => editable
	return `{"code":1,"msg":"成功","dataMap":{"task_name":"t","circle_type_id":9256,"hours":` + jsonStr(hours) + `, "type_name":"x","dimension_id":9,"dimension_name":"y","task_id":1001,"remark":"","type":` + jsonStr(typeVal) + `}}`
}
func jsonStr(v any) string { b, _ := json.Marshal(v); return string(b) }

// TestSubmitTask_TargetName1 思想品德
func TestSubmitTask_TargetName1_Compat(t *testing.T) {
	var cap types.TaskAddCirclePayload
	srv := mockBizWithMetaAndCapture(t, metaForTask(2, 1), &cap)
	defer srv.Close()
	c, _ := New(WithBaseURL(srv.URL), WithSSOBase(srv.URL), WithTimeout(5*1000*1000*1000))
	defer c.Close()
	_, err := c.SubmitTask(t.Context(), "tok", types.TaskSubmitInput{
		TaskID: 1001, Content: "心得",
		Name: "活动名称1", Address: "操场", Hours: "2", PlayRole: "2",
	})
	if err != nil {
		t.Fatalf("target1 submit: %v", err)
	}
	if cap.Name != "活动名称1" || cap.Address != "操场" || cap.PlayRole != "2" {
		t.Fatalf("target1 field mismatch: %+v", cap)
	}
	if cap.Hours != 2 {
		t.Fatalf("target1 hours want 2 got %v", cap.Hours)
	}
}

// Test target 4: 艺术素养-艺术活动项目（用户例子：班班有歌声）
func TestSubmitTask_TargetName4_ArtCompat(t *testing.T) {
	var cap types.TaskAddCirclePayload
	srv := mockBizWithMetaAndCapture(t, metaForTask(4, 4), &cap)
	defer srv.Close()
	c, _ := New(WithBaseURL(srv.URL), WithSSOBase(srv.URL), WithTimeout(5*1000*1000*1000))
	defer c.Close()
	_, err := c.SubmitTask(t.Context(), "tok", types.TaskSubmitInput{
		TaskID: 1001, Content: "歌声心得",
		Name:     "2026年青春唱响逐新章，美育涵养润芳华班班有歌声",
		HostName: "校团委", ObtainTime: "2026-04-15", Rank: "一等奖", Level: "5",
	})
	if err != nil {
		t.Fatalf("target4 submit: %v", err)
	}
	if cap.Name == "" || cap.HostName != "校团委" || cap.ObtainTime != "2026-04-15" {
		t.Fatalf("target4 mismatch: %+v", cap)
	}
	if cap.Rank != "一等奖" || cap.Level != "5" {
		t.Fatalf("target4 rank/level mismatch: %+v", cap)
	}
	// hours preset>0 时可空，但本例 meta=4 且用户未显式 hours -> 用 preset
	if cap.Hours != 4 {
		t.Fatalf("target4 hours preset want 4 got %v", cap.Hours)
	}
}

// Test target 6: 实践创新 120h
func TestSubmitTask_TargetName6_Practice120h(t *testing.T) {
	var cap types.TaskAddCirclePayload
	srv := mockBizWithMetaAndCapture(t, metaForTask(120, 6), &cap)
	defer srv.Close()
	c, _ := New(WithBaseURL(srv.URL), WithSSOBase(srv.URL), WithTimeout(5*1000*1000*1000))
	defer c.Close()
	_, err := c.SubmitTask(t.Context(), "tok", types.TaskSubmitInput{
		TaskID: 1001, Content: "暑期实践心得",
		OrgName: "XX社区", Address: "XX社区", CheckResult: "1",
		// Hours 留空 -> 应自动用 120
	})
	if err != nil {
		t.Fatalf("target6 submit: %v", err)
	}
	if cap.OrgName != "XX社区" || cap.Address != "XX社区" || cap.CheckResult != "1" {
		t.Fatalf("target6 mismatch: %+v", cap)
	}
	if cap.Hours != 120 {
		t.Fatalf("target6 hours want 120 got %v", cap.Hours)
	}
}

func TestSubmitTask_TargetName6_HoursOverride(t *testing.T) {
	var cap types.TaskAddCirclePayload
	srv := mockBizWithMetaAndCapture(t, metaForTask(120, 6), &cap)
	defer srv.Close()
	c, _ := New(WithBaseURL(srv.URL), WithSSOBase(srv.URL), WithTimeout(5*1000*1000*1000))
	defer c.Close()
	_, err := c.SubmitTask(t.Context(), "tok", types.TaskSubmitInput{
		TaskID: 1001, Content: "心得", OrgName: "o", Address: "a", CheckResult: "1", Hours: "80",
	})
	if err != nil {
		t.Fatalf("override: %v", err)
	}
	if cap.Hours != 80 {
		t.Fatalf("override hours want 80 got %v", cap.Hours)
	}
}

// 其它 targetName 快速冒烟：2,3,5,7,8,9,10,11,12,13,14
func TestSubmitTask_AllTargetNames_Smoke(t *testing.T) {
	cases := []struct {
		name    string
		typeVal int
		hours   float64
		input   types.TaskSubmitInput
		check   func(types.TaskAddCirclePayload) error
	}{
		{"2-学科竞赛", 2, 0, types.TaskSubmitInput{TaskID: 1001, Content: "c", ActivityName: "数学竞赛", HostName: "教育局", ObtainTime: "2026-01-01", Rank: "一等", Level: "3"}, nil},
		{"3-体育", 3, 0, types.TaskSubmitInput{TaskID: 1001, Content: "c", SportsName: "百米", HostName: "体育局", ObtainTime: "2026-01-01"}, nil},
		{"5-艺术团队", 5, 0, types.TaskSubmitInput{TaskID: 1001, Content: "c", TeamName: "合唱团", OrgName: "学校", CheckResult: "1", Rank: "一等", CircleBeginDate: "2026-01-01", CircleEndDate: "2026-01-10", Hours: "2"}, nil},
		{"7-科技创新", 7, 0, types.TaskSubmitInput{TaskID: 1001, Content: "c", ResultsName: "小发明", HostName: "主办", ObtainTime: "2026-01-01"}, nil},
		{"8-创造发明", 8, 0, types.TaskSubmitInput{TaskID: 1001, Content: "c", ResultsName: "专利作品", PatentType: "实用新型", ObtainTime: "2026-01-01", PatentNum: "ZL123", Hours: "1"}, nil},
		{"9-爱好", 9, 1, types.TaskSubmitInput{TaskID: 1001, Content: "c", LikeSpecialty1: "篮球"}, nil},
		{"10-劳动素养", 10, 0, types.TaskSubmitInput{TaskID: 1001, Content: "c", OrgName: "学校", Address: "操场", Hours: "2", Level: "5"}, nil},
		{"11-代表性劳动", 11, 1, types.TaskSubmitInput{TaskID: 1001, Content: "c", ObtainTime: "2026-01-01", Address: "基地"}, nil},
		{"12-特长技术", 12, 1, types.TaskSubmitInput{TaskID: 1001, Content: "c", SpecialtyTechnology: "电工"}, nil},
		{"13-劳动成果", 13, 1, types.TaskSubmitInput{TaskID: 1001, Content: "c", ResultsName: "作品", Address: "车间", ObtainTime: "2026-01-01"}, nil},
		{"14-劳动竞赛", 14, 0, types.TaskSubmitInput{TaskID: 1001, Content: "c", SportsName: "技能大赛", HostName: "主办", ObtainTime: "2026-01-01", Address: "场馆", Hours: "1"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var cap types.TaskAddCirclePayload
			srv := mockBizWithMetaAndCapture(t, metaForTask(tc.hours, tc.typeVal), &cap)
			defer srv.Close()
			c, _ := New(WithBaseURL(srv.URL), WithSSOBase(srv.URL), WithTimeout(5*1000*1000*1000))
			defer c.Close()
			// 兜底 hours：若 meta<=0 且 input.Hours 空则补 "1" 以过 parseHours
			in := tc.input
			if in.Hours == "" && tc.hours <= 0 {
				// 对需要 hours 的分支已填；无需
			}
			_, err := c.SubmitTask(t.Context(), "tok", in)
			if err != nil {
				t.Fatalf("%s submit: %v", tc.name, err)
			}
			if cap.Content != "c" {
				t.Fatalf("content lost")
			}
			if tc.check != nil {
				if err := tc.check(cap); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

// Golden JSON: emulate frontend JSON.stringify(form) for 实践创新 120h
func _goldenDecodeTaskSubmitInput(data []byte) (types.TaskSubmitInput, error) {
	// 复刻 cmd/nazhi 的 normalizeTaskInputJSON：hours允许小数，level/checkResult/playRole 仅有限整数转字符串
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		var in types.TaskSubmitInput
		return in, err
	}
	for _, name := range []string{"hours", "level", "checkResult", "playRole"} {
		raw, ok := fields[name]
		if !ok {
			continue
		}
		// 也兼容大小写变体（如 Level）——简易处理：遍历兜底
		if len(raw) == 0 || raw[0] == '"' {
			continue
		}
		var num json.Number
		if err := json.Unmarshal(raw, &num); err != nil {
			var in types.TaskSubmitInput
			return in, err
		}
		enc, _ := json.Marshal(num.String())
		fields[name] = enc
		// 同步大小写键：若原键是 Level，删除后用小写
		for k := range fields {
			if k != name && len(k) == len(name) { /* 大小写兼容已由 CLI 层处理，这里 golden 用小写即可 */
			}
		}
	}
	// 处理 Level 大小写：若存在大写键则搬运
	for _, canon := range []string{"level", "hours", "checkResult", "playRole"} {
		for k, v := range fields {
			if len(k) != len(canon) {
				continue
			}
			if k == canon {
				continue
			}
			ok := true
			for i := range k {
				a := k[i]
				b := canon[i]
				if a >= 'A' && a <= 'Z' {
					a += 32
				}
				if b >= 'A' && b <= 'Z' {
					b += 32
				}
				if a != b {
					ok = false
					break
				}
			}
			if ok {
				fields[canon] = v
				delete(fields, k)
			}
		}
	}
	norm, _ := json.Marshal(fields)
	var in types.TaskSubmitInput
	if err := json.Unmarshal(norm, &in); err != nil {
		return in, err
	}
	return in, nil
}
func TestSubmitTask_GoldenFrontendPractice120h(t *testing.T) {
	raw := []byte(`{
		"taskId": 1001,
		"content": "2025-2026第二学期暑期社会实践的收获与感悟，xxxxxxxxxxxxxxxxxxxx",
		"orgName": "共青团XX委员会",
		"address": "XX实践基地",
		"hours": "120",
		"checkResult": "1",
		"circleTaskId": 1001,
		"pictureList": []
	}`)
	// CLI path should normalize aliases + numeric codes (here all strings already)
	in, err := _goldenDecodeTaskSubmitInput(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if in.OrgName != "共青团XX委员会" || in.Hours != "120" || in.CheckResult != "1" {
		t.Fatalf("golden decode mismatch: %+v", in)
	}
	var cap types.TaskAddCirclePayload
	srv := mockBizWithMetaAndCapture(t, metaForTask(120, 6), &cap)
	defer srv.Close()
	c, _ := New(WithBaseURL(srv.URL), WithSSOBase(srv.URL), WithTimeout(5*1000*1000*1000))
	defer c.Close()
	_, err = c.SubmitTask(t.Context(), "tok", in)
	if err != nil {
		t.Fatalf("golden practice submit: %v", err)
	}
	b, _ := json.Marshal(cap)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, k := range []string{"name", "hostName", "circleDate", "rank", "level", "content", "pictureList", "circleTaskId", "circleTypeId", "dimensionId", "hours", "circleBeginDate", "circleEndDate", "checkResult", "patentType", "patentNum", "address", "termName", "activityName", "sportsName", "teamName", "orgName", "resultsName", "obtainTime", "specialtyTechnology", "playRole", "likeSpecialty1", "likeSpecialty2", "likeSpecialty3"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("payload missing key %s in %s", k, string(b))
		}
	}
	if m["hours"] != float64(120) || m["address"] != "XX实践基地" || m["checkResult"] != "1" {
		t.Fatalf("golden fields mismatch: %s", string(b))
	}
}

func TestSubmitTask_GoldenFrontendArtSinging(t *testing.T) {
	raw := []byte(`{
		"taskId": 1002,
		"content": "班班有歌声心得",
		"name": "2026年青春唱响逐新章，美育涵养润芳华班班有歌声",
		"hostName": "校团委",
		"obtainTime": "2026-04-15",
		"rank": "一等奖",
		"level": 5,
		"hours": 4
	}`)
	in, err := _goldenDecodeTaskSubmitInput(raw)
	if err != nil {
		t.Fatalf("decode art: %v", err)
	}
	if in.Level != "5" || in.Hours != "4" {
		t.Fatalf("art numeric->string: %+v", in)
	}
	var cap types.TaskAddCirclePayload
	srv := mockBizWithMetaAndCapture(t, metaForTask(4, 4), &cap)
	defer srv.Close()
	c, _ := New(WithBaseURL(srv.URL), WithSSOBase(srv.URL), WithTimeout(5*1000*1000*1000))
	defer c.Close()
	_, err = c.SubmitTask(t.Context(), "tok", in)
	if err != nil {
		t.Fatalf("art submit: %v", err)
	}
	if cap.Name == "" || cap.HostName != "校团委" || cap.Hours != 4 {
		t.Fatalf("art cap mismatch: %+v", cap)
	}
}

func TestParseHours_ZeroAllowedForNonRequiredTarget(t *testing.T) {
	if _, err := parseHours("", 0, 2); err != nil {
		t.Fatalf("target 2 hours 0 should be allowed: %v", err)
	}
	if _, err := parseHours("", 0, 4); err != nil {
		t.Fatalf("target 4 hours 0 should be allowed: %v", err)
	}
	if _, err := parseHours("", 0, 6); err == nil {
		t.Fatal("target 6 hours 0 should be required")
	}
	if _, err := parseHours("", 0, 1); err == nil {
		t.Fatal("target 1 hours 0 should be required")
	}
	if _, err := parseHours("", 0, 10); err == nil {
		t.Fatal("target 10 hours 0 should be required")
	}
}

func TestEditCircle_TargetName4And6(t *testing.T) {
	var cap types.TaskAddCirclePayload
	srv := mockBizWithMetaAndCapture(t, metaForTask(4, 4), &cap)
	defer srv.Close()
	c, _ := New(WithBaseURL(srv.URL), WithSSOBase(srv.URL), WithTimeout(5*1000*1000*1000))
	defer c.Close()
	_, err := c.EditCircle(t.Context(), "tok", types.TaskEditInput{ID: 5400001, TaskID: 1001, Content: "修改后", Name: "活动", HostName: "h", ObtainTime: "2026-04-15"})
	if err != nil {
		t.Fatalf("edit 4: %v", err)
	}
	if cap.ID == nil || *cap.ID != 5400001 {
		t.Fatalf("edit id not carried: %+v", cap.ID)
	}

	var cap2 types.TaskAddCirclePayload
	srv2 := mockBizWithMetaAndCapture(t, metaForTask(120, 6), &cap2)
	defer srv2.Close()
	c2, _ := New(WithBaseURL(srv2.URL), WithSSOBase(srv2.URL), WithTimeout(5*1000*1000*1000))
	defer c2.Close()
	_, err = c2.EditCircle(t.Context(), "tok", types.TaskEditInput{ID: 5400002, TaskID: 1001, Content: "修改实践", OrgName: "o2", Address: "a2", CheckResult: "2"})
	if err != nil {
		t.Fatalf("edit 6: %v", err)
	}
	if cap2.Hours != 120 || cap2.OrgName != "o2" {
		t.Fatalf("edit 6 cap: %+v", cap2)
	}
}

func TestSubmitTask_PresetHoursZeroRequiresExplicit(t *testing.T) {
	var cap types.TaskAddCirclePayload
	srv := mockBizWithMetaAndCapture(t, metaForTask(0, 6), &cap)
	defer srv.Close()
	c, _ := New(WithBaseURL(srv.URL), WithSSOBase(srv.URL), WithTimeout(5*1000*1000*1000))
	defer c.Close()
	_, err := c.SubmitTask(t.Context(), "tok", types.TaskSubmitInput{TaskID: 1001, Content: "c", OrgName: "o", Address: "a", CheckResult: "1"})
	if err == nil {
		t.Fatal("meta 0 且未填 hours 应失败")
	}
}
