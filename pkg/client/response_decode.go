package client

import "github.com/Wenaixi/nazhi-cli/pkg/types"

// tryDecodeFallback 按顺序尝试多个解码器，返回第一个成功解码的结果。
// 全部失败时返回 nil。
// 日志行为：
//   - 解码器返回 err → 通过 c.logDebug 输出（定位响应结构变化）
//   - 解码器返回 (nil, nil) → 字段为空，静默尝试下一个（不含日志噪音）
//
// 用法示例：
//
//	v := tryDecodeFallback(c, "QuerySelfEvaluation", resp,
//	    func() (*T, error) { return types.DecodeReturnData[T](resp) },
//	    func() (*T, error) { return types.DecodeDataMap[T](resp) },
//	)
func tryDecodeFallback[T any](c *Client, opName string, resp *types.UnifiedResponse, decoders ...func() (*T, error)) *T {
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
