package client

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// CircleListType 是写实列表类型：查询写实列表时的入口分类。
//
// 命名对应 CONTEXT.md「状态与列表」中的「写实列表类型」——它描述「从哪个
// 列表看」，与单条写实自身的状态（CircleRecord.Status：0 已发布 / 1 已锁定 /
// 2 被撤回）无关，也与记录自身的 type 字段（判是否学生本人发布）无关。
// 这三者极易混读，改动前请先读 CONTEXT.md「一处已订正的冲突」。
//
// 四个常量的整数值是 getStudentCircle 查询参数 type 的线协议约定，
// 改动会破坏与平台的兼容，勿动。
type CircleListType int

const (
	// CircleListPublic 公示：教师代写的全部写实记录（1 公示 / 2 教师写实
	// / 3 我发布的 / 4 被撤回的 中的第 1 类）。
	CircleListPublic CircleListType = 1
	// CircleListTeacher 教师写实：教师代写的全部写实记录。
	CircleListTeacher CircleListType = 2
	// CircleListSubmitted 我发布的：写实状态为零且由当前学生发布的记录。
	// 与「已提交（任务维度）」不是同一概念——后者说的是「这个任务交没交」。
	CircleListSubmitted CircleListType = 3
	// CircleListWithdrawn 被撤回的：写实状态为二并附撤回原因的记录。
	CircleListWithdrawn CircleListType = 4
)

// Label 返回写实列表类型的中文领域名，供 CLI 文案与错误信息使用。
// 未知类型返回空串，调用方应先经 Valid 判定。
func (t CircleListType) Label() string {
	switch t {
	case CircleListPublic:
		return "公示"
	case CircleListTeacher:
		return "教师写实"
	case CircleListSubmitted:
		return "我发布的"
	case CircleListWithdrawn:
		return "被撤回"
	default:
		return ""
	}
}

// Valid 报告该类型是否为平台承认的四种之一。
//
// 判定在发请求之前：查询参数 type 直接驱动服务端的列表过滤，传错值不会
// 报错，只会静默返回另一个列表的数据——这正是「写实列表类型」被反复
// 误读的根源，具名类型加前置校验是让它不再依赖注释提醒的根本手段。
func (t CircleListType) Valid() bool {
	switch t {
	case CircleListPublic, CircleListTeacher, CircleListSubmitted, CircleListWithdrawn:
		return true
	default:
		return false
	}
}

// CircleListTypeFromValue 从平台的 type 参数值反查写实列表类型。
// 用于把 CLI 的 --type 输入与 SDK 共享同一份映射，避免两侧各写一张表。
func CircleListTypeFromValue(v int) (CircleListType, bool) {
	t := CircleListType(v)
	if !t.Valid() {
		return 0, false
	}
	return t, true
}

// errInvalidCircleListType 构造非法写实列表类型的错误。
//
// 与其它参数错误一样归 ErrInvalidPayload：调用方可控的输入问题，
// 映射为 400 / 退出码 3，且不发出任何业务请求。
func errInvalidCircleListType(t CircleListType) error {
	return fmt.Errorf("%w: 非法写实列表类型 %d（合法值 1=公示 2=教师写实 3=我发布的 4=被撤回）", ErrInvalidPayload, int(t))
}

// ListCirclesJSON 按写实列表类型获取写实记录，返回平台原始 JSON 数组。
//
// 这是 GetPublicCirclesJSON / GetTeacherCirclesJSON / GetSubmittedCirclesJSON /
// GetWithdrawnCirclesJSON 四个入口的统一形式——它们除方法名与那个数字外
// 逐字相同，四个入口曾各自占一行转发（raw_json.go），调用方要学四个名字却
// 只用上一份知识。旧入口保留为薄壳，转发到本方法，行为逐字不变
// （由 TestListCirclesJSON_EquivOldEntrypoints 锁定）。
//
// key 为搜索关键字（可空，对应 getStudentCircle 的 key 查询参数）。
// 取消语义：ctx 取消时返回（已有合并数据, ctx.Err()），调用方按 partial envelope 处理。
func (c *Client) ListCirclesJSON(ctx context.Context, token string, listType CircleListType, key string) (json.RawMessage, error) {
	if !listType.Valid() {
		return nil, errInvalidCircleListType(listType)
	}
	raw, _, err := c.getCirclesJSON(ctx, token, int(listType), key, "ListCirclesJSON")
	return raw, err
}

// ListCirclesLimitJSON 按写实列表类型分页获取写实记录，返回平台原始 JSON 数组与分页元数据。
//
// offset=0, limit=0 时为全量（等于 ListCirclesJSON），此时仍保留 PageBean——
// 丢弃分页元数据是已修复过的回归，不得回退。
// offset/limit 超出实际数据量时返回空数组，不报错。
func (c *Client) ListCirclesLimitJSON(ctx context.Context, token string, listType CircleListType, offset, limit int, key string) (json.RawMessage, *types.PageBean, error) {
	if !listType.Valid() {
		return nil, nil, errInvalidCircleListType(listType)
	}
	return c.getCirclesLimitJSON(ctx, token, offset, limit, int(listType), key, "ListCirclesLimitJSON")
}

// PeekCircleTotal 按写实列表类型取总条数，不做全量取数。
//
// 这是 PeekPublicTotal / PeekTeacherTotal / PeekSubmittedTotal /
// PeekWithdrawnTotal 四个入口的统一形式，理由同 ListCirclesJSON。
// CLI 的 --count 分支用它输出记录总数。
func (c *Client) PeekCircleTotal(ctx context.Context, token string, listType CircleListType, key string) (int, error) {
	if !listType.Valid() {
		return 0, errInvalidCircleListType(listType)
	}
	return c.peekCircleTotal(ctx, token, int(listType), key, "PeekCircleTotal")
}
