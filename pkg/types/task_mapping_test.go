package types

import (
	"encoding/json"
	"testing"
)

func TestTaskRefreshSubmitted(t *testing.T) {
	var task Task
	if err := json.Unmarshal([]byte(`{"circleTaskStatus":"上传期 已提交","upPic":1}`), &task); err != nil {
		t.Fatal(err)
	}

	task.RefreshSubmitted()
	if !task.Submitted || !task.NeedPic {
		t.Fatalf("映射结果错误：Submitted=%v NeedPic=%v", task.Submitted, task.NeedPic)
	}
}
