package types

import (
	"encoding/json"
	"testing"
)

// TestTask_NeedPicFromUpPic 平台只发 upPic number，不发 needPic。
func TestTask_NeedPicFromUpPic(t *testing.T) {
	raw := `{"id":1,"name":"t","typeName":"x","hours":0.5,"score":1,"circleTaskStatus":"上传期 未提交","upPic":1,"creationTime":[2026,1,1]}`
	var task Task
	if err := json.Unmarshal([]byte(raw), &task); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if task.NeedPic {
		t.Fatal("解码后 NeedPic 应为 false（平台无 needPic 键）")
	}
	if task.UpPic != 1 {
		t.Fatalf("UpPic=%d", task.UpPic)
	}
	task.SetNeedPicFromUpPic()
	if !task.NeedPic {
		t.Fatal("SetNeedPicFromUpPic 后 NeedPic 应为 true")
	}
	task.UpPic = 0
	task.SetNeedPicFromUpPic()
	if task.NeedPic {
		t.Fatal("UpPic=0 时期望 NeedPic=false")
	}
}
