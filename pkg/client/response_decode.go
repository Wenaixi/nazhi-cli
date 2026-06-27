package client

import "encoding/json" // 用于 parseRawData（auth.go 中 buildLoginResponse 也调用）

// parseRawData 将原始 JSON 字节解析为 map，用于注入 Raw 字段。
func parseRawData(data []byte) map[string]any {
	if len(data) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	return m
}

// tryDecodeFallback 按顺序尝试多个解码器，返回第一个成功解码的结果。
// 全部失败时返回 nil。
// 日志行为：
//   - 解码器返回 err → 通过 c.logDebug 输出（定位响应结构变化）
//   - 解码器返回 (nil, nil) → 字段为空，静默尝试下一个（不含日志噪音）
//
// 用法示例：
//
//	v := tryDecodeFallback(c, "QuerySelfEvaluation",
//	    func() (*T, error) { return types.DecodeReturnData[T](resp) },
//	    func() (*T, error) { return types.DecodeDataMap[T](resp) },
//	)
func tryDecodeFallback[T any](c *Client, opName string, decoders ...func() (*T, error)) *T {
	for _, decode := range decoders {
		v, err := decode()
		if err == nil {
			if v != nil {
				return v
			}
			// 字段为空（nil result），静默尝试下一个
			continue
		}
		// 解析失败，记录日志但不中断
		c.logDebug("%s fallback 失败: %v", opName, err)
	}
	return nil
}
