//go:build linux && arm64 && ddddocr_embed

package ocr

import _ "embed"

//go:embed models/libonnxruntime_lin_arm64.so
var OnnxRuntimeDLL []byte
