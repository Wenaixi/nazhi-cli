//go:build linux && amd64 && ddddocr_embed

package ocr

import _ "embed"

//go:embed models/libonnxruntime_lin_amd64.so
var OnnxRuntimeDLL []byte
