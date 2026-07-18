package client

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// NotificationListResult 通知列表分页结果。
type NotificationListResult struct {
	Records []map[string]any `json:"records"`
	Page    *types.PageBean  `json:"page,omitempty"`
}

// GetUnreadNotificationsCount 获取未读通知数量。
// GET /api/uiAnnouncement/queryUnreadNotificationByTeacher
func (c *Client) GetUnreadNotificationsCount(ctx context.Context, token string) (int, error) {
	resp, err := c.doBizAndDecode(ctx, token, "GetUnreadNotificationsCount",
		"/api/uiAnnouncement/queryUnreadNotificationByTeacher", http.MethodGet, nil)
	if err != nil {
		return 0, fmt.Errorf("GetUnreadNotificationsCount 失败: %w", err)
	}
	records, err := types.DecodeDataList[map[string]any](*resp)
	if err != nil {
		return 0, nil
	}
	return len(records), nil
}

// GetNotificationByID 获取通知详情。
// GET /api/uiAnnouncement/getAnnouncementById?notificationId=
func (c *Client) GetNotificationByID(ctx context.Context, token string, id int64) (*map[string]any, error) {
	path := "/api/uiAnnouncement/getAnnouncementById?notificationId=" + strconv.FormatInt(id, 10)
	resp, err := c.doBizAndDecode(ctx, token, "GetNotificationByID", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetNotificationByID 失败: %w", err)
	}
	records, err := types.DecodeDataList[map[string]any](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetNotificationByID 解析失败: %w", err)
	}
	if len(records) == 0 {
		return nil, nil
	}
	return &records[0], nil
}

// ReadNotification 标记通知为已读。
// GET /api/uiAnnouncement/readUnreadNotificationByTeacher?notificationId=
func (c *Client) ReadNotification(ctx context.Context, token string, id int64) error {
	path := "/api/uiAnnouncement/readUnreadNotificationByTeacher?notificationId=" + strconv.FormatInt(id, 10)
	_, err := c.doBizAndDecode(ctx, token, "ReadNotification", path, http.MethodGet, nil)
	if err != nil {
		return fmt.Errorf("ReadNotification 失败: %w", err)
	}
	return nil
}

// GetAllNotifications 获取全部通知（分页）。
// GET /api/uiAnnouncement/queryAllNotificationByTeacher?pageNo=&pageSize=
func (c *Client) GetAllNotifications(ctx context.Context, token string, pageNo, pageSize int) (*NotificationListResult, error) {
	path := "/api/uiAnnouncement/queryAllNotificationByTeacher?pageNo=" + strconv.Itoa(pageNo) + "&pageSize=" + strconv.Itoa(pageSize)
	resp, err := c.doBizAndDecode(ctx, token, "GetAllNotifications", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetAllNotifications 失败: %w", err)
	}
	pb, err := types.DecodePageBean(*resp)
	if err != nil {
		return nil, fmt.Errorf("GetAllNotifications 解析分页信息失败: %w", err)
	}
	records, err := types.DecodeDataList[map[string]any](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetAllNotifications 解析通知列表失败: %w", err)
	}
	return &NotificationListResult{Records: records, Page: pb}, nil
}
