package main

import (
	"io"
	"os"
	"strings"
)

// parsePayloadFromArg 解析命令行 payload 参数，支持 @file.json 和 -（stdin）语法。
// 这是 task_submit.go 和 honor.go 中公共的 payload 读取逻辑抽取。
func parsePayloadFromArg(raw string) ([]byte, error) {
	if strings.HasPrefix(raw, "@") {
		return os.ReadFile(raw[1:])
	}
	if raw == "-" {
		return io.ReadAll(io.LimitReader(os.Stdin, 16<<20))
	}
	return []byte(raw), nil
}
