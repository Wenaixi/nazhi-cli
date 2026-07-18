package client

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// GetUnreadNotifications 查询未读通知。
//
// 对应前端：header.vue → queryUnreadNotificationByTeacher
// API: GET /api/uiAnnouncement/queryUnreadNotificationByTeacher
func (c *Client) GetUnreadNotifications(ctx context.Context, token string) ([]types.Notification, error) {
	resp, err := c.doBizAndDecode(ctx, token, "GetUnreadNotifications", "/api/uiAnnouncement/queryUnreadNotificationByTeacher", http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetUnreadNotifications 失败: %w", err)
	}

	notifications, err := types.DecodeDataList[types.Notification](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetUnreadNotifications 解析通知列表失败: %w", err)
	}

	return notifications, nil
}

// GetNotificationByID 查看通知详情。
//
// 对应前端：NewnoticeBOX.vue → getAnnouncementById
// API: GET /api/uiAnnouncement/getAnnouncementById?notificationId={id}
func (c *Client) GetNotificationByID(ctx context.Context, token string, notificationID int64) (*types.Notification, error) {
	path := "/api/uiAnnouncement/getAnnouncementById?notificationId=" + strconv.FormatInt(notificationID, 10)
	resp, err := c.doBizAndDecode(ctx, token, "GetNotificationByID", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetNotificationByID 失败: %w", err)
	}

	notification, err := types.DecodeDataList[types.Notification](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetNotificationByID 解析通知详情失败: %w", err)
	}

	if len(notification) == 0 {
		return nil, fmt.Errorf("GetNotificationByID 未找到通知")
	}

	return &notification[0], nil
}

// ReadNotification 标记通知为已读。
//
// 对应前端：NewnoticeBOX.vue → readUnreadNotificationByTeacher
// API: GET /api/uiAnnouncement/readUnreadNotificationByTeacher?notificationId={id}
func (c *Client) ReadNotification(ctx context.Context, token string, notificationID int64) error {
	path := "/api/uiAnnouncement/readUnreadNotificationByTeacher?notificationId=" + strconv.FormatInt(notificationID, 10)
	_, err := c.doBizAndDecode(ctx, token, "ReadNotification", path, http.MethodGet, nil)
	if err != nil {
		return fmt.Errorf("ReadNotification 失败: %w", err)
	}
	return nil
}

// GetAllNotifications 分页查询所有通知。
//
// 对应前端：NewnoticeBOX.vue → queryAllNotificationByTeacher
// API: GET /api/uiAnnouncement/queryAllNotificationByTeacher?pageNo={pageNo}&pageSize={pageSize}
func (c *Client) GetAllNotifications(ctx context.Context, token string, pageNo, pageSize int) ([]types.Notification, *types.PageBean, error) {
	path := "/api/uiAnnouncement/queryAllNotificationByTeacher?pageNo=" + strconv.Itoa(pageNo) + "&pageSize=" + strconv.Itoa(pageSize)
	resp, err := c.doBizAndDecode(ctx, token, "GetAllNotifications", path, http.MethodGet, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("GetAllNotifications 失败: %w", err)
	}

	pb, err := types.DecodePageBean(*resp)
	if err != nil {
		return nil, nil, fmt.Errorf("GetAllNotifications 解析分页信息失败: %w", err)
	}

	notifications, err := types.DecodeDataList[types.Notification](*resp)
	if err != nil {
		return nil, nil, fmt.Errorf("GetAllNotifications 解析通知列表失败: %w", err)
	}

	return notifications, pb, nil
}
