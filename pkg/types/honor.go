package types

// HonorType 一种可申报的荣誉类型（来自 getHonorType 接口）。
type HonorType struct {
	ID            int64  `json:"id"`             // 荣誉类型 ID
	Name          string `json:"name"`           // 荣誉名称
	LevelName     string `json:"level_name"`     // 级别名（校/区县/市/省/国家）
	Level         int    `json:"level"`          // 级别代码（5=校, 4=区县, 3=市, 2=省, 1=国家）
	Score         string `json:"score"`          // 分数说明（如"分数：+5.0"）
	DimensionID   int64  `json:"dimension_id"`   // 所属维度 ID
	DimensionName string `json:"dimension_name"` // 维度名
	SortNo        int    `json:"sort_no"`        // 排序号
}

// HonorRecord 一条已申报的荣誉记录（来自 getHonorByStudentId 接口）。
type HonorRecord struct {
	ID               int64   `json:"id"`                // 荣誉记录 ID
	TypeName         string  `json:"type_name"`         // 荣誉类型名
	TypeID           int64   `json:"type_id"`           // 荣誉类型 ID
	Level            int     `json:"level"`             // 级别代码
	LevelName        string  `json:"level_name"`        // 级别名
	DimensionID      int64   `json:"dimension_id"`      // 维度 ID
	DimensionName    string  `json:"dimension_name"`    // 维度名
	Score            float64 `json:"score"`             // 分数
	Status           int     `json:"status"`            // 0=未审核, 1=审核通过
	StatusName       string  `json:"statusName"`        // 状态名
	StudentName      string  `json:"student_name"`      // 学生姓名
	ClassName        string  `json:"class_name"`        // 班级名
	GetDate          string  `json:"get_date"`          // 获得日期
	EvaluationAgency string  `json:"evaluation_agency"` // 颁发机构
	IfShow           string  `json:"ifshow"`            // 是否展示
	AuditorName      string  `json:"auditor_name"`      // 审核人
	ShowReportFlag   int     `json:"show_report_flag"`  // 是否在报告中展示
}

// AddHonorPayload 是 addHonor 接口的请求体。
type AddHonorPayload struct {
	Name                string `json:"name"`                // 荣誉名称
	TypeID              int64  `json:"typeId"`              // 荣誉类型 ID
	TypeName            string `json:"typeName"`            // 荣誉类型名
	Level               int    `json:"level"`               // 级别代码
	EvaluationAgency    string `json:"evaluationAgency"`    // 颁发机构
	GetDate             string `json:"getDate"`             // 获得日期（YYYY-MM-DD）
	CertImgAttachmentID string `json:"certImgAttachmentId"` // 证书图片附件 ID 或空
}

// HonorSelectOption 是下拉选择选项（label/value 对）。
type HonorSelectOption struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}
