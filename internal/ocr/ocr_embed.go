//go:build ddddocr_embed

package ocr

import _ "embed"

//go:embed models/common_old.onnx
var modelOnnx []byte

//go:embed models/charsets_old.json
var charsetJSON []byte
