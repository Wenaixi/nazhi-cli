package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

func mockExhaustive(t *testing.T, metaJSON string, cap *types.TaskAddCirclePayload) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{\"code\":1,\"msg\":\"成功\"}"))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{\"code\":1,\"msg\":\"成功\",\"returnData\":{\"name\":\"张三\",\"schoolName\":\"测试中学\"}}"))
		case "/api/studentCircleNew/getCircleTypeByTaskId":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(metaJSON))
		case "/api/studentCircleNew/addCircle":
			_ = json.NewDecoder(r.Body).Decode(cap)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{\"code\":1,\"msg\":\"提交成功\"}"))
		case "/api/studentCircleNew/editCircle":
			_ = json.NewDecoder(r.Body).Decode(cap)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{\"code\":1,\"msg\":\"修改成功\"}"))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
}

func metaEx(t *testing.T, hours float64, typeVal int, remark string) string {
	t.Helper()
	m := map[string]any{
		"code": 1, "msg": "ok",
		"dataMap": map[string]any{
			"task_name": "t", "circle_type_id": int64(9256), "hours": hours,
			"type_name": "x", "dimension_id": int64(9), "dimension_name": "y",
			"task_id": int64(1001), "remark": remark, "type": typeVal,
		},
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func TestParseHours_Exhaustive(t *testing.T) {
	cases := []struct {
		name, user string
		meta       float64
		target     int
		wantErr    bool
		wantVal    float64
	}{
		{"meta pos empty required", "", 2, 1, false, 2},
		{"meta pos empty non-required", "", 2, 2, false, 2},
		{"meta zero empty required 1", "", 0, 1, true, 0},
		{"meta zero empty required 6", "", 0, 6, true, 0},
		{"meta zero empty required 10", "", 0, 10, true, 0},
		{"meta zero empty non-required 2", "", 0, 2, false, 0},
		{"meta zero empty non-required 4", "", 0, 4, false, 0},
		{"meta zero empty non-required 7", "", 0, 7, false, 0},
		{"meta zero empty non-required 9", "", 0, 9, false, 0},
		{"meta negative empty required", "", -1, 1, true, 0},
		{"meta negative empty non-required", "", -1, 2, false, 0},
		{"whitespace uses meta", "   ", 3.5, 6, false, 3.5},
		{"whitespace required still error", "   ", 0, 6, true, 0},
		{"explicit zero", "0", 0, 6, false, 0},
		{"explicit negative", "-1", 1, 1, false, -1},
		{"explicit scientific", "1e2", 1, 1, false, 100},
		{"invalid abc", "abc", 1, 1, true, 0},
		{"invalid NaN", "NaN", 1, 1, true, 0},
		{"invalid Inf", "Inf", 1, 1, true, 0},
		{"invalid 1e999", "1e999", 1, 1, true, 0},
		{"valid 120", "120", 0, 6, false, 120},
		{"valid 0.5", "0.5", 0, 2, false, 0.5},
		{"unknown target zero allowed", "", 0, 99, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseHours(tc.user, tc.meta, tc.target)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want err got %v", got)
				}
				if !errors.Is(err, ErrInvalidPayload) {
					t.Fatalf("want ErrInvalidPayload got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected %v", err)
			}
			if got != tc.wantVal {
				t.Fatalf("want %v got %v", tc.wantVal, got)
			}
		})
	}
}

func TestBuildTaskPayload_30KeysAlwaysPresentExhaustive(t *testing.T) {
	var cap types.TaskAddCirclePayload
	srv := mockExhaustive(t, metaEx(t, 5, 2, ""), &cap)
	defer srv.Close()
	c, _ := New(WithBaseURL(srv.URL), WithSSOBase(srv.URL), WithTimeout(5*1000*1000*1000))
	defer c.Close()
	_, err := c.SubmitTask(t.Context(), "tok", types.TaskSubmitInput{TaskID: 1001, Content: "c"})
	if err != nil {
		t.Fatalf("30 keys %v", err)
	}
	b, _ := json.Marshal(cap)
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%v", err)
	}
	required := []string{"name", "hostName", "circleDate", "rank", "level", "content", "pictureList", "circleTaskId", "circleTypeId", "dimensionId", "hours", "circleBeginDate", "circleEndDate", "checkResult", "patentType", "patentNum", "address", "termName", "activityName", "sportsName", "teamName", "orgName", "resultsName", "obtainTime", "specialtyTechnology", "playRole", "likeSpecialty1", "likeSpecialty2", "likeSpecialty3"}
	for _, k := range required {
		if _, ok := m[k]; !ok {
			t.Fatalf("missing %s in %s", k, string(b))
		}
	}
	if _, ok := m["id"]; ok {
		t.Fatalf("submit should not have id")
	}
}

func TestBuildTaskPayload_All14_TemplateFieldsCarriedExhaustive(t *testing.T) {
	cases := []struct {
		name    string
		typeVal int
		hours   float64
		in      types.TaskSubmitInput
		want    map[string]string
	}{
		{"1", 1, 0, types.TaskSubmitInput{TaskID: 1001, Content: "c", Name: "act", Address: "loc", Hours: "2", PlayRole: "1"}, map[string]string{"name": "act", "playRole": "1"}},
		{"4-art", 4, 0, types.TaskSubmitInput{TaskID: 1001, Content: "c", Name: "sing", HostName: "org", ObtainTime: "2026-04-15", Rank: "A", Level: "5", Hours: "1"}, map[string]string{"level": "5"}},
		{"6-practice", 6, 0, types.TaskSubmitInput{TaskID: 1001, Content: "c", OrgName: "comm", Address: "base", Hours: "120", CheckResult: "2"}, map[string]string{"checkResult": "2"}},
		{"9-like", 9, 1, types.TaskSubmitInput{TaskID: 1001, Content: "c", LikeSpecialty1: "ball"}, map[string]string{"likeSpecialty1": "ball"}},
		{"12-tech", 12, 1, types.TaskSubmitInput{TaskID: 1001, Content: "c", SpecialtyTechnology: "tech"}, map[string]string{"specialtyTechnology": "tech"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var cap types.TaskAddCirclePayload
			srv := mockExhaustive(t, metaEx(t, tc.hours, tc.typeVal, ""), &cap)
			defer srv.Close()
			c, _ := New(WithBaseURL(srv.URL), WithSSOBase(srv.URL), WithTimeout(5*1000*1000*1000))
			defer c.Close()
			_, err := c.SubmitTask(t.Context(), "tok", tc.in)
			if err != nil {
				t.Fatalf("%s %v", tc.name, err)
			}
			b, _ := json.Marshal(cap)
			var m map[string]any
			_ = json.Unmarshal(b, &m)
			for k, want := range tc.want {
				if fmt.Sprint(m[k]) != want {
					t.Fatalf("%s %s want %q got %v", tc.name, k, want, m[k])
				}
			}
		})
	}
}

func TestBuildTaskPayload_GetCircleTypeFailureExhaustive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.WriteHeader(200)
			_, _ = w.Write([]byte("{\"code\":1,\"msg\":\"ok\"}"))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(200)
			_, _ = w.Write([]byte("{\"code\":1,\"msg\":\"ok\",\"returnData\":{\"name\":\"a\"}}"))
		case "/api/studentCircleNew/getCircleTypeByTaskId":
			w.WriteHeader(200)
			_, _ = w.Write([]byte("{\"code\":500,\"msg\":\"not found\"}"))
		default:
			w.WriteHeader(500)
		}
	}))
	defer srv.Close()
	c, _ := New(WithBaseURL(srv.URL), WithSSOBase(srv.URL), WithTimeout(5*1000*1000*1000))
	defer c.Close()
	_, err := c.SubmitTask(t.Context(), "tok", types.TaskSubmitInput{TaskID: 9999, Content: "c"})
	if err == nil {
		t.Fatal("should fail")
	}
	if !strings.Contains(err.Error(), "SubmitTask") {
		t.Fatalf("should contain caller %v", err)
	}
}
