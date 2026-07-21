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
| 本地文件路径 | 透明底合成、缩放、JPEG、multipart；`bussinessType=12&groupName=other`；剥离鉴权头 |

### 请求示例

```go
r, err := c.UploadFile(ctx, "./shot.png")
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
- 写实/荣誉/典型案例：可先 Upload 再填 attachmentId / CertImgAttachmentID / pictureList  

---

## DownloadFile

```go
func (c *Client) DownloadFile(ctx context.Context, attachmentID int64, dst string) error
```

跟随 302；写到 `dst` 路径。成功 `nil`。

## 相关类型

- `types.UploadFileResult`  
