package types

// PageBean 目标平台通用分页信息。
type PageBean struct {
	PageNo    int `json:"pageNo"`
	PageSize  int `json:"pageSize"`
	TotalNum  int `json:"totalNum"`
	TotalPage int `json:"totalPage"`
}

// CircleRecord 一条已提交的写实记录（来自 getStudentCircle 接口）。
type CircleRecord struct {
	ID           int64         `json:"id"`
	Name         string        `json:"name"`
	Content      string        `json:"content"`
	CircleTaskID int64         `json:"circle_task_id"`
	CircleTypeID int64         `json:"circle_type_id"`
	DimensionID  int64         `json:"dimension_id"`
	TypeName     string        `json:"type_name"`
	Status       int           `json:"status"` // 0=待审, 1=已通过
	CircleDate   string        `json:"circle_date"`
	Hours        float64       `json:"hours"`
	StudentID    int64         `json:"studentId"`
	ClassName    string        `json:"class_name"`
	IfMySelf     int           `json:"ifMySelf"`
	ImgList      []CircleImage `json:"imgList"`
	Remark       string        `json:"remark"`
}

// CircleImage 写实记录关联的图片附件。
type CircleImage struct {
	ID           int64 `json:"id"`
	CircleID     int64 `json:"circle_id"`
	AttachmentID int64 `json:"attachment_id"`
	TaskID       int64 `json:"task_id"`
	ClassID      int64 `json:"class_id"`
}
