package types

import "time"

// Notification 通知消息（来自 queryUnreadNotificationByTeacher / queryAllNotificationByTeacher 接口）。
type Notification struct {
	ID          int64     `json:"id"`          // 通知 ID
	Title       string    `json:"title"`       // 通知标题
	Content     string    `json:"content"`     // 通知内容
	Type        int       `json:"type"`        // 通知类型
	Status      int       `json:"status"`      // 阅读状态（0=未读, 1=已读）
	CreateTime  time.Time `json:"createTime"`  // 创建时间
	CreatorName string    `json:"creatorName"` // 创建人
}

// NotificationListResult 是通知查询的统一返回对象。
type NotificationListResult struct {
	Records []Notification `json:"records"`
	Page    *PageBean      `json:"page,omitempty"`
}
