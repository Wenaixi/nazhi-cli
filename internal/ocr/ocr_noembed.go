//go:build !ddddocr_embed

package ocr

// 无嵌入数据：modelOnnx/charsetJSON/OnnxRuntimeDLL 均为 nil。
// extractModels 在有 modelDir 时从外部目录加载，无 modelDir 时返回明确错误。

var modelOnnx []byte
var charsetJSON []byte
var OnnxRuntimeDLL []byte
