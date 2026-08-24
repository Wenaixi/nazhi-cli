package main

import (
	"context"
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
		// 与 stdin 路径对齐：@file 也受 16 MiB 上限保护，防止误传大文件撑爆内存
		f, err := os.Open(raw[1:])
		if err != nil {
			return nil, err
		}
		defer f.Close()
		payload, err := io.ReadAll(io.LimitReader(f, maxPayloadSize+1))
		if err != nil {
			return nil, err
		}
		if len(payload) > maxPayloadSize {
			return nil, fmt.Errorf("文件 payload 超过 16 MiB 上限")
		}
		return payload, nil
	}
	if raw == "-" {
		// 与 self-eval submit 的 stdin 保护对齐：交互终端下先给提示符，
		// 再走带超时的读取——避免手滑写 - 时无提示无超时地永久阻塞。
		printPrompt("请输入 payload JSON（Ctrl+D 结束）: ")
		content, readErr := readStdinWithTimeout(context.Background(), 60)
		if readErr != nil {
			return nil, readErr
		}
		payload := []byte(content)
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
