package types

import "time"

// PageBean 目标平台通用分页信息。
type PageBean struct {
	PageNo    int `json:"pageNo"`
	PageSize  int `json:"pageSize"`
	TotalNum  int `json:"totalNum"`
	TotalPage int `json:"totalPage"`
}

// CircleRecord 一条已提交的写实记录（来自 getStudentCircle 接口）。
type CircleRecord struct {
	ID             int64         `json:"id"`             // 写实记录主键
	Name           string        `json:"name"`           // 写实标题
	Content        string        `json:"content"`        // 写实正文
	TypeName       string        `json:"typeName"`       // 类型名（替代原 type_name）
	Approved       bool          `json:"approved"`       // 是否已通过审核（true=已通过，替代原 int status）
	CircleDate     time.Time     `json:"circleDate"`     // 写实发生日期（替代原 circle_date 字符串）
	Hours          float64       `json:"hours"`          // 实践时长（小时）
	ImgList        []CircleImage `json:"imgList"`        // 关联图片附件列表
	ImgPreViewList []string      `json:"imgPreViewList"` // 图片预览 URL 列表（服务端直接返回的可访问链接）
	Remark         string        `json:"remark"`         // 备注
}

// CircleImage 写实记录关联的图片附件。
type CircleImage struct {
	ID           int64  `json:"id"`           // 图片记录主键
	CircleID     int64  `json:"circle_id"`    // 关联的写实记录 ID
	ClassID      int64  `json:"class_id"`     // 班级 ID
	TaskID       int64  `json:"task_id"`      // 关联的任务 ID
	AttachmentID int64  `json:"attachment_id"` // 附件 ID（用于查询/下载图片）
	ImgPath      string `json:"imgPath"`       // 图片扩展名（如 .jpg）
}
