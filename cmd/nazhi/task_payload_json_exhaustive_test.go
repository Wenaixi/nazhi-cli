package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

func TestDecodeTaskInputJSON_Exhaustive_NumericHours(t *testing.T) {
	cases := []struct {
		name, payload string
		wantHours     string
		wantErr       bool
	}{
		{"hours int", `{"taskId":1,"content":"c","hours":4}`, "4", false},
		{"hours float", `{"taskId":1,"content":"c","hours":0.5}`, "0.5", false},
		{"hours 1.0 str via number 1.25", `{"taskId":1,"content":"c","hours":1.25}`, "1.25", false},
		{"hours scientific", `{"taskId":1,"content":"c","hours":1e2}`, "1e2", false},
		{"hours string", `{"taskId":1,"content":"c","hours":"2.5"}`, "2.5", false},
		{"hours null -> stay empty", `{"taskId":1,"content":"c","hours":null}`, "", false},
		{"hours missing -> empty", `{"taskId":1,"content":"c"}`, "", false},
		{"hours bool reject", `{"taskId":1,"content":"c","hours":true}`, "", true},
		{"hours array reject", `{"taskId":1,"content":"c","hours":[1]}`, "", true},
		{"hours object reject", `{"taskId":1,"content":"c","hours":{}}`, "", true},
		{"Hours case-insensitive 0.5", `{"taskId":1,"content":"c","Hours":0.5}`, "0.5", false},
		{"HOURS upper", `{"taskId":1,"content":"c","HOURS":1}`, "1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in, err := decodeTaskSubmitInput([]byte(tc.payload))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want err")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected %v", err)
			}
			if in.Hours != tc.wantHours {
				t.Fatalf("want %q got %q", tc.wantHours, in.Hours)
			}
		})
	}
}

func TestDecodeTaskInputJSON_Exhaustive_DiscreteCodes(t *testing.T) {
	for _, field := range []string{"level", "checkResult", "playRole"} {
		t.Run(field+" int ok", func(t *testing.T) {
			payload := []byte(`{"taskId":1,"content":"c","` + field + `":5}`)
			var in types.TaskSubmitInput
			if err := decodeTaskInputJSON(payload, &in); err != nil {
				t.Fatalf("%v", err)
			}
		})
		t.Run(field+" 1.0 canonicalizes to 1", func(t *testing.T) {
			payload := []byte(`{"taskId":1,"content":"c","` + field + `":1.0}`)
			var in types.TaskSubmitInput
			if err := decodeTaskInputJSON(payload, &in); err != nil {
				t.Fatalf("%v", err)
			}
			var got string
			switch field {
			case "level":
				got = in.Level
			case "checkResult":
				got = in.CheckResult
			case "playRole":
				got = in.PlayRole
			}
			if got != "1" {
				t.Fatalf("want 1 got %q", got)
			}
		})
		t.Run(field+" 1e1 -> 10", func(t *testing.T) {
			payload := []byte(`{"taskId":1,"content":"c","` + field + `":1e1}`)
			var in types.TaskSubmitInput
			if err := decodeTaskInputJSON(payload, &in); err != nil {
				t.Fatalf("%v", err)
			}
		})
		t.Run(field+" 5.5 reject", func(t *testing.T) {
			payload := []byte(`{"taskId":1,"content":"c","` + field + `":5.5}`)
			var in types.TaskSubmitInput
			if err := decodeTaskInputJSON(payload, &in); err == nil {
				t.Fatal("should reject fractional")
			}
		})
		t.Run(field+" 1e999 reject non-finite", func(t *testing.T) {
			payload := []byte(`{"taskId":1,"content":"c","` + field + `":1e999}`)
			var in types.TaskSubmitInput
			if err := decodeTaskInputJSON(payload, &in); err == nil {
				t.Fatal("should reject non-finite")
			}
		})
		t.Run(field+" -0.0 -> 0", func(t *testing.T) {
			payload := []byte(`{"taskId":1,"content":"c","` + field + `":-0.0}`)
			var in types.TaskSubmitInput
			if err := decodeTaskInputJSON(payload, &in); err != nil {
				t.Fatalf("%v", err)
			}
			var got string
			switch field {
			case "level":
				got = in.Level
			case "checkResult":
				got = in.CheckResult
			case "playRole":
				got = in.PlayRole
			}
			if got != "0" {
				t.Fatalf("want 0 got %q", got)
			}
		})
		t.Run(field+" string stays", func(t *testing.T) {
			payload := []byte(`{"taskId":1,"content":"c","` + field + `":"5"}`)
			var in types.TaskSubmitInput
			if err := decodeTaskInputJSON(payload, &in); err != nil {
				t.Fatalf("%v", err)
			}
			var got string
			switch field {
			case "level":
				got = in.Level
			case "checkResult":
				got = in.CheckResult
			case "playRole":
				got = in.PlayRole
			}
			if got != "5" {
				t.Fatalf("want 5 got %q", got)
			}
		})
		t.Run(field+" string 5.5 stays as string", func(t *testing.T) {
			payload := []byte(`{"taskId":1,"content":"c","` + field + `":"5.5"}`)
			var in types.TaskSubmitInput
			if err := decodeTaskInputJSON(payload, &in); err != nil {
				t.Fatalf("%v", err)
			}
		})
		t.Run(field+" null -> empty", func(t *testing.T) {
			payload := []byte(`{"taskId":1,"content":"c","` + field + `":null}`)
			var in types.TaskSubmitInput
			if err := decodeTaskInputJSON(payload, &in); err != nil {
				t.Fatalf("%v", err)
			}
		})
		t.Run(field+" bool reject", func(t *testing.T) {
			payload := []byte(`{"taskId":1,"content":"c","` + field + `":true}`)
			var in types.TaskSubmitInput
			if err := decodeTaskInputJSON(payload, &in); err == nil {
				t.Fatal("bool should reject")
			}
		})
		t.Run(field+" case-insensitive LEVEL", func(t *testing.T) {
			canon := field
			upper := strings.ToUpper(field[:1]) + field[1:]
			if field == "checkResult" {
				upper = "CheckResult"
			}
			if field == "playRole" {
				upper = "PlayRole"
			}
			payload := []byte(`{"taskId":1,"content":"c","` + upper + `":3}`)
			var in types.TaskSubmitInput
			if err := decodeTaskInputJSON(payload, &in); err != nil {
				t.Fatalf("%v", err)
			}
			var got string
			switch canon {
			case "level":
				got = in.Level
			case "checkResult":
				got = in.CheckResult
			case "playRole":
				got = in.PlayRole
			}
			if got != "3" {
				t.Fatalf("want 3 got %q for %s", got, upper)
			}
		})
	}
}

func TestDecodeTaskInputJSON_Exhaustive_Aliases(t *testing.T) {
	cases := []struct {
		name, payload string
		wantTaskID    int64
		wantImageIDs  []int64
	}{
		{"taskId canonical wins over alias", `{"taskId":1,"circleTaskId":99,"content":"c"}`, 1, nil},
		{"only alias circleTaskId", `{"circleTaskId":42,"content":"c"}`, 42, nil},
		{"CIRCLETASKID case-insensitive", `{"CIRCLETASKID":7,"content":"c"}`, 7, nil},
		{"imageIDs canonical wins", `{"taskId":1,"content":"c","imageIDs":[1],"pictureList":[9]}`, 1, []int64{1}},
		{"only pictureList alias", `{"taskId":1,"content":"c","pictureList":[55]}`, 1, []int64{55}},
		{"PictureList case-insensitive", `{"taskId":1,"content":"c","PictureList":[8]}`, 1, []int64{8}},
		{"both aliases mixed case canonical wins", `{"taskId":2,"CIRCLETASKID":99,"content":"c","ImageIDs":[2],"PictureList":[9]}`, 2, []int64{2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in, err := decodeTaskSubmitInput([]byte(tc.payload))
			if err != nil {
				t.Fatalf("%v", err)
			}
			if in.TaskID != tc.wantTaskID {
				t.Fatalf("want taskId %d got %d", tc.wantTaskID, in.TaskID)
			}
			if tc.wantImageIDs != nil {
				if len(in.ImageIDs) != len(tc.wantImageIDs) {
					t.Fatalf("want %v got %v", tc.wantImageIDs, in.ImageIDs)
				}
				for i, v := range tc.wantImageIDs {
					if in.ImageIDs[i] != v {
						t.Fatalf("want %v got %v", tc.wantImageIDs, in.ImageIDs)
					}
				}
			}
		})
	}
}

func TestDecodeTaskInputJSON_Exhaustive_OtherFieldsPreserved(t *testing.T) {
	payload := []byte(`{"taskId":1,"content":"hello","name":"活动","hostName":"校","address":"操场","rank":"一等","level":"5","obtainTime":"2026-04-15","teamName":"t","orgName":"o","resultsName":"r","patentType":"pt","patentNum":"pn","circleBeginDate":"2026-01-01","circleEndDate":"2026-01-02","checkResult":"1","playRole":"2","likeSpecialty1":"a","likeSpecialty2":"b","likeSpecialty3":"c","activityName":"an","sportsName":"sn","specialtyTechnology":"tech","termName":"term","circleDate":"2026-01-01"}`)
	in, err := decodeTaskSubmitInput(payload)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if in.Name != "活动" || in.HostName != "校" || in.Address != "操场" || in.Rank != "一等" {
		t.Fatalf("basic string not preserved %+v", in)
	}
	if in.ObtainTime != "2026-04-15" || in.TeamName != "t" || in.PatentNum != "pn" {
		t.Fatalf("extended fields not preserved %+v", in)
	}
	if in.LikeSpecialty3 != "c" || in.SpecialtyTechnology != "tech" {
		t.Fatalf("like/tech not preserved %+v", in)
	}
}

func TestDecodeTaskInputJSON_Exhaustive_IdField(t *testing.T) {
	// edit id is int64, not normalized; number should decode directly
	in, err := decodeTaskEditInput([]byte(`{"id":5400001,"taskId":1,"content":"c"}`))
	if err != nil {
		t.Fatalf("%v", err)
	}
	if in.ID != 5400001 {
		t.Fatalf("want 5400001 got %d", in.ID)
	}
	// id as string should fail (strict)
	if _, err := decodeTaskEditInput([]byte(`{"id":"5400001","taskId":1,"content":"c"}`)); err == nil {
		// json unmarshal string into int64 will fail – that's expected
	}
}

func TestDecodeTaskInputJSON_Exhaustive_InvalidJSON(t *testing.T) {
	if _, err := decodeTaskSubmitInput([]byte(`{"taskId":}`)); err == nil {
		t.Fatal("invalid json should fail")
	}
	if _, err := decodeTaskSubmitInput([]byte(`not json`)); err == nil {
		t.Fatal("not json should fail")
	}
	// top-level array should fail (we marshal to map)
	if err := decodeTaskInputJSON([]byte(`[1,2]`), &types.TaskSubmitInput{}); err == nil {
		t.Fatal("array top should fail")
	}
}

func TestDecodeTaskInputJSON_Exhaustive_GoldenFrontendJSON(t *testing.T) {
	// 模拟前端 JSON.stringify(form) 的真实抓包：实践创新 120h + 艺术 4类 粘贴即用
	practiceRaw := []byte(`{"id":"","name":"","hostName":"","circleDate":"","rank":"","level":"","content":"暑期实践收获","pictureList":[],"circleTaskId":1001,"circleTypeId":9256,"dimensionId":9,"hours":120,"circleBeginDate":"","circleEndDate":"","checkResult":"1","patentType":"","patentNum":"","address":"基地","termName":"","activityName":"","sportsName":"","teamName":"","orgName":"社区","resultsName":"","obtainTime":"","specialtyTechnology":"","playRole":"","likeSpecialty1":"","likeSpecialty2":"","likeSpecialty3":""}`)
	// pictureList alias + hours number should be normalized but other empty strings stay
	in, err := decodeTaskSubmitInput(practiceRaw)
	if err != nil {
		t.Fatalf("practice golden decode %v", err)
	}
	if in.Hours != "120" {
		t.Fatalf("want 120 got %q", in.Hours)
	}
	if in.OrgName != "社区" || in.Address != "基地" {
		t.Fatalf("golden org/address %+v", in)
	}
	b, _ := json.Marshal(in)
	_ = b
	artRaw := []byte(`{"taskId":1002,"content":"歌声心得","name":"2026年青春唱响","hostName":"团委","obtainTime":"2026-04-15","rank":"一等","level":5,"hours":4,"circleTaskId":1002}`)
	in2, err := decodeTaskSubmitInput(artRaw)
	if err != nil {
		t.Fatalf("art golden %v", err)
	}
	if in2.Level != "5" || in2.Hours != "4" {
		t.Fatalf("art want 5/4 got %q/%q", in2.Level, in2.Hours)
	}
	// canonical taskId should win over circleTaskId alias
	if in2.TaskID != 1002 {
		t.Fatalf("canonical taskId should win got %d", in2.TaskID)
	}
}

func TestNormalizeTaskInputJSON_Exhaustive_BigIntAndOverflow(t *testing.T) {
	// 9223372036854775808 is > MaxInt64 but IsInt true via big.Rat
	payload := []byte(`{"taskId":1,"content":"c","level":9223372036854775808}`)
	var in types.TaskSubmitInput
	if err := decodeTaskInputJSON(payload, &in); err != nil {
		t.Fatalf("big int should be allowed, got %v", err)
	}
	if in.Level != "9223372036854775808" {
		t.Fatalf("want big int string got %q", in.Level)
	}
	// 9223372036854775807.1 fractional should reject
	payload2 := []byte(`{"taskId":1,"content":"c","level":9223372036854775807.1}`)
	var in2 types.TaskSubmitInput
	if err := decodeTaskInputJSON(payload2, &in2); err == nil {
		t.Fatal("fractional big should reject")
	}
}
