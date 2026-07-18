package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// UpdateMyInfo 更新个人信息。
//
// 对应前端：modifyBox.vue → submit
// API: POST /api/studentInfo/updateMyInfo
func (c *Client) UpdateMyInfo(ctx context.Context, token string, userInfo types.UserInfo) error {
	_, err := c.doBizAndDecode(ctx, token, "UpdateMyInfo", "/api/studentInfo/updateMyInfo", http.MethodPost, userInfo)
	if err != nil {
		return fmt.Errorf("UpdateMyInfo 失败: %w", err)
	}
	return nil
}
