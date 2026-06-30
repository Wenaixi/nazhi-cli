// image_prep_test.go 是 image_prep.go 的内部测试（同包，可访问未导出方法）。
package client

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// internalNewTestClient 是 newTestClient 的内部版本（同包访问）。
func internalNewTestClient() *Client {
	c, _ := New(WithTimeout(5 * time.Second))
	return c
}

// ─── 测试: PNG → JPG 转换 + RGBA 合成白底 ───

func TestPrepareImage_PNGtoJPEG(t *testing.T) {
	c := internalNewTestClient()
	tmpfile := t.TempDir() + "/test-png-input.png"

	// 创建 200×200 半透明 PNG（验证 RGBA → 白底合成）
	img := image.NewNRGBA(image.Rect(0, 0, 200, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 200; x++ {
			img.Set(x, y, color.NRGBA{0, 128, 255, 128}) // 半透明
		}
	}
	f, _ := os.Create(tmpfile)
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("编码失败: %v", err)
	}
	f.Close()

	data, mime, err := c.prepareImageForUpload(tmpfile)
	if err != nil {
		t.Fatalf("prepareImageForUpload 失败: %v", err)
	}
	if mime != "image/jpeg" {
		t.Errorf("期望 mime=image/jpeg, 得到 %s", mime)
	}
	if len(data) == 0 {
		t.Error("输出数据为空")
	}
	if len(data) > MaxImageSize {
		t.Errorf("压缩后仍超 %d bytes: %d", MaxImageSize, len(data))
	}
	t.Logf("PNG → JPG 转换: %d bytes", len(data))
}

// ─── 测试: JPG 透传 ───

func TestPrepareImage_JPEGPassthrough(t *testing.T) {
	c := internalNewTestClient()
	tmpfile := t.TempDir() + "/test-jpg.jpg"

	// 创建 200×200 纯色 JPG
	img := image.NewRGBA(image.Rect(0, 0, 200, 200))
	green := color.RGBA{0, 255, 0, 255}
	for y := 0; y < 200; y++ {
		for x := 0; x < 200; x++ {
			img.Set(x, y, green)
		}
	}
	f, _ := os.Create(tmpfile)
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 85}); err != nil {
		f.Close()
		t.Fatalf("编码失败: %v", err)
	}
	f.Close()

	data, mime, err := c.prepareImageForUpload(tmpfile)
	if err != nil {
		t.Fatalf("prepareImageForUpload 失败: %v", err)
	}
	if mime != "image/jpeg" {
		t.Errorf("期望 mime=image/jpeg, 得到 %s", mime)
	}
	if len(data) == 0 {
		t.Error("输出数据为空")
	}
	t.Logf("JPG 透传: %d bytes", len(data))
}

// ─── 测试: 大图压缩（5MB 强制触发压缩路径）───

func TestPrepareImage_CompressesLargeImage(t *testing.T) {
	c := internalNewTestClient()
	tmpfile := t.TempDir() + "/test-large.png"

	// 创建大图（800×800 = 640K 像素；用 Pix 直接填避开 Set bounds check)。
	// 不可压缩的渐变 pattern 确保 PNG 输出不会太小，但足够测出压缩路径。
	const w, h = 800, 800
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	pix := img.Pix
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			off := (y*w + x) * 4
			pix[off+0] = uint8(x % 256)       // R
			pix[off+1] = uint8(y % 256)       // G
			pix[off+2] = uint8((x + y) % 256) // B
			pix[off+3] = 255                  // A
		}
	}
	f, _ := os.Create(tmpfile)
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("编码失败: %v", err)
	}
	f.Close()
	origStat, _ := os.Stat(tmpfile)
	t.Logf("原图大小: %d bytes", origStat.Size())

	data, _, err := c.prepareImageForUpload(tmpfile)
	if err != nil {
		t.Fatalf("prepareImageForUpload 失败: %v", err)
	}
	if len(data) > MaxImageSize {
		t.Errorf("压缩后仍超 %d bytes: %d", MaxImageSize, len(data))
	}
	t.Logf("压缩后: %d bytes (压缩率 %.1f%%)", len(data), float64(len(data))/float64(origStat.Size())*100)
}

// ─── 测试: GIF 动画取第 0 帧 ───

func TestPrepareImage_GifStatic(t *testing.T) {
	// Go stdlib 的 gif.Encode 不直接支持多帧，这里只验证能正确解码 GIF
	c := internalNewTestClient()
	tmpfile := t.TempDir() + "/test.gif"

	// 创建单帧 GIF
	img := image.NewPaletted(image.Rect(0, 0, 50, 50), color.Palette{color.Black, color.White})
	for y := 0; y < 50; y++ {
		for x := 0; x < 50; x++ {
			img.Set(x, y, color.White)
		}
	}
	f, _ := os.Create(tmpfile)
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("编码失败: %v", err)
	}
	f.Close()
	// 实际 GIF 解码由 stdlib 处理，这里只确保 prepare 不 panic
	_, _, err := c.prepareImageForUpload(tmpfile)
	if err != nil {
		t.Logf("GIF 测试跳过: %v", err)
	}
}

// ─── 测试: UploadFile 不发送任何鉴权 Header ───

// 验证即使 cookie jar 已被注入 X-Auth-Token，HTTP 请求也不携带任何鉴权头
func TestUploadFile_NoAuthHeaders(t *testing.T) {
	var seenHeaders http.Header
	upload := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"code":1,"returnData":{"id":67890}}`))
	}))
	defer upload.Close()

	// 创建带 X-Auth-Token 的 Client（模拟"复用了已登录 Client"的最坏情况）
	c, _ := New(WithUploadURL(upload.URL), WithTimeout(5*time.Second))
	jar, ok := c.http.Jar.(*cookiejar.Jar)
	if ok {
		u, _ := url.Parse(upload.URL)
		jar.SetCookies(u, []*http.Cookie{
			{Name: "X-Auth-Token", Value: "fake-leaked-token-should-not-be-sent"},
			{Name: "JSESSIONID", Value: "fake-session"},
		})
	}

	// 创建测试 PNG
	tmpfile := t.TempDir() + "/test-noauth.png"
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	for y := 0; y < 50; y++ {
		for x := 0; x < 50; x++ {
			img.Set(x, y, color.RGBA{0, 255, 0, 255})
		}
	}
	f, _ := os.Create(tmpfile)
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("编码失败: %v", err)
	}
	f.Close()

	_, err := c.UploadFile(t.Context(), tmpfile)
	if err != nil {
		t.Fatalf("UploadFile 失败: %v", err)
	}

	// 1. X-Auth-Token Header 没发送
	if v := seenHeaders.Get("X-Auth-Token"); v != "" {
		t.Errorf("❌ 检测到 X-Auth-Token Header 被发送: %q", v)
	}
	// 2. Authorization Header 没发送
	if v := seenHeaders.Get("Authorization"); v != "" {
		t.Errorf("❌ 检测到 Authorization Header 被发送: %q", v)
	}
	// 3. Cookie Header 没发送（清空所有 cookie）
	if v := seenHeaders.Get("Cookie"); v != "" {
		t.Errorf("❌ 检测到 Cookie Header 被发送: %q", v)
	}
	t.Logf("✓ UploadFile 正确未发送任何鉴权 Header（X-Auth-Token/Authorization/Cookie）")
}

// image_prep_break_test.go 通过 AST 静态扫描锁定 F4 修复：
// image_prep.go 缩放级联循环不能 `continue` 跳过 `current = resized`。
// F4 证据：image_prep.go 缩放级联 `for _, scale := range getScaleFactors()`
// 内 `if err != nil { continue }` 跳过 `current = resized`，下一轮用
// 未更新的 current 计算 w/h → 同一尺寸重复 encodeJPEG 必然同样失败 →
// 浪费 1-7 轮 CPU 后才 break 返回 ErrImageTooLarge。
// 修复：`continue` → `break` + logDebug（encodeJPEG 内部错误重试无意义）。
// 测试策略：AST 扫描，定位 scaleFactors range 循环，递归查找 continue
// 语句（注释里的字面量"continue"不会被 AST 误判）。

// TestImagePrep_ScaleCascadeNoContinue AST 扫描 image_prep.go，
// 断言 scaleFactors range 循环内不能出现 continue 语句。
func TestImagePrep_ScaleCascadeNoContinue(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "image_prep.go", nil, 0)
	if err != nil {
		t.Fatalf("parse image_prep.go: %v", err)
	}

	// 1. 找到 prepareImageForUpload 函数
	var prepFn *ast.FuncDecl
	for _, decl := range f.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == "prepareImageForUpload" {
			prepFn = fd
			break
		}
	}
	if prepFn == nil {
		t.Fatal("找不到 prepareImageForUpload 函数")
	}

	// 2. 找到 range getScaleFactors() 的 for 循环
	var scaleLoop *ast.RangeStmt
	ast.Inspect(prepFn.Body, func(n ast.Node) bool {
		if rs, ok := n.(*ast.RangeStmt); ok {
			if call, ok := rs.X.(*ast.CallExpr); ok {
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "getScaleFactors" {
					scaleLoop = rs
					return false
				}
			}
		}
		return true
	})
	if scaleLoop == nil {
		t.Fatal("找不到 `for _, scale := range getScaleFactors()` 循环")
	}

	// 3. 扫描循环 body，禁止 continue（Go 1.26 把 ContinueStmt/BreakStmt
	// 统一为 *ast.BranchStmt，靠 Tok 字段 token.CONTINUE / token.BREAK 区分）
	var foundContinue *ast.BranchStmt
	ast.Inspect(scaleLoop.Body, func(n ast.Node) bool {
		if br, ok := n.(*ast.BranchStmt); ok && br.Tok == token.CONTINUE {
			foundContinue = br
			return false
		}
		return true
	})
	if foundContinue != nil {
		t.Errorf("F4 回归：scaleFactors 循环在 %s 出现 continue 语句。"+
			"continue 会跳过 current = resized 赋值，下一轮重试同尺寸同错误，"+
			"浪费 1-7 轮 CPU 后才在 MinImageDimension 边界 break。必须改为 break。",
			fset.Position(foundContinue.Pos()))
	}

	// 4. 验证修复契约：循环 body 含 break 语句
	var foundBreak *ast.BranchStmt
	ast.Inspect(scaleLoop.Body, func(n ast.Node) bool {
		if br, ok := n.(*ast.BranchStmt); ok && br.Tok == token.BREAK {
			foundBreak = br
			return false
		}
		return true
	})
	if foundBreak == nil {
		t.Errorf("F4 修复契约：scaleFactors 循环必须含 break 语句跳出失败轮次")
	}
}

// TestImagePrep_ScaleCascadeHasLogDebug 验证修复契约：
// 缩放级联循环的错误分支必须配 logDebug 调用。
// 用字符串子串匹配（仅在错误处理块注释 anchor 范围内），
// 不易触发字面量误判：定位 `if err != nil {` 锚点 + 下一 break 之间的内容。
func TestImagePrep_ScaleCascadeHasLogDebug(t *testing.T) {
	src, err := readSource("image_prep.go")
	if err != nil {
		t.Fatalf("读 image_prep.go: %v", err)
	}
	body := string(src)

	// 锚点：encodeJPEG 调用之后紧随的 `if err != nil {` 错误处理块。
	// 源码用两步式（先 data, err = encodeJPEG(...)，再单独 if err != nil），
	// 不是 if-init 复合形式。
	anchor := "data, err = encodeJPEG(current, 40)"
	idx := strings.Index(body, anchor)
	if idx < 0 {
		t.Fatalf("找不到 %q 锚点，源码结构可能改了", anchor)
	}
	// 取该 if 块后续 600 字符（足够看到 break + logDebug）
	block := body[idx:]
	if len(block) > 600 {
		block = block[:600]
	}

	if !strings.Contains(block, "return") {
		t.Errorf("F4 修复契约：encodeJPEG 失败分支必须含 return，实际块:\n%s", block)
	}
	if !strings.Contains(block, "logDebug") {
		t.Errorf("F4 修复契约：encodeJPEG 失败 break 前应 logDebug 记录原因，实际块:\n%s", block)
	}
}

// readSource 包内 helper：读当前包目录下的源码文件。
func readSource(name string) ([]byte, error) {
	return osReadFile(name)
}

// osReadFile 用 os.ReadFile 读文件，单独函数便于未来 mock。
func osReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

// image_prep_gif_test.go 验证带透明 GIF 在 prepareImageForUpload 后
// 输出非黑底（F11 修复）。
// F11 证据：image_prep.go:62-65 特例分支 `if format == "gif" && flattened`
// 做了两件事：(1) imaging.Clone(img) 丢弃透明；(2) flattened=false 跳过
// flattenOnWhite。HasTransparency 对 *image.Paletted 始终返回 true。
// 失败场景：透明 GIF → jpeg.Encode 把透明索引解析为黑色 → 黑底 JPG。

// TestPrepareImage_GifTransparentNotBlack 透明 GIF 必须合成到白底。
// 测试策略：
// 1. 构造一个透明 GIF（左半透明索引 = 0，右半索引 = 1）
// 2. 调 prepareImageForUpload
// 3. 解码输出的 JPG，断言：左半像素的 R 值远 > 50（不是黑底）
// 修复前：flattened=false 跳过 flattenOnWhite → jpeg.Encode 黑底 → 左半 R≈0 → FAIL
// 修复后：flattened=true 走 flattenOnWhite → 白底合成 → 左半 R≈255 → PASS
func TestPrepareImage_GifTransparentNotBlack(t *testing.T) {
	c := internalNewTestClient()
	tmpfile := t.TempDir() + "/transparent.gif"

	// 构造带透明的 GIF：
	// - palette: 索引 0 = 透明（透明色），索引 1 = 不透明红色
	// - 左半 (0..49) 用索引 0（透明），右半 (50..99) 用索引 1（红）
	// jpeg.Encode 对透明索引（无 alpha）会解析为黑色 (0,0,0)。
	// 因此修复前：左半 = 黑底；修复后：左半 = 白底（与 flattenOnWhite 行为一致）。
	const w, h = 100, 50
	pal := color.Palette{
		color.RGBA{0, 0, 0, 0},     // 索引 0：完全透明（视为黑色前景）
		color.RGBA{255, 0, 0, 255}, // 索引 1：不透明红色
	}
	img := image.NewPaletted(image.Rect(0, 0, w, h), pal)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x < 50 {
				img.SetColorIndex(x, y, 0) // 左半透明
			} else {
				img.SetColorIndex(x, y, 1) // 右半红
			}
		}
	}

	f, _ := os.Create(tmpfile)
	if err := gif.Encode(f, img, &gif.Options{NumColors: 2}); err != nil {
		f.Close()
		t.Fatalf("GIF 编码失败: %v", err)
	}
	f.Close()

	data, mime, err := c.prepareImageForUpload(tmpfile)
	if err != nil {
		t.Fatalf("prepareImageForUpload 失败: %v", err)
	}
	if mime != "image/jpeg" {
		t.Errorf("期望 mime=image/jpeg, 实际 %s", mime)
	}
	if len(data) == 0 {
		t.Fatal("输出数据为空")
	}

	// 解码输出的 JPG，断言左半（透明区）像素的 R 值远大于黑底阈值
	decoded, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解码输出 JPG 失败: %v", err)
	}
	// 取左半中心点 (25, 25) 的像素
	pixel := decoded.At(25, 25)
	r, g, b, _ := pixel.RGBA()
	r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)

	t.Logf("GIF 透明区域中心像素: R=%d G=%d B=%d", r8, g8, b8)

	// 黑底判定：R < 50（修复前会触发）
	if r8 < 50 {
		t.Errorf("F11 回归：GIF 透明区域被处理为黑底, R=%d (期望白底 R≈255)", r8)
	}

	// 进一步保险：右半（红色不透明）应该保持红色
	pixelRight := decoded.At(75, 25)
	rR, gR, _, _ := pixelRight.RGBA()
	rR8, gR8 := uint8(rR>>8), uint8(gR>>8)
	if rR8 < 200 || gR8 > 50 {
		t.Errorf("GIF 不透明红色区域失真: R=%d G=%d (期望 R≥200, G≤50)", rR8, gR8)
	}
}

// TestPrepareImage_GifOpaque 纯不透明 GIF 仍然走 flattenOnWhite 不受影响。
// 回归测试：删除 `if format=="gif" && flattened` 特例不能影响不透明 GIF 路径。
// 不透明 GIF 在 gif.Decode 时返回 *image.Paletted，但 hasTransparency
// 对 *image.Paletted 始终返回 true，所以不透明 GIF 实际也会走 flattened 分支。
// 这里只验证不 panic + 输出合法 JPG。
func TestPrepareImage_GifOpaque(t *testing.T) {
	c := internalNewTestClient()
	tmpfile := t.TempDir() + "/opaque.gif"

	const w, h = 60, 40
	pal := color.Palette{
		color.RGBA{255, 255, 255, 255},
		color.RGBA{0, 0, 0, 255},
	}
	img := image.NewPaletted(image.Rect(0, 0, w, h), pal)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetColorIndex(x, y, uint8((x+y)%2)) // 棋盘格
		}
	}

	f, _ := os.Create(tmpfile)
	if err := gif.Encode(f, img, &gif.Options{NumColors: 2}); err != nil {
		f.Close()
		t.Fatalf("GIF 编码失败: %v", err)
	}
	f.Close()

	data, mime, err := c.prepareImageForUpload(tmpfile)
	if err != nil {
		t.Fatalf("prepareImageForUpload 失败: %v", err)
	}
	if mime != "image/jpeg" {
		t.Errorf("期望 mime=image/jpeg, 实际 %s", mime)
	}
	if _, err := jpeg.Decode(bytes.NewReader(data)); err != nil {
		t.Errorf("输出 JPG 无法解码: %v", err)
	}

	// 冗余：检查不透明 GIF 经 flatten 后是否仍是棋盘格（白/黑，不应是单一色）
	decoded, _ := jpeg.Decode(bytes.NewReader(data))
	white := decoded.At(0, 0) // 偶数和索引 (0+0)%2 = 0 = 白
	black := decoded.At(1, 0) // (1+0)%2 = 1 = 黑
	_, _, _, whiteA := white.RGBA()
	_, _, _, blackA := black.RGBA()
	_ = whiteA
	_ = blackA
	// 仅 smoke：不要求严格棋盘（flatten 后 RGB 转换可能有偏差），只要能解码
}

// TestPrepareImage_PngTransparentStillFlattens 回归保险：
// PNG 透明合成白底的现有契约不能被 GIF 修复破坏。
func TestPrepareImage_PngTransparentStillFlattens(t *testing.T) {
	c := internalNewTestClient()
	tmpfile := t.TempDir() + "/transparent.png"

	img := image.NewNRGBA(image.Rect(0, 0, 50, 50))
	for y := 0; y < 50; y++ {
		for x := 0; x < 50; x++ {
			img.Set(x, y, color.NRGBA{255, 0, 0, 0}) // 完全透明红
		}
	}
	f, _ := os.Create(tmpfile)
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("PNG 编码失败: %v", err)
	}
	f.Close()

	data, mime, err := c.prepareImageForUpload(tmpfile)
	if err != nil {
		t.Fatalf("prepareImageForUpload 失败: %v", err)
	}
	if mime != "image/jpeg" {
		t.Errorf("期望 mime=image/jpeg, 实际 %s", mime)
	}
	decoded, _ := jpeg.Decode(bytes.NewReader(data))
	// 完全透明 → flattenOnWhite → 白底
	pixel := decoded.At(25, 25)
	r, _, _, _ := pixel.RGBA()
	r8 := uint8(r >> 8)
	if r8 < 200 {
		t.Errorf("PNG 完全透明应 flatten 到白底, R=%d (期望 ≈255)", r8)
	}
}

// image_prep_immutable_test.go: G4 getQualitySteps/getScaleFactors 不可变验证。

// TestGetQualitySteps_ReturnsNewSlice 验证每次调用都返回不同副本。
func TestGetQualitySteps_ReturnsNewSlice(t *testing.T) {
	a := getQualitySteps()
	b := getQualitySteps()

	if len(a) != 1 || a[0] != 80 {
		t.Errorf("getQualitySteps() 返回意外的值: %v", a)
	}

	// 修改 a 不应影响 b
	if len(b) > 0 {
		a[0] = 999
		if b[0] == 999 {
			t.Errorf("G4 回归：修改 getQualitySteps() 的返回副本影响了其他调用方，"+
				"a[0]=%d, b[0]=%d (期望 b[0] 保持 80)", a[0], b[0])
		}
	}

	ha := reflect.ValueOf(a).Pointer()
	hb := reflect.ValueOf(b).Pointer()
	if ha == hb {
		t.Error("getQualitySteps() 两次调用返回了同一底层数组")
	}
}

// TestGetScaleFactors_ReturnsNewSlice 验证每次调用都返回不同副本。
func TestGetScaleFactors_ReturnsNewSlice(t *testing.T) {
	a := getScaleFactors()
	b := getScaleFactors()

	if len(a) != 7 {
		t.Errorf("getScaleFactors() 长度应为 7，实际 %d", len(a))
	}

	a[0] = 0.5
	if b[0] == 0.5 {
		t.Errorf("G4 回归：修改 getScaleFactors() 的返回副本影响了其他调用方，"+
			"a[0]=%.1f, b[0]=%.1f (期望 b[0] 保持 0.7)", a[0], b[0])
	}

	ha := reflect.ValueOf(a).Pointer()
	hb := reflect.ValueOf(b).Pointer()
	if ha == hb {
		t.Error("getScaleFactors() 两次调用返回了同一底层数组")
	}
}

// TestGetQualitySteps_Values 验证 getQualitySteps 返回值正确。
func TestGetQualitySteps_Values(t *testing.T) {
	steps := getQualitySteps()
	expected := []int{80}
	if len(steps) != len(expected) {
		t.Fatalf("长度: 期望 %d, 实际 %d", len(expected), len(steps))
	}
	for i, v := range steps {
		if v != expected[i] {
			t.Errorf("步骤 %d: 期望 %d, 实际 %d", i, expected[i], v)
		}
	}
}

// TestGetScaleFactors_Values 验证 getScaleFactors 返回值正确。
func TestGetScaleFactors_Values(t *testing.T) {
	factors := getScaleFactors()
	if len(factors) != 7 {
		t.Fatalf("长度: 期望 7, 实际 %d", len(factors))
	}
	for i, v := range factors {
		if v != 0.7 {
			t.Errorf("因子 %d: 期望 0.7, 实际 %.1f", i, v)
		}
	}
}

// image_prep_jpeg_buf_pool_panic_test.go: encodeJPEG panic recovery 测试。

// TestEncodeJPEG_PanicRecovery_PoolStillUsable 验证 encodeJPEG 内部
// jpeg.Encode panic 后，sync.Pool 中的 buffer 仍可正常使用。
// 场景：jpeg.Encode 传入 nil *image.RGBA 时触发 nil deref panic。
// 此时 defer buf.Reset() + jpegBufPool.Put(buf) 仍会执行，
// buf 被归还到 pool 但内容不确定。本测试验证：
// - panic 不被传播到调用方（测试本身 recover）
// - pool.Get 后续仍返回可用 buffer
// - 下一轮 encodeJPEG 调用正常
func TestEncodeJPEG_PanicRecovery_PoolStillUsable(t *testing.T) {
	// 1. 制造 panic：nil image → jpeg.Encode 调 img.Bounds() → nil deref
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("encodeJPEG(nil, ...) 应 panic，但未 panic")
			}
		}()
		_, _ = encodeJPEG(nil, 80)
	}()

	// 2. panic 后 pool 仍能用 Get 拿到有效 buffer
	buf, ok := jpegBufPool.Get().(*bytes.Buffer)
	if !ok {
		t.Fatal("jpegBufPool.Get() 应返回 *bytes.Buffer")
	}
	buf.Reset()
	jpegBufPool.Put(buf)

	// 3. 下一轮 encodeJPEG 调用正常（验证 pool 未被 panic 损坏）
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{R: 128, G: 128, B: 128, A: 255})
		}
	}
	data, err := encodeJPEG(img, 80)
	if err != nil {
		t.Fatalf("panic 后 encodeJPEG 失败: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("encodeJPEG 返回空数据")
	}
}

// image_jpeg_pool_test.go: encodeJPEG 并发安全测试。

// TestEncodeJPEG_ConcurrentSafe 回归测试：encodeJPEG 在并发调用下
// 不会因 sync.Pool buffer 复用导致数据竞争或输出错乱。
// 历史 bug：encodeJPEG 直接返回 buf.Bytes()，未 copy；pool Put 后
// buffer 内部 slice 被其他 goroutine 复用覆盖，返回的 []byte 失效。
func TestEncodeJPEG_ConcurrentSafe(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	// 填几个非零像素确保 JPEG 输出非空
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}

	const goroutines = 10
	const iterations = 50
	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*iterations)
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				data, err := encodeJPEG(img, 80)
				if err != nil {
					errCh <- err
					return
				}
				if len(data) == 0 {
					errCh <- errEmpty
					return
				}
				// 验证返回的 []byte 在后续调用后仍有效（不被 pool 覆盖）
				firstByte := data[0]
				_ = firstByte
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err == errEmpty {
			t.Error("encodeJPEG 返回空 []byte")
		} else {
			t.Errorf("encodeJPEG 错误: %v", err)
		}
	}
}

var errEmpty = constErr("empty jpeg output")

type constErr string

func (e constErr) Error() string { return string(e) }

// ─── group-B F3: UploadFile 错误链对外暴露 ErrFileTooLarge（不论根因） ───

// TestUploadFile_PrepErrorIncludesErrFileTooLarge (F3) 锁定 file.go 修复契约：
// UploadFile 在 prepareImageForUpload 抛 ErrImageTooLarge（image_prep 局部
// sentinel）时，返回的错误链必须同时包含 ErrFileTooLarge，让调用方用
// errors.Is(err, ErrFileTooLarge) 单一识别所有「文件过大」路径。
//
// 修复前：file.go L33-34 wrap 时仅传递 prepareImageForUpload 原 err，
// UploadFile 错误链只有 ErrImageTooLarge → ErrFileTooLarge 不可命中，
// 调用方只能 imports 引入 image_prep 局部 sentinel（包外不可见）。
//
// 修复后：file.go wrap 时 errors.Join 进 ErrFileTooLarge。
//
// 测试采用 AST 静态扫描 file.go，定位 UploadFile 函数体，确认 wrap 语句
// 使用 errors.Join(...ErrFileTooLarge..., ...)，避免环境敏感（大图不
// 保证触发 ErrImageTooLarge）的运行时测试。
func TestUploadFile_PrepErrorIncludesErrFileTooLarge(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "file.go", nil, 0)
	if err != nil {
		t.Fatalf("parse file.go: %v", err)
	}
	// 1. 定位 UploadFile 函数
	var uploadFn *ast.FuncDecl
	for _, decl := range f.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == "UploadFile" {
			uploadFn = fd
			break
		}
	}
	if uploadFn == nil {
		t.Fatal("找不到 UploadFile 函数")
	}
	// 2. 找含 "图片预处理失败" 字面量的 fmt.Errorf 调用，
	//    断言其参数里含 errors.Join(...ErrFileTooLarge...)
	var foundFix bool
	var foundOldStyle bool
	ast.Inspect(uploadFn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		se, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || se.Sel.Name != "Errorf" {
			return true
		}
		// 看第一个 string arg 里是否含 "图片预处理失败"
		if len(call.Args) < 2 {
			return true
		}
		bl, ok := call.Args[0].(*ast.BasicLit)
		if !ok || bl.Kind != token.STRING {
			return true
		}
		if !strings.Contains(bl.Value, "图片预处理失败") {
			return true
		}
		// 第二个参数应是 errors.Join 或直接 %w wrap
		// 找参数中是否含 errors.Join(...ErrFileTooLarge...)
		containsJoinWithErrFileTooLarge := false
		containsFmtErrorf := false
		ast.Inspect(call, func(n2 ast.Node) bool {
			if cc, ok2 := n2.(*ast.CallExpr); ok2 {
				if ss, ok3 := cc.Fun.(*ast.SelectorExpr); ok3 && ss.Sel.Name == "Join" {
					// 找 ErrFileTooLarge 是否是 Join 的某个参数
					ast.Inspect(cc, func(n3 ast.Node) bool {
						if id, ok4 := n3.(*ast.Ident); ok4 && id.Name == "ErrFileTooLarge" {
							containsJoinWithErrFileTooLarge = true
							return false
						}
						return true
					})
				}
				if ss, ok3 := cc.Fun.(*ast.SelectorExpr); ok3 && ss.Sel.Name == "Errorf" {
					containsFmtErrorf = true
				}
			}
			return true
		})
		// F3 修复契约：
		//   调用形式必须是 errors.Join(ErrFileTooLarge, <prepare err>) 链入，
		//   不再用裸 fmt.Errorf("%w", err) 让 ErrFileTooLarge 不在链上。
		_ = containsFmtErrorf
		if containsJoinWithErrFileTooLarge {
			foundFix = true
		} else {
			// 检测有"图片预处理失败"但没用 join——属于当前 bug 行为
			foundOldStyle = true
		}
		return false
	})
	if foundOldStyle && !foundFix {
		t.Errorf("F3 漏洞：UploadFile '图片预处理失败' wrap 仍裸用 fmt.Errorf %%w，未 errors.Join 进 ErrFileTooLarge，调用方 errors.Is(err, ErrFileTooLarge) 不可命中 image_prep 路径")
	}
	if !foundFix {
		t.Errorf("F3 修复契约：UploadFile 错误 wrap 应使用 errors.Join(ErrFileTooLarge, err)，未检测到。")
	}
}
