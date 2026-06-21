# nazhi-cli

**绾虫櫤缁煎悎璇勪环绯荤粺 鑷姩鍖?CLI + Go SDK**

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/Wenaixi/nazhi-cli)](https://github.com/Wenaixi/nazhi-cli/releases)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![CI](https://img.shields.io/github/actions/workflow/status/Wenaixi/nazhi-cli/ci.yml?branch=main)](https://github.com/Wenaixi/nazhi-cli/actions)

涓€绔欏紡鍛戒护琛屽伐鍏?+ Go SDK锛岀敤浜庣撼鏅虹患鍚堣瘎浠风郴缁熺殑鑷姩鍖栨搷浣溿€傛彁渚?**SSO 鐧诲綍**銆?*浠诲姟绠＄悊**銆?*鑷垜璇勪环**銆?*鏂囦欢涓婁紶** 绛夊畬鏁村姛鑳姐€?
鉁?**鐗硅壊**

- 馃攼 **鍏ㄨ嚜鍔?OCR 楠岃瘉鐮?* 鈥?妯″瀷宸插祵鍏ヤ簩杩涘埗锛屾棤闇€涓嬭浇銆佹棤闇€閰嶇疆锛屽紑绠卞嵆鐢?- 馃實 **璺ㄥ钩鍙版敮鎸?* 鈥?Windows / Linux / macOS锛? 涓钩鍙?脳 鏋舵瀯缁勫悎锛夛紝鍗曚簩杩涘埗杩愯
- 馃摝 **杩涚▼绾?OCR 鍗曚緥** 鈥?澶氬疄渚嬪叡浜紩鎿庯紝閬垮厤閲嶅瑙ｅ帇妯″瀷
- 馃洜锔?**CLI + SDK 鍙屽舰鎬?* 鈥?鑴氭湰鍙洿鎺ヨ皟鐢紝闆嗘垚鏂瑰鍏?Go 鍖?- 馃И **瀹屾暣娴嬭瘯瑕嗙洊** 鈥?鍗曞厓娴嬭瘯 + race 妫€娴?+ 璺ㄥ钩鍙?CI

---

## 鐩綍

- [瀹夎](#瀹夎)
- [蹇€熷紑濮媇(#蹇€熷紑濮?
- [鍛戒护鍙傝€僝(#鍛戒护鍙傝€?
- [璺ㄥ钩鍙版敮鎸乚(#璺ㄥ钩鍙版敮鎸?
- [浣滀负 Go SDK 浣跨敤](#浣滀负-go-sdk-浣跨敤)
- [鏋舵瀯鎬昏](#鏋舵瀯鎬昏)
- [寮€鍙慮(#寮€鍙?
- [甯歌闂](#甯歌闂)
- [鍗忚](#鍗忚)

---

## 瀹夎

### 棰勭紪璇戜簩杩涘埗锛堟帹鑽愶級

浠?[Releases](https://github.com/Wenaixi/nazhi-cli/releases) 涓嬭浇瀵瑰簲骞冲彴鐨勬渶鏂扮増鏈細

| 骞冲彴 | 鏋舵瀯 | 鏂囦欢 |
|---|---|---|
| Windows | amd64 / arm64 | `nazhi-windows-amd64.exe` / `nazhi-windows-arm64.exe` |
| Linux | amd64 / arm64 | `nazhi-linux-amd64` / `nazhi-linux-arm64` |
| macOS | arm64 (Apple Silicon) | `nazhi-darwin-arm64` |

> macOS 浠呮敮鎸?arm64锛圓pple Silicon锛夛紝鍥犱负 Microsoft 宸插仠鍙?onnxruntime macOS x86_64銆?
### go install

```bash
go install github.com/Wenaixi/nazhi-cli/cmd/nazhi@latest
```

### 浠庢簮鐮佹瀯寤?
```bash
git clone https://github.com/Wenaixi/nazhi-cli.git
cd nazhi-cli
make build         # 褰撳墠骞冲彴
make release       # 鍏ㄥ钩鍙?```

---

## 蹇€熷紑濮?
### 1. 鐧诲綍鑾峰彇 Token

```bash
nazhi login -u S1234567890 -p testpass123
```

杈撳嚭锛圝SON 鏍煎紡锛夛細

```json
{
  "token": "eyJhbGciOiJIUzUxMiJ9.eyJzdWIiOiJHMzUwMTgxMjAwOTEyMTEwMDM1...",
  "refresh_after": "...",
  "expires_at": "...",
  "user_info": null
}
```

> 馃敀 **瀹夊叏鎻愮ず**锛歍oken 绛夊悓浜庡瘑鐮侊紝璇峰Ε鍠勪繚绠°€傝剼鏈腑鍙娇鐢?`--token` 鍙傛暟鎴栦粠鐜鍙橀噺浼犲叆銆?
### 2. 婵€娲讳笟鍔?Session锛堜娇鐢?Token锛?
```bash
nazhi session activate --token "eyJhbGciOiJIUzUxMiJ9..."
```

> Session 婵€娲诲悗浼氫繚鎸佹湇鍔＄鐘舵€侊紙Cookie锛夛紝鍚庣画 API 璋冪敤闇€甯?`--token`銆?
### 3. 涓氬姟鎿嶄綔

```bash
# 鏌ョ湅涓汉淇℃伅
nazhi whoami --token "eyJ..."

# 鍒楀嚭鎵€鏈変换鍔?nazhi task list --token "eyJ..."

# 鎻愪氦浠诲姟锛堜粠 JSON 瀛楃涓诧級
nazhi task submit --token "eyJ..." --payload '{"circleTaskId":1001,"name":"鐝細"}'

# 鎻愪氦浠诲姟锛堜粠鏂囦欢锛?nazhi task submit --token "eyJ..." --payload @task.json

# 鎻愪氦鑷垜璇勪环
nazhi self-eval submit --token "eyJ..." --comment "寰堝ソ鐨勫鏈?

# 鏌ョ湅璇勪环鐘舵€?nazhi self-eval status --token "eyJ..."

# 涓婁紶鍥剧墖锛堢敤浜庝换鍔￠檮浠讹級
nazhi file upload -f ./photo.jpg
```

### 鍏ㄥ眬閫夐」

鎵€鏈夊懡浠ゆ敮鎸侊細

| 鏍囧織 | 璇存槑 |
|---|---|
| `-v, --verbose` | 璇︾粏鏃ュ織杈撳嚭鍒?stderr |
| `--quiet` | 闈欓粯妯″紡 |
| `--output json` | 杈撳嚭鏍煎紡锛堥粯璁?JSON锛?|

---

## 鍛戒护鍙傝€?
```
nazhi
鈹溾攢鈹€ login                          SSO 鐧诲綍锛堝叏鑷姩 OCR锛?鈹?  鈹溾攢鈹€ -u/--username       蹇呭～   瀛﹀彿
鈹?  鈹溾攢鈹€ -p/--password       蹇呭～   瀵嗙爜
鈹?  鈹溾攢鈹€ --sso-base          閫夊～   SSO 鏍瑰湴鍧€锛堥粯璁?https://www.nazhisoft.com锛?鈹?  鈹斺攢鈹€ --timeout           閫夊～   HTTP 瓒呮椂绉掓暟锛堥粯璁?15锛?鈹?鈹溾攢鈹€ school                          鏌ヨ瀛︽牎 ID锛堜笉闇€鐧诲綍锛?鈹?  鈹斺攢鈹€ -u/--username       蹇呭～   瀛﹀彿
鈹?鈹溾攢鈹€ session
鈹?  鈹斺攢鈹€ activate                    婵€娲讳笟鍔?Session
鈹?      鈹斺攢鈹€ --token          蹇呭～   X-Auth-Token
鈹?鈹溾攢鈹€ whoami                          鑾峰彇褰撳墠鐢ㄦ埛淇℃伅
鈹?  鈹斺攢鈹€ --token            蹇呭～
鈹?鈹溾攢鈹€ task
鈹?  鈹溾攢鈹€ list                        鍒楀嚭鍏ㄧ淮搴︿换鍔?鈹?  鈹?  鈹斺攢鈹€ --token        蹇呭～
鈹?  鈹斺攢鈹€ submit                      鎻愪氦浠诲姟
鈹?      鈹溾攢鈹€ --token        蹇呭～
鈹?      鈹斺攢鈹€ --payload      蹇呭～     JSON 瀛楃涓叉垨 @file.json
鈹?鈹溾攢鈹€ self-eval
鈹?  鈹溾攢鈹€ submit                      鎻愪氦鑷垜璇勪环
鈹?  鈹?  鈹溾攢鈹€ --token        蹇呭～
鈹?  鈹?  鈹斺攢鈹€ --comment      蹇呭～     鏀寔 stdin: -
鈹?  鈹斺攢鈹€ status                      鏌ヨ璇勪环鐘舵€?鈹?      鈹斺攢鈹€ --token        蹇呭～
鈹?鈹斺攢鈹€ file
    鈹斺攢鈹€ upload                      涓婁紶鍥剧墖
        鈹斺攢鈹€ -f/--file       蹇呭～   鏈湴鍥剧墖璺緞
```

### 杈撳嚭鏍煎紡

鎴愬姛鏃惰緭鍑?JSON 鍒?stdout锛?
```json
{ "code": 1, "msg": "鎴愬姛", "data": [...] }
```

澶辫触鏃惰緭鍑?JSON 鍒?stderr 骞堕€€鍑虹爜 1锛?
```json
{ "error": true, "message": "鍏蜂綋閿欒淇℃伅" }
```

鍙€氳繃 `--quiet` 灞忚斀鎵€鏈?stderr 杈撳嚭锛屼究浜庤剼鏈閬撳鐞嗐€?
---

## 璺ㄥ钩鍙版敮鎸?
| 骞冲彴 | 鏋舵瀯 | 鐘舵€?| 澶囨敞 |
|---|---|---|---|
| **Windows** | amd64 | 鉁?| 涓诲姏娴嬭瘯骞冲彴 |
| **Windows** | arm64 | 鉁?| Windows on ARM |
| **Linux** | amd64 | 鉁?| 鏈嶅姟鍣ㄤ富娴?|
| **Linux** | arm64 | 鉁?| ARM 鏈嶅姟鍣?/ Raspberry Pi |
| **macOS** | arm64 | 鉁?| Apple Silicon |
| macOS | x86_64 | 鉂?| Microsoft 宸插仠鍙?onnxruntime |

### OCR 鍘熺敓搴撳垎鍙?
姣忎釜骞冲彴鎼哄甫瀵瑰簲 onnxruntime 搴擄紙C 寮曟搸锛夛紝閫氳繃 Go build tag 闅旂宓屽叆锛?
```
internal/ocr/
鈹溾攢鈹€ onnx_win_amd64.go   //go:build windows && amd64
鈹溾攢鈹€ onnx_win_arm64.go   //go:build windows && arm64
鈹溾攢鈹€ onnx_lin_amd64.go   //go:build linux && amd64
鈹溾攢鈹€ onnx_lin_arm64.go   //go:build linux && arm64
鈹斺攢鈹€ onnx_mac_arm64.go   //go:build darwin && arm64
```

缂栬瘧鏃跺彧宓屽叆褰撳墠骞冲彴閭ｄ唤锛堢害 15-37 MB锛夛紝鎵€浠?Windows amd64 浜岃繘鍒朵笉浼氬甫 macOS 鐨?dylib銆?
### OCR 杩涚▼绾у崟渚?
`internal/ocr.GetDefault()` 杩涚▼鍏变韩涓€涓?OCR 寮曟搸锛?
- 澶氫釜 `client.New()` 鍏变韩鍚屼竴 `*OCR` 瀹炰緥
- 妯″瀷鍙В鍘嬩竴娆★紙绾?14 MB 鈫?涓存椂鐩綍锛?- 鍐呴儴 `sync.Mutex` 淇濊瘉骞跺彂瀹夊叏
- 99 娆￠噸璇曟満鍒讹紙鍚屼竴鍥剧墖锛夋彁楂樿瘑鍒噯纭巼

### CI 鐭╅樀

`onnxruntime_go` 鍦?Linux/macOS 寮哄埗 CGO锛屾棤娉曚粠鍏朵粬 OS 浜ゅ弶缂栬瘧銆侰I 姣忎釜骞冲彴鐢?native runner锛?
- `ubuntu-latest` / `ubuntu-22.04-arm64` 缂栬瘧 Linux锛圕GO=1锛?- `macos-latest` 缂栬瘧 macOS锛圕GO=1锛?- `windows-latest` / `windows-11-arm` 缂栬瘧 Windows锛圕GO=0锛?
---

## 浣滀负 Go SDK 浣跨敤

### 蹇€熶笂鎵?
```go
import (
    "context"
    "log"
    "github.com/Wenaixi/nazhi-cli/pkg/client"
    "github.com/Wenaixi/nazhi-cli/pkg/types"
)

func main() {
    // 鍒涘缓瀹㈡埛绔紙OCR 榛樿鍚敤銆佽繘绋嬬骇鍗曚緥锛?    c := client.New(
        client.WithSSOBase("https://www.nazhisoft.com"),
        client.WithTimeout(15 * time.Second),
    )

    // 1. 鐧诲綍
    resp, err := c.Login(context.Background(), types.LoginRequest{
        Username: "S1234567890",
        Password: "testpass123",
    })
    if err != nil {
        log.Fatal(err)
    }
    token := resp.Token

    // 2. 婵€娲?Session
    if _, err := c.ActivateSession(context.Background(), token); err != nil {
        log.Fatal(err)
    }

    // 3. 涓氬姟鎿嶄綔
    tasks, err := c.FetchTasks(context.Background(), token)
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("鍏?%d 涓换鍔?, len(tasks))

    // 鎻愪氦浠诲姟
    result, err := c.SubmitTask(context.Background(), token, types.TaskSubmitPayload{
        CircleTaskID: 1001,
        Name:         "鐝細",
        // ... 鍏朵粬 28 涓瓧娈?    })
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("鎻愪氦缁撴灉: code=%d", result.Code)

    // 鑷垜璇勪环
    if err := c.SubmitSelfEvaluation(context.Background(), token, "寰堝ソ鐨勫鏈?); err != nil {
        log.Fatal(err)
    }

    // 涓婁紶鍥剧墖
    imageID, err := c.UploadFile(context.Background(), "./photo.jpg")
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("鍥剧墖 ID: %d", imageID)
}
```

### 杩涢樁閫夐」

```go
// 鑷畾涔?HTTP 瀹㈡埛绔?c := client.New(
    client.WithHTTPClient(&http.Client{
        Timeout: 30 * time.Second,
        Transport: &http.Transport{
            MaxIdleConns:    100,
            IdleConnTimeout: 60 * time.Second,
        },
    }),
    client.WithLogger(slog.Default()),
)

// 淇敼 SSO 鏍瑰湴鍧€锛堢敤浜庡紑鍙?娴嬭瘯鐜锛?c := client.New(
    client.WithSSOBase("http://localhost:8080"),
)
```

### 閿欒澶勭悊

SDK 閫氳繃 `errors.Is` 鍒ゆ柇閿欒绫诲瀷锛?
```go
import "errors"

_, err := c.Login(ctx, req)
switch {
case errors.Is(err, client.ErrLoginRejected):
    // 瀛﹀彿/瀵嗙爜閿欒
case errors.Is(err, client.ErrTokenExpired):
    // Token 杩囨湡锛岄渶閲嶆柊鐧诲綍
case errors.Is(err, client.ErrNetwork):
    // 缃戠粶闂锛堣秴鏃?鏂繛锛?}
```

瀹屾暣閿欒鍒楄〃瑙?`pkg/client/errors.go`銆?
### 绾跨▼瀹夊叏

`Client` 瀹炰緥鏄嚎绋嬪畨鍏ㄧ殑锛?
- 鐙珛 cookie jar锛堟瘡涓?Client 闅旂锛?- 鐙珛 HTTP 杩炴帴姹?- 鍏变韩杩涚▼绾?OCR 寮曟搸锛坄ocr.GetDefault()`锛?
鍙互鍦ㄥ涓?goroutine 涓苟鍙戣皟鐢ㄥ悓涓€ Client銆?
---

## 鏋舵瀯鎬昏

```
nazhi-cli
鈹溾攢鈹€ cmd/nazhi/          鈫?CLI 灞傦紙cobra 鍛戒护锛?鈹?  鈹溾攢鈹€ login.go
鈹?  鈹溾攢鈹€ school.go
鈹?  鈹溾攢鈹€ session.go
鈹?  鈹溾攢鈹€ task_*.go
鈹?  鈹溾攢鈹€ self_eval_*.go
鈹?  鈹溾攢鈹€ file_upload.go
鈹?  鈹斺攢鈹€ output.go       鈫?缁熶竴 JSON 杈撳嚭 + 閿欒澶勭悊
鈹?鈹溾攢鈹€ pkg/                鈫?鍏紑 SDK
鈹?  鈹溾攢鈹€ client/         鈫?Client + Option 妯″紡
鈹?  鈹?  鈹溾攢鈹€ auth.go           SSO 鐧诲綍
鈹?  鈹?  鈹溾攢鈹€ session.go        Session 婵€娲?鈹?  鈹?  鈹溾攢鈹€ task.go           浠诲姟 CRUD
鈹?  鈹?  鈹溾攢鈹€ self_eval.go      鑷垜璇勪环
鈹?  鈹?  鈹溾攢鈹€ user.go           鐢ㄦ埛淇℃伅
鈹?  鈹?  鈹溾攢鈹€ file.go           鏂囦欢涓婁紶
鈹?  鈹?  鈹溾攢鈹€ client.go         Client 缁撴瀯浣?+ Option
鈹?  鈹?  鈹溾攢鈹€ request.go        HTTP 瀹㈡埛绔皝瑁?鈹?  鈹?  鈹斺攢鈹€ errors.go         鍝ㄥ叺閿欒
鈹?  鈹斺攢鈹€ types/          鈫?璇锋眰/鍝嶅簲绫诲瀷
鈹?鈹斺攢鈹€ internal/           鈫?鍐呴儴鍖咃紙鏈粨搴撲笓鐢級
    鈹溾攢鈹€ ocr/            鈫?ddddocr + onnxruntime 灏佽
    鈹?  鈹溾攢鈹€ ocr.go           OCR 鏈嶅姟锛堝崟渚?+ 骞冲彴鍒嗗彂锛?    鈹?  鈹溾攢鈹€ onnx_*.go        build tag 闅旂鐨勫師鐢熷簱 embed
    鈹?  鈹斺攢鈹€ models/          妯″瀷 + 瀛楃闆?+ 5 骞冲彴 onnxruntime
    鈹斺攢鈹€ version/        鈫?鐗堟湰鍙?```

璇︾粏鏋舵瀯璇存槑瑙?[CLAUDE.md](./CLAUDE.md)銆?
---

## 寮€鍙?
### 甯哥敤鍛戒护

```bash
# 鏋勫缓锛堝綋鍓嶅钩鍙帮級
make build

# 娴嬭瘯锛堝惈 race 妫€娴嬶級
make test               # 闈欓粯
make test-verbose       # 璇︾粏杈撳嚭

# 浠ｇ爜璐ㄩ噺
make lint               # golangci-lint
make vet                # go vet
make fmt                # gofmt

# 璺ㄥ钩鍙版瀯寤?make build-linux        # 浜ゅ弶缂栬瘧 Linux amd64
make build-darwin       # 浜ゅ弶缂栬瘧 macOS arm64
make build-windows      # 浜ゅ弶缂栬瘧 Windows amd64
make release            # 鍏ㄥ钩鍙板彂甯?
# 娓呯悊
make clean              # 娓呯悊 bin/ 绛?```

### 椤圭洰瑕佹眰

- Go 1.26+
- Windows / Linux / macOS

### 娴嬭瘯

```bash
# 鍏ㄩ噺鍗曟祴
go test -race -count=1 ./...

# 浠?SDK 娴嬭瘯
go test -race -count=1 ./pkg/client/...

# 璇︾粏杈撳嚭
go test -race -count=1 -v ./...
```

### 璐＄尞

娆㈣繋 PR锛佹祦绋嬭 [CONTRIBUTING.md](./CONTRIBUTING.md)銆傛彁浜ゅ墠璇风‘淇濓細

- `make test` 閫氳繃
- `make lint` 閫氳繃
- 鎻愪氦淇℃伅閬靛惊 Conventional Commits

---

## 甯歌闂

### Q: Windows 涓婅鏉€姣掕蒋浠惰鎶ワ紵

A: 鍐呭祵鐨?`onnxruntime.dll` 鏄?Microsoft 瀹樻柟浜岃繘鍒讹紝閮ㄥ垎鏉€杞細璇姤銆傝繖鏄?go-ddddocr + onnxruntime 鐢熸€佺殑閫氱敤闂銆傚缓璁湪鐧藉悕鍗曚腑娣诲姞鏈▼搴忥紝鎴栧湪浼佷笟鍐呯綉鐜浣跨敤銆?
### Q: macOS x86_64 浣曟椂鏀寔锛?
A: Microsoft onnxruntime v1.25.0 宸插仠姝㈠彂甯?macOS x86_64 鐗堟湰锛圓pple 鍏ㄩ潰杞悜 Silicon锛夈€傛湰椤圭洰涓嶆墦绠楁敮鎸併€傚闇€ Intel Mac 璇蜂娇鐢?v1.20.x 鐨?onnxruntime锛堥渶鑷 fork OCR 搴擄級銆?
### Q: 鑳藉惁瀹屽叏绂荤嚎杩愯锛堜笉鑱旂綉锛夛紵

A: 鉁?鍙互銆侽CR 妯″瀷鍦ㄧ紪璇戞椂閫氳繃 `//go:embed` 宓屽叆浜岃繘鍒讹紝杩愯鏃朵粎璁块棶 `www.nazhisoft.com`锛圫SO 鏈嶅姟鍣級銆?
### Q: 鐧诲綍澶辫触鎻愮ず"楠岃瘉鐮佹牎楠屽け璐?锛?
A: OCR 瀵瑰悓涓€寮犻獙璇佺爜鏈€澶氶噸璇?99 娆★紙绁炵粡缃戠粶鏈韩鏈夐殢鏈烘€э級銆傚鎸佺画澶辫触锛?- 妫€鏌ヨ处鍙峰瘑鐮佹槸鍚︽纭?- 閲嶈瘯 2-3 娆★紙楠岃瘉鐮佷細鍒锋柊锛?- 鍙兘鏄湇鍔＄闄愬埗锛岀◢绛夊嚑鍒嗛挓

### Q: 濡備綍鍦?CI 涓皟鐢?nazhi锛?
A: 鐩存帴涓嬭浇 release 浜岃繘鍒跺埌 runner锛岃皟鐢ㄥ嵆鍙細

```yaml
- name: Login
  run: ./nazhi login -u ${{ secrets.USERNAME }} -p ${{ secrets.PASSWORD }} > token.json
- name: Get token
  id: gettoken
  run: echo "::set-output name=token::$(jq -r .token token.json)"
```

### Q: SDK 鏄惁鑳戒綔涓哄簱琚閮?Go 椤圭洰 import锛?
A: 鉁?鍙互銆俙pkg/client` 鍜?`pkg/types` 鏄叕寮€鍖咃細

```go
import (
    "github.com/Wenaixi/nazhi-cli/pkg/client"
    "github.com/Wenaixi/nazhi-cli/pkg/types"
)
```

`internal/ocr` 鏄唴閮ㄥ寘锛屽閮ㄩ」鐩棤娉曞鍏ワ紝浣嗕笉褰卞搷 SDK 浣跨敤锛圤CR 閫氳繃 `GetDefault()` 鑷姩鍒濆鍖栵級銆?
### Q: Linux ARM64 (Raspberry Pi) 鑳藉惁杩愯锛?
A: 鉁?鍙互锛屽凡鍦?CI 楠岃瘉銆傛敞鎰?onnxruntime ARM64 浜岃繘鍒堕渶瑕?ARMv8-A 鏋舵瀯锛堟爲鑾撴淳 4+锛夈€?
### Q: 濡備綍璋冭瘯 HTTP 璇锋眰锛?
A: 浣跨敤 `--verbose` 鏍囧織鍙湅鍒拌缁嗘棩蹇楋細

```bash
nazhi login -u xxx -p xxx -v
```

杈撳嚭鍖呭惈璇锋眰鏂规硶銆乁RL銆佺姸鎬佺爜銆佽€楁椂绛夈€?
---

## 鍗忚

[MIT License](./LICENSE) 鈥?璇﹁ LICENSE 鏂囦欢銆?
---

*Built with 鉂わ笍 for 绾虫櫤缁煎悎璇勪环绯荤粺鑷姩鍖?

