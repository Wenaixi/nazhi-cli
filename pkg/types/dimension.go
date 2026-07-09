package types

// Dimension 是维度信息。
//
// 用途：GetDimensions() 返回所有可用维度（思想品德/学业水平/身心健康/艺术素养/社会实践/劳动素养）。
// FetchTasks 内部按维度并发拉取任务，最终注入 DimensionName 到 Task。
type Dimension struct {
	ID   int64  `json:"id"   example:"9"     description:"维度 ID（0=全部汇总，FetchTasks 会跳过）"`
	Name string `json:"name" example:"思想品德" description:"维度名"`
}
