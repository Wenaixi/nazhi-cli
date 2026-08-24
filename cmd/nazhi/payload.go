package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

const maxPayloadSize = 16 << 20

// parsePayloadFromArg 解析命令行 payload 参数，支持 @file.json 和 -（stdin）语法。
// 这是 task_submit.go 和 honor.go 中公共的 payload 读取逻辑抽取。
func parsePayloadFromArg(raw string) ([]byte, error) {
	if strings.HasPrefix(raw, "@") {
		return os.ReadFile(raw[1:])
	}
	if raw == "-" {
		payload, err := io.ReadAll(io.LimitReader(os.Stdin, maxPayloadSize+1))
		if err != nil {
			return nil, err
		}
		if len(payload) > maxPayloadSize {
			return nil, fmt.Errorf("stdin payload 超过 16 MiB 上限")
		}
		return payload, nil
	}
	return []byte(raw), nil
}

// parseJSONObjectPayload 读取并校验对象型 JSON payload。
// 文件、stdin 和内联 JSON 的读取语义保持由 parsePayloadFromArg 负责。
func parseJSONObjectPayload(raw string) ([]byte, error) {
	payload, err := parsePayloadFromArg(raw)
	if err != nil {
		return nil, err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(payload, &object); err != nil {
		return nil, err
	}
	if object == nil {
		return nil, fmt.Errorf("顶层 JSON 必须为对象")
	}
	return payload, nil
}
