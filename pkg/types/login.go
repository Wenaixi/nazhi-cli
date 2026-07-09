package types

import "time"

// LoginRequest 是目标平台 SSO 登录请求。
//
// 字段无需 JSON tag，因为本结构只用于 Login() 入参（不参与 HTTP 请求体序列化）。
// HTTP 请求体由 SDK 内部显式构造，避免结构体反射开销。
type LoginRequest struct {
	SchoolID string // 学校 ID（可为空，服务端自学号推断）
	Username string // 学号
	Password string // 密码
}

// LoginResponse 是 SSO 登录成功后的响应。
//
// 字段约定（v1.0.0 精简版）：仅保留登录 token + 过期时间 + OCR 降级标记。
// 用户基本信息请通过 Client.GetMyInfo() 单独获取。
type LoginResponse struct {
	Token        string    `json:"token"        example:"eyJhbGciOiJIUzUxMiJ9..." description:"X-Auth-Token 凭证（后续业务接口必带）"`
	ExpiresAt    time.Time `json:"expiresAt"    example:"2026-07-23T18:38:00+08:00"  description:"token 过期时间（ISO 8601 + 时区）"`
	FallbackUsed bool      `json:"fallbackUsed" example:"false"                       description:"本次登录是否降级到 ddddocr OCR（primary 失败后）"`
}
