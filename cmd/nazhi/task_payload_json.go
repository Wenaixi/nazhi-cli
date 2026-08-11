package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

var taskInputNumericStringFields = [...]string{
	"hours",
	"level",
	"checkResult",
	"playRole",
}

var taskInputFieldAliases = [...]struct {
	canonical string
	alias     string
}{
	{canonical: "taskId", alias: "circleTaskId"},
	{canonical: "imageIDs", alias: "pictureList"},
}

// decodeTaskInputJSON 解码 CLI 写实 payload，并兼容前端编辑回填的数字字段与提交字段别名。
// 归一化只属于 CLI 输入边界；SDK 公开的 Task*Input 仍保持普通 Go 字段语义。
func decodeTaskInputJSON(data []byte, target any) error {
	normalized, err := normalizeTaskInputJSON(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(normalized, target)
}

func normalizeTaskInputJSON(data []byte) ([]byte, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}

	for _, field := range taskInputFieldAliases {
		if raw, ok := findTaskInputField(fields, field.canonical); ok {
			setTaskInputField(fields, field.canonical, raw)
			continue
		}
		if raw, ok := findTaskInputField(fields, field.alias); ok {
			setTaskInputField(fields, field.canonical, raw)
		}
	}

	for _, name := range taskInputNumericStringFields {
		raw, ok := findTaskInputField(fields, name)
		if !ok {
			continue
		}
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 || raw[0] == '"' || bytes.Equal(raw, []byte("null")) {
			continue
		}

		var number json.Number
		if err := json.Unmarshal(raw, &number); err != nil {
			return nil, fmt.Errorf("%s: 期望字符串或数字: %w", name, err)
		}
		encoded, err := json.Marshal(number.String())
		if err != nil {
			return nil, fmt.Errorf("%s: 数字转字符串失败: %w", name, err)
		}
		setTaskInputField(fields, name, encoded)
	}

	return json.Marshal(fields)
}

func findTaskInputField(fields map[string]json.RawMessage, name string) (json.RawMessage, bool) {
	if raw, ok := fields[name]; ok {
		return raw, true
	}
	keys := make([]string, 0, 1)
	for key := range fields {
		if strings.EqualFold(key, name) {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return nil, false
	}
	// 对大小写变体保持确定性；精确 canonical key 已在上方优先处理。
	sort.Strings(keys)
	return fields[keys[0]], true
}

func setTaskInputField(fields map[string]json.RawMessage, name string, value json.RawMessage) {
	for key := range fields {
		if strings.EqualFold(key, name) {
			delete(fields, key)
		}
	}
	fields[name] = value
}

func decodeTaskSubmitInput(data []byte) (types.TaskSubmitInput, error) {
	var input types.TaskSubmitInput
	if err := decodeTaskInputJSON(data, &input); err != nil {
		return input, err
	}
	return input, nil
}

func decodeTaskEditInput(data []byte) (types.TaskEditInput, error) {
	var input types.TaskEditInput
	if err := decodeTaskInputJSON(data, &input); err != nil {
		return input, err
	}
	return input, nil
}
