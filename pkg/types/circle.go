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
	ID         int64         `json:"id"`         // 写实记录主键
	Name       string        `json:"name"`       // 写实标题
	Content    string        `json:"content"`    // 写实正文
	TypeName   string        `json:"typeName"`   // 类型名（替代原 type_name）
	Approved   bool          `json:"approved"`   // 是否已通过审核（true=已通过，替代原 int status）
	CircleDate time.Time     `json:"circleDate"` // 写实发生日期（替代原 circle_date 字符串）
	Hours      float64       `json:"hours"`      // 实践时长（小时）
	ImgList    []CircleImage `json:"imgList"`    // 关联图片附件列表
	Remark     string        `json:"remark"`     // 备注
}

// CircleImage 写实记录关联的图片附件。
type CircleImage struct {
	AttachmentID int64 `json:"attachmentId"` // 附件 ID（用于查询/下载图片）
}
