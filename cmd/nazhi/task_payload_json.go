package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

var taskInputNumericStringFields = [...]string{
	"hours",
	"level",
	"checkResult",
	"playRole",
}

// taskInputDeprecatedFields 是 TaskSubmitInput/TaskEditInput 中前端表单有键但
// 无 v-model（用户从不手填）的历史兼容字段。CLI 允许传入但 SDK 不消费——
// 与 user update 的 nationalStudentNumber 同理：避免把"旧调用方还在传"误判
// 为"未知键"，否则会误伤历史 payload。
// 前端 form 对照实证：practice 表单 JSON.stringify 恒含 id/""、name/""、hostName/""、
// circleDate/""、rank/""、level/""、termName/""；art 表单含 name；edit 恒注入 id。
var taskInputDeprecatedFields = map[string]struct{}{
	"id": {}, "name": {}, "hostName": {}, "circleDate": {}, "rank": {},
	"level": {}, "termName": {},
}

// taskInputAllowedKeys 是 task submit/edit payload 顶层 JSON 的全部允许键：
// TaskInput 消费的 json 键 + 别名对（circleTaskId/pictureList）+ 历史兼容字段。
// 清晰列出允许集，未知键以参数错误拒绝（对齐 user update）。
// 注意：键集统一存小写（unknownUpdatePayloadKeys 对用户键 ToLower 后比较），
// 大小写变体按 findTaskInputField 的 EqualFold 语义天然允许。
var taskInputAllowedKeys = func() map[string]struct{} {
	allowed := map[string]struct{}{
		// TaskInput 消费的普通字段（TaskAddCirclePayload 出站 json 键全集，统一小写）
		"id": {}, "name": {}, "hostname": {}, "circledate": {}, "rank": {},
		"level": {}, "content": {}, "picturelist": {}, "circletaskid": {},
		"circletypeid": {}, "dimensionid": {}, "hours": {}, "circlebegindate": {},
		"circleenddate": {}, "checkresult": {}, "patenttype": {}, "patentnum": {},
		"address": {}, "termname": {}, "activityname": {}, "sportsname": {},
		"teamname": {}, "orgname": {}, "resultsname": {}, "obtaintime": {},
		"specialtytechnology": {}, "playrole": {}, "likespecialty1": {},
		"likespecialty2": {}, "likespecialty3": {},
		// TaskInput 接口消费但非出站 json 键的输入字段（小写）
		"taskid": {}, "imagepaths": {}, "imageids": {},
	}
	for k := range taskInputDeprecatedFields {
		allowed[strings.ToLower(k)] = struct{}{}
	}
	return allowed
}()

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
		if name != "hours" {
			value, err := strconv.ParseFloat(number.String(), 64)
			if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
				return nil, fmt.Errorf("%s: 数字代码必须是有限整数", name)
			}
			integer, ok := new(big.Rat).SetString(number.String())
			if !ok || !integer.IsInt() {
				return nil, fmt.Errorf("%s: 数字代码必须是有限整数", name)
			}
			number = json.Number(integer.Num().String())
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
