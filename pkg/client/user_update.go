package client

import (
	"context"
	"fmt"
	"net/http"
)

// UpdateMyInfo 更新当前用户个人信息。
// POST /api/studentInfo/updateMyInfo
// updates 是 map，只传需要修改的字段，如：
//
//	{"telephone": "138xxx", "familyAddress": "福建省福州市", "hobbies": "阅读"}
//
// 可用字段参考 types.UserInfo 中的 json tag 名。
func (c *Client) UpdateMyInfo(ctx context.Context, token string, updates map[string]any) error {
	_, err := c.doBizAndDecode(ctx, token, "UpdateMyInfo",
		"/api/studentInfo/updateMyInfo", http.MethodPost, updates)
	if err != nil {
		return fmt.Errorf("UpdateMyInfo 失败: %w", err)
	}
	return nil
}
