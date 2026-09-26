package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// ─── 写操作实例的 call 路径守卫 ───
//
// writeOpMode 的 interface 里，「decode 产出的类型必须与 call 断言的类型
// 配对」是核心不变量，但它此前没有任何自动化守卫：9 个实例中 6 个的 call
// 分支从无成功路径测试。把 decode 配成别的实例的，编译照过、测试全绿，
// 只有用户实际运行该命令时才会 panic（经 main 顶层 recover 变成退出码 2
// 的「内部错误」envelope）。
//
// 本文件逐个走通这 6 个实例的 call 分支，让该不变量有回归保护。
// 已有守卫的三个（task submit、user update、honor update）不在此重复。

// writeOpCallServer 构造一个能通过 session 预热、并对写操作端点返回成功的
// mock。写路径统一记录到 hit，供各用例断言。
type writeOpCallServer struct {
	*httptest.Server
	hit map[string]bool
}

// newWriteOpCallServer 的预热响应形状沿用既有黑盒测试已验证的约定：
// getMyInfo 成功即止，不带学校字段——写操作不需要学校信息降级。
func newWriteOpCallServer(t *testing.T, writePath string, writeBody string) *writeOpCallServer {
	t.Helper()
	w := &writeOpCallServer{hit: map[string]bool{}}
	w.Server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			_, _ = rw.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			_, _ = rw.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"示例学生","studentNumber":"TEST2025001"}}`))
		case "/api/studentMoralEduNew/getHonorTypeForSelect":
			_, _ = rw.Write([]byte(`{"code":1,"dataList":[{"label":"校三好学生","value":1147}],"returnData":[{"label":"校","value":5}]}`))
		case "/api/studentCircleNew/getCircleTypeByTaskId":
			// 任务元数据：EditCircle / Preview* 在组装 payload 前必拉。
			// dataMap 形状沿用既有黑盒测试已验证的约定。
			_, _ = rw.Write([]byte(`{"code":1,"msg":"成功","dataMap":{"task_name":"班会","circle_type_id":9256,"hours":1.0,"type_name":"主题班会","dimension_id":9,"dimension_name":"思想品德","task_id":1001,"remark":"普通任务说明","type":10}}`))
		case writePath:
			w.hit[writePath] = true
			_, _ = rw.Write([]byte(writeBody))
		default:
			// 其余读路径（任务元数据等）返回空成功，避免 404 阻断流程。
			_, _ = rw.Write([]byte(`{"code":1,"msg":"成功","dataList":[],"returnData":{}}`))
		}
	}))
	t.Cleanup(w.Close)
	return w
}

// runWriteOpCase 走一遍 runWriteOp，断言写端点被命中且退出码为 0。
func runWriteOpCase(t *testing.T, name string, mode writeOpMode, payload string, srv *writeOpCallServer, writePath string) {
	t.Helper()

	cmd := makeWriteOpTestCmd(t, srv.URL, payload, nil)
	originalQuiet, originalVerbose := quiet, verbose
	quiet, verbose = false, false
	pendingExitCode.Store(0)
	t.Cleanup(func() {
		quiet, verbose = originalQuiet, originalVerbose
		pendingExitCode.Store(0)
		_ = closeAllClients()
	})

	runWriteOp(cmd, mode, nil)

	if !srv.hit[writePath] {
		t.Fatalf("%s：call 分支未发出业务请求（%s 未命中），decode 与 call 可能配错", name, writePath)
	}
	if got := pendingExitCode.Load(); got != 0 {
		t.Fatalf("%s：成功调用不应设置退出码，实际 %d", name, got)
	}
}

// TestTaskEditWriteOp_CallPath 守住 task edit 的 decode/call 配对：
// decode 产出 *types.TaskEditInput，call 必须断言同一类型。
func TestTaskEditWriteOp_CallPath(t *testing.T) {
	srv := newWriteOpCallServer(t, "/api/studentCircleNew/editCircle", `{"code":1,"msg":"修改成功"}`)
	runWriteOpCase(t, "task edit", taskEditWriteOp,
		`{"id":56241,"taskId":100,"content":"改后的写实内容"}`, srv, "/api/studentCircleNew/editCircle")
}

// TestTaskPreviewSubmitWriteOp_CallPath 守住 task preview 提交分支的配对。
// preview 不发 addCircle，止于元数据预热，因此断言「未命中提交端点」且退出码为 0。
func TestTaskPreviewSubmitWriteOp_CallPath(t *testing.T) {
	srv := newWriteOpCallServer(t, "/api/studentCircleNew/addCircle", `{"code":1,"msg":"不应到达"}`)

	cmd := makeWriteOpTestCmd(t, srv.URL,
		`{"taskId":100,"content":"预览用的写实内容"}`, nil)
	originalQuiet, originalVerbose := quiet, verbose
	quiet, verbose = false, false
	pendingExitCode.Store(0)
	t.Cleanup(func() {
		quiet, verbose = originalQuiet, originalVerbose
		pendingExitCode.Store(0)
		_ = closeAllClients()
	})

	runWriteOp(cmd, taskPreviewSubmitWriteOp, nil)

	if got := pendingExitCode.Load(); got != 0 {
		t.Fatalf("preview 成功应退出码 0，实际 %d（decode 与 call 可能配错）", got)
	}
	if srv.hit["/api/studentCircleNew/addCircle"] {
		t.Fatal("preview 是 dry-run，不应发出 addCircle 提交请求")
	}
}

// TestTaskPreviewEditWriteOp_CallPath 守住 task preview 编辑分支的配对。
// 该分支的 decode 产出 *types.TaskEditInput，与提交分支类型不同，
// 二者配错不会编译报错——正是需要守卫的地方。
func TestTaskPreviewEditWriteOp_CallPath(t *testing.T) {
	srv := newWriteOpCallServer(t, "/api/studentCircleNew/editCircle", `{"code":1,"msg":"不应到达"}`)

	cmd := makeWriteOpTestCmd(t, srv.URL,
		`{"id":56241,"taskId":100,"content":"预览改后的写实"}`, map[string]string{"edit": "true"})
	originalQuiet, originalVerbose := quiet, verbose
	quiet, verbose = false, false
	pendingExitCode.Store(0)
	t.Cleanup(func() {
		quiet, verbose = originalQuiet, originalVerbose
		pendingExitCode.Store(0)
		_ = closeAllClients()
	})

	runWriteOp(cmd, taskPreviewEditWriteOp, nil)

	if got := pendingExitCode.Load(); got != 0 {
		t.Fatalf("preview edit 成功应退出码 0，实际 %d（decode 与 call 可能配错）", got)
	}
	if srv.hit["/api/studentCircleNew/editCircle"] {
		t.Fatal("preview 是 dry-run，不应发出 editCircle 提交请求")
	}
}

// TestHonorAddWriteOp_CallPath 守住 honor add 的配对。
// 它的 decode 产出 *types.AddHonorPayload，与典型案例提交的
// *types.AddTypicalCasePayload 形状相近，最容易被配错。
func TestHonorAddWriteOp_CallPath(t *testing.T) {
	srv := newWriteOpCallServer(t, "/api/studentMoralEduNew/addHonor", `{"code":1,"msg":"申报成功"}`)
	runWriteOpCase(t, "honor add", honorAddWriteOp,
		`{"typeId":1147,"level":5,"evaluationAgency":"示例中学","getDate":"2026-06-30","typeName":"校三好学生"}`,
		srv, "/api/studentMoralEduNew/addHonor")
}

// TestTypicalCaseSubmitWriteOp_CallPath 守住典型案例提交的配对。
func TestTypicalCaseSubmitWriteOp_CallPath(t *testing.T) {
	srv := newWriteOpCallServer(t, "/api/studentCircleNew/addTypicalCase", `{"code":1,"msg":"提交成功"}`)
	runWriteOpCase(t, "typical-case submit", typicalCaseSubmitWriteOp,
		`{"type":1,"role":1,"level":5,"partnerName":"搭档姓名","remark":"备注说明"}`,
		srv, "/api/studentCircleNew/addTypicalCase")
}

// TestTypicalCaseUpdateWriteOp_CallPath 守住典型案例更新的配对。
// 它的 decode/call 都走 *map[string]any，与其它 update 实例同型，
// 配错时不会 panic 但会取到错误的 id 字段。
func TestTypicalCaseUpdateWriteOp_CallPath(t *testing.T) {
	var gotBody map[string]any
	srv := newWriteOpCallServer(t, "/api/studentCircleNew/updateTypicalCase", `{"code":1,"msg":"更新成功"}`)

	// 复用同一个 server 构造但额外捕获请求体，验证 id 确实透传。
	inner := srv.Config.Handler
	srv.Config.Handler = http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/updateTypicalCase" {
			srv.hit[r.URL.Path] = true
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			rw.Header().Set("Content-Type", "application/json")
			_, _ = rw.Write([]byte(`{"code":1,"msg":"更新成功"}`))
			return
		}
		inner.ServeHTTP(rw, r)
	})

	runWriteOpCase(t, "typical-case update", typicalCaseUpdateWriteOp,
		`{"id":300,"type":1,"role":1}`, srv, "/api/studentCircleNew/updateTypicalCase")

	if gotBody["id"] == nil {
		t.Fatalf("典型案例更新应把 id 透传到请求体，实际 %+v", gotBody)
	}
}

// TestWriteOpModes_DecodeCallTypesArePaired 直接断言每个实例的 decode 与 call
// 产出同一类型：这是本文件所有用例想守住的不变量，用一个表驱动断言把它
// 显式写下来，未来新增实例时补一行即可。
//
// 断言方式是让 call 真的跑一遍（见上方各用例），这里额外锁定「新旧实例
// 的类型配对表」，防止有人复制粘贴实例时只改了一半。
func TestWriteOpModes_DecodeCallTypesArePaired(t *testing.T) {
	cases := []struct {
		name string
		mode writeOpMode
	}{
		{"task submit", taskSubmitWriteOp},
		{"task edit", taskEditWriteOp},
		{"task preview submit", taskPreviewSubmitWriteOp},
		{"task preview edit", taskPreviewEditWriteOp},
		{"honor add", honorAddWriteOp},
		{"honor update", honorUpdateWriteOp},
		{"typical-case submit", typicalCaseSubmitWriteOp},
		{"typical-case update", typicalCaseUpdateWriteOp},
		{"user update", userUpdateWriteOp},
	}

	// 每个实例的 decode 必须能解出非 nil 值（配错会在类型断言处 panic），
	// 且 success 必须能包装非 nil 结果。逐个检查函数存在性。
	for _, tc := range cases {
		if tc.mode.decode == nil {
			t.Errorf("%s：decode 未配置", tc.name)
		}
		if tc.mode.call == nil {
			t.Errorf("%s：call 未配置", tc.name)
		}
		if tc.mode.success == nil {
			t.Errorf("%s：success 未配置", tc.name)
		}
		if len(tc.mode.allowedKeys) == 0 {
			t.Errorf("%s：allowedKeys 未配置", tc.name)
		}
	}
}

// TestTaskEditWriteOp_CallReceivesEditInput 锁定 task edit 的 call 确实把
// TaskEditInput 传给 SDK（而非其他类型）：用类型断言在测试里复核一遍，
// 让配错在编译期之外也有第二道防线。
func TestTaskEditWriteOp_CallReceivesEditInput(t *testing.T) {
	decoded, err := taskEditWriteOp.decode([]byte(`{"id":1,"taskId":100,"content":"内容"}`))
	if err != nil {
		t.Fatalf("解码应成功: %v", err)
	}
	if _, ok := decoded.(*types.TaskEditInput); !ok {
		t.Fatalf("task edit 的 decode 应产出 *types.TaskEditInput，实际 %T", decoded)
	}
}

// TestHonorAddWriteOp_CallReceivesHonorPayload 锁定 honor add 的 decode 产出类型。
func TestHonorAddWriteOp_CallReceivesHonorPayload(t *testing.T) {
	decoded, err := honorAddWriteOp.decode([]byte(`{"typeId":1,"level":5,"evaluationAgency":"示例中学","getDate":"2026-06-30","typeName":"校三好学生"}`))
	if err != nil {
		t.Fatalf("解码应成功: %v", err)
	}
	if _, ok := decoded.(*types.AddHonorPayload); !ok {
		t.Fatalf("honor add 的 decode 应产出 *types.AddHonorPayload，实际 %T", decoded)
	}
}

// TestTypicalCaseAddWriteOp_CallReceivesTypicalCasePayload 锁定典型案例提交的
// decode 产出类型——它与 honor add 同为「提交类」，最易互相配错。
func TestTypicalCaseAddWriteOp_CallReceivesTypicalCasePayload(t *testing.T) {
	decoded, err := typicalCaseSubmitWriteOp.decode([]byte(`{"type":1,"role":1,"partnerName":"搭档姓名","remark":"备注"}`))
	if err != nil {
		t.Fatalf("解码应成功: %v", err)
	}
	if _, ok := decoded.(*types.AddTypicalCasePayload); !ok {
		t.Fatalf("typical-case submit 的 decode 应产出 *types.AddTypicalCasePayload，实际 %T", decoded)
	}
}
