//go:build darwin && arm64 && ddddocr_embed

package ocr

import _ "embed"

//go:embed models/libonnxruntime_mac_arm64.dylib
var OnnxRuntimeDLL []byte
