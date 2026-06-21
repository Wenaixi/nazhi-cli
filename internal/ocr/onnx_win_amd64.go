//go:build windows && amd64

package ocr

import _ "embed"

//go:embed models/onnxruntime_win_amd64.dll
var OnnxRuntimeDLL []byte
