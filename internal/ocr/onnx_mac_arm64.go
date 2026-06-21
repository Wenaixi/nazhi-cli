//go:build darwin && arm64

package ocr

import _ "embed"

//go:embed models/libonnxruntime_mac_arm64.dylib
var OnnxRuntimeDLL []byte
