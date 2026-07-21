package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// PlayRoleCode 是写实列表/表单的承担角色代码："1" 主持策划 / "2" 主要参与 / "3" 参与者。
//
// 提交表单 el-radio label 为字符串；列表 getStudentCircle 的 play_role 前端用
// switch(map.play_role) case 1/2/3 判断，平台常返回 JSON number。本类型两种都收，
// 统一成字符串码，便于与 TaskSubmitInput.PlayRole 及 PlayRole* 常量比较。
type PlayRoleCode string

// String 返回角色码（空表示未填）。
func (p PlayRoleCode) String() string { return string(p) }

// UnmarshalJSON 接受 JSON number、string 或 null。
func (p *PlayRoleCode) UnmarshalJSON(data []byte) error {
	if p == nil {
		return fmt.Errorf("PlayRoleCode: UnmarshalJSON on nil pointer")
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*p = ""
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*p = PlayRoleCode(strings.TrimSpace(s))
		return nil
	}
	// number（含科学计数极少见；用 float64 再 FormatInt 会丢大整数，角色码仅 1–3）
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("PlayRoleCode: 期望 string 或 number，得到 %s: %w", string(data), err)
	}
	// 优先整型字面量
	if i, err := n.Int64(); err == nil {
		*p = PlayRoleCode(strconv.FormatInt(i, 10))
		return nil
	}
	f, err := n.Float64()
	if err != nil {
		return fmt.Errorf("PlayRoleCode: 无法解析 number %q: %w", n.String(), err)
	}
	*p = PlayRoleCode(strconv.FormatInt(int64(f), 10))
	return nil
}

// MarshalJSON 始终输出 JSON 字符串（与提交 playRole 字符串一致）。
func (p PlayRoleCode) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(p))
}
