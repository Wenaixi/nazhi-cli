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

// IntList 解码平台「日期/时间」数组字段：JSON 中为 number 列表，如
// admissionDate=[2025,9,1]、birthday=[2009,12,11]、creationTime=[...]
//
// 历史上 UserInfo.AdmissionDate 曾误标为 []string，真实 getMyInfo 返回 number，
// 导致 Unmarshal 失败并被 fallback 当成「空用户」。
type IntList []int

// UnmarshalJSON 接受 [number...]、[string 数字...]、null、空数组。
func (l *IntList) UnmarshalJSON(data []byte) error {
	if l == nil {
		return fmt.Errorf("IntList: UnmarshalJSON on nil pointer")
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*l = nil
		return nil
	}
	// 优先 number 数组（平台主路径）
	var nums []int
	if err := json.Unmarshal(data, &nums); err == nil {
		*l = IntList(nums)
		return nil
	}
	// 兼容字符串数字数组
	var strs []string
	if err := json.Unmarshal(data, &strs); err != nil {
		return fmt.Errorf("IntList: 期望 number/string 数组，得到 %s: %w", string(data), err)
	}
	out := make([]int, 0, len(strs))
	for _, s := range strs {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("IntList: 元素 %q 不是整数: %w", s, err)
		}
		out = append(out, n)
	}
	*l = IntList(out)
	return nil
}

// MarshalJSON 输出 JSON number 数组（与平台 getMyInfo 一致）。
func (l IntList) MarshalJSON() ([]byte, error) {
	if l == nil {
		return []byte("null"), nil
	}
	return json.Marshal([]int(l))
}
