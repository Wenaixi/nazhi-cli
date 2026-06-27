package client

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

// tryDecodeWithRaw 是 tryDecodeFallback 的增强版，解码成功后自动将原始 JSON
// 字节对应的 map 注入目标类型的 Raw 字段。
//
// 每个 decoder 返回 (*T, []byte, error) 三元组，其中 []byte 是解码所使用的
// 原始 JSON 字段字节（如 *resp.ReturnData 或 *resp.DataMap）。
// setRaw 负责将 parseRawData(raw) 的结果赋值给目标类型的 Raw 字段。
//
// 消除 user.go 中 tryDecodeFallback 每个 decoder 分支重复的
//
//	if err == nil && u != nil { u.Raw = parseRawData(*resp.XXX) }
//
// 模式。
//
// 用法示例：
//
//	v := tryDecodeWithRaw(c, "GetMyInfo",
//	    func(u *types.UserInfo, raw map[string]any) { u.Raw = raw },
//	    func() (*types.UserInfo, []byte, error) {
//	        return types.DecodeReturnData[types.UserInfo](resp)
//	    },
//	    func() (*types.UserInfo, []byte, error) {
//	        return types.DecodeDataMap[types.UserInfo](resp)
//	    },
//	)
func tryDecodeWithRaw[T any](c *Client, opName string, setRaw func(*T, map[string]any),
	decoders ...func() (*T, []byte, error)) *T {

	for _, decode := range decoders {
		v, rawBytes, err := decode()
		if err == nil {
			if v != nil && setRaw != nil && len(rawBytes) > 0 {
				setRaw(v, parseRawData(rawBytes))
			}
			return v
		}
		c.logDebug("%s fallback 失败: %v", opName, err)
	}
	return nil
}
