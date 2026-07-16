//go:build windows && amd64 && ddddocr_embed

package ocr

import _ "embed"

//go:embed models/onnxruntime_win_amd64.dll
var OnnxRuntimeDLL []byte
