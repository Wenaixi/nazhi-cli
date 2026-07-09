package types

// Task 是目标平台的一个任务条目。
type Task struct {
	ID            int64   `json:"id"`               // 任务 ID（即 circleTaskId）
	Name          string  `json:"name"`             // 任务名称
	CircleTypeID  int64   `json:"circleTypeId"`     // 圈子类型 ID
	TypeName      string  `json:"typeName"`         // 类型名称
	DimensionID   int64   `json:"dimensionId"`      // 维度 ID
	DimensionName string  `json:"dimensionName"`    // 维度名称
	Hours         float64 `json:"hours"`            // 学时
	Status        string  `json:"circleTaskStatus"` // 任务状态
	PushNum       int     `json:"pushNum"`          // 推送次数
	UpPic         int     `json:"upPic"`            // 1=需要图片
	Score         float64 `json:"score"`
	StartDate     string  `json:"startDateStr"` // 平台返回 "2026-01-12"
	EndDate       string  `json:"endDateStr"`   // 平台返回 "2026-02-10"
	CreatorName   string  `json:"creatorName"`
	RoleName      string  `json:"roleName"`
	TermID        int64   `json:"termId"`
	ScopeType     int     `json:"scopeType"`
	ScopeTypeName string  `json:"scopeTypeName"`
}

// TaskSubmitPayload 是 addCircle 接口的完整请求体（29 字段透传）。
type TaskSubmitPayload struct {
	ID                  *int64  `json:"id"`
	Name                string  `json:"name"`
	HostName            string  `json:"hostName"`
	CircleDate          string  `json:"circleDate"`
	Rank                string  `json:"rank"`
	Level               string  `json:"level"`
	Content             string  `json:"content"`
	PictureList         []int64 `json:"pictureList"`
	CircleTaskID        int64   `json:"circleTaskId"`
	CircleTypeID        int64   `json:"circleTypeId"`
	DimensionID         int64   `json:"dimensionId"`
	Hours               float64 `json:"hours"`
	CircleBeginDate     string  `json:"circleBeginDate"`
	CircleEndDate       string  `json:"circleEndDate"`
	CheckResult         string  `json:"checkResult"`
	PatentType          string  `json:"patentType"`
	PatentNum           string  `json:"patentNum"`
	Address             string  `json:"address"`
	TermName            string  `json:"termName"`
	ActivityName        string  `json:"activityName"`
	SportsName          string  `json:"sportsName"`
	TeamName            string  `json:"teamName"`
	OrgName             string  `json:"orgName"`
	ResultsName         string  `json:"resultsName"`
	ObtainTime          string  `json:"obtainTime"`
	SpecialtyTechnology string  `json:"specialtyTechnology"`
	PlayRole            string  `json:"playRole"`
	LikeSpecialty1      string  `json:"likeSpecialty1"`
	LikeSpecialty2      string  `json:"likeSpecialty2"`
	LikeSpecialty3      string  `json:"likeSpecialty3"`
}

type TaskResult struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}
