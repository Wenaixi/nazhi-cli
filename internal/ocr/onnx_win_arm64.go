//go:build windows && arm64

package ocr

import _ "embed"

//go:embed models/onnxruntime_win_arm64.dll
var OnnxRuntimeDLL []byte
