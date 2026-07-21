# 文件域

公开文件服务器上传/下载，**不接业务 token**。对应 `pkg/client/file.go`。

## 方法一览

| 方法 | 说明 | CLI |
|------|------|-----|
| `UploadFile` | 上传图片（预处理为 JPG≤5MB） | `nazhi file upload` |
| `DownloadFile` | 按附件 id 下载到本地 | `nazhi file download` |

## 使用方法

```go
c, err := client.New(client.WithUploadURL("http://doc.nazhisoft.com"))
// Upload 不读业务 token；独立 clean client，不带 Cookie/Authorization
res, err := c.UploadFile(ctx, "./photo.png")
fmt.Println(res.AttachmentID, res.AttachmentName)

err = c.DownloadFile(ctx, res.AttachmentID, "./out.jpg")
```

---

## UploadFile

### 签名

```go
func (c *Client) UploadFile(ctx context.Context, filePath string) (*types.UploadFileResult, error)
```

### 用户输入 vs SDK 自动

| 用户 | SDK 自动 |
|------|----------|
| 本地文件路径 | 解码 → 白底扁平化 → 缩放 → JPEG；目标体积 ≤5MB |
| — | multipart 字段名与 `bussinessType=12&groupName=other`（拼写与平台一致） |
| — | 上传文件名 = `basename(path)` 改 `.jpg`（不含目录） |
| — | **剥离** Authorization / X-Auth-Token / Cookie（独立 clean client） |

### 请求示例

```go
r, err := c.UploadFile(ctx, "./shot.png")
// r.AttachmentID 供写实 pictureList / 荣誉 CertImgAttachmentID / 典型案例 attachmentId
```

### 响应示例

```json
{
  "attachmentId": 5139876,
  "attachmentName": "shot.jpg",
  "url": ""
}
```

### 错误 / 注意

- `ErrFileTooLarge` / `ErrImageTooLarge`：压缩后仍超限  
- `ErrUploadRejected`：服务端拒绝  
- 写实 `SubmitTask` 的 `ImagePaths` 会在内部自动调 Upload；荣誉/典型案例证书需调用方先 Upload 再填 id  
- 总表：[autofill.md](./autofill.md)  

---

## DownloadFile

```go
func (c *Client) DownloadFile(ctx context.Context, attachmentID int64, dst string) error
```

跟随 302；写到 `dst` 路径。成功 `nil`。

## 相关类型

- `types.UploadFileResult`  
