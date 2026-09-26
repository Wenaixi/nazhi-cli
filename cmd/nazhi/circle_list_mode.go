package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"github.com/spf13/cobra"
)

// circleListMode 描述一个写实列表命令的领域差异。
//
// 背景（2026-09-26 架构核实）：public / teacher / submitted / withdrawn
// 四个命令的 Run 回调各约 76 行，其中约 56 行完全字面重复——
// flag 读取、rejectLoneOffset、count/limit/全量三分支、partial envelope、
// total 取值、空数组归一全部各写一遍，仅领域词与方法名不同。
//
// 提取后「列表模式」只有一处实现：改 count 冲突规则、partial 判定或
// envelope 形状时只改这里，四个命令不再需要手工同步。
//
// 输出契约由 circle_list_mode_test.go 锁定（三种模式 × 四个命令）：
//   - --count：envelope.Success({"total": N})
//   - --limit/--offset：envelope.Success({"records": [...], "total": N})
//   - 全量：envelope.Success(裸记录数组)
type circleListMode struct {
	// label 是领域名词，用于 verbose 与错误文案（如「公示」「被撤回」）
	label string
	// countErrorLabel 用于 --count 分支的错误文案；为空时复用 label
	countErrorLabel string

	// listType 是写实列表类型（公示 / 教师写实 / 我发布的 / 被撤回的）。
	//
	// 此前本结构持三个函数指针（peekTotal / listLimit / listAll），每个 mode
	// 各配一份、每个都是一行转发——四个 mode 共 12 个转发闭包，占 52 行。
	// SDK 侧改为按写实列表类型的统一入口后，命令只需声明「我是哪个列表」，
	// 三条路径由下面三个方法按 listType 分发。
	listType client.CircleListType
}

// peekTotal 取该列表的总条数。
func (m circleListMode) peekTotal(ctx context.Context, c *client.Client, token, key string) (int, error) {
	return c.PeekCircleTotal(ctx, token, m.listType, key)
}

// listLimit 分页取该列表的记录。
func (m circleListMode) listLimit(ctx context.Context, c *client.Client, token, key string, offset, limit int) (json.RawMessage, *types.PageBean, error) {
	return c.ListCirclesLimitJSON(ctx, token, m.listType, offset, limit, key)
}

// listAll 取该列表的全部记录（自动翻页合并）。
func (m circleListMode) listAll(ctx context.Context, c *client.Client, token, key string) (json.RawMessage, error) {
	return c.ListCirclesJSON(ctx, token, m.listType, key)
}

// run 执行写实列表命令的完整控制流。
//
// 失败契约：
//   - 已取到数据但后续出错 → envelope.Partial(207, ...)，不丢弃已取数据
//   - 无数据且出错 → printError（按哨兵映射 HTTP 码与退出码）
func (m circleListMode) run(cmd *cobra.Command) {
	c, token, err := buildBizClient(cmd)
	if err != nil {
		printParamError(err)
		return
	}

	onlyCount, _ := cmd.Flags().GetBool("count")
	offset, _ := cmd.Flags().GetInt("offset")
	limit, _ := cmd.Flags().GetInt("limit")
	key, _ := cmd.Flags().GetString("key")

	// rejectLoneOffset 必须先于 onlyCount——否则 --count --offset 5
	// 会绕过 offset/limit 校验静默返回 total。
	if rejectLoneOffset(cmd) {
		return
	}

	ctx := cmd.Context()

	if onlyCount {
		printVerbose("正在获取%s写实记录总数...", m.label)
		total, err := m.peekTotal(ctx, c, token, key)
		if err != nil {
			printError(fmt.Errorf("获取%s写实记录总数失败: %w", m.countLabel(), err))
			return
		}
		printEnvelope(envelope.Success(map[string]int{"total": total}))
		return
	}

	if offset > 0 || limit > 0 {
		printVerbose("正在获取%s写实记录（limit=%d, offset=%d）...", m.label, limit, offset)
		raw, pb, err := m.listLimit(ctx, c, token, key, offset, limit)
		if err != nil {
			if len(raw) > 0 {
				printEnvelope(envelope.Partial(207, m.listFailMsg(err),
					map[string]any{"records": json.RawMessage(raw), "total": totalOf(pb)}))
				return
			}
			printError(fmt.Errorf("%s", m.listFailMsg(err)))
			return
		}
		printEnvelope(envelope.Success(map[string]any{
			"records": json.RawMessage(raw),
			"total":   totalOf(pb),
		}))
		return
	}

	printVerbose("正在获取%s写实记录...", m.label)
	raw, err := m.listAll(ctx, c, token, key)
	if err != nil {
		if len(raw) > 0 {
			printEnvelope(envelope.Partial(207, m.listFailMsg(err), json.RawMessage(raw)))
			return
		}
		printError(fmt.Errorf("%s", m.listFailMsg(err)))
		return
	}
	if len(raw) == 0 {
		printEnvelope(envelope.Success(json.RawMessage("[]")))
		return
	}
	printEnvelope(envelope.Success(json.RawMessage(raw)))
}

// countLabel 返回 --count 分支使用的文案标签。
//
// submitted 命令的 count 错误文案历史上是「获取记录总数失败」（无「写实」二字），
// 与其余三个命令的「获取X写实记录总数失败」不同。此处保留该差异，
// 避免重构悄悄改变用户可见文案。
func (m circleListMode) countLabel() string {
	if m.countErrorLabel != "" {
		return m.countErrorLabel
	}
	return m.label + "写实记录"
}

// listFailMsg 统一列表错误文案：保持与重构前逐字一致，
// 避免破坏依赖字符串匹配的脚本。
func (m circleListMode) listFailMsg(err error) string {
	return fmt.Sprintf("获取%s写实记录失败: %s", m.label, err.Error())
}

// totalOf 安全读取分页总数：PageBean 可能为 nil。
func totalOf(pb *types.PageBean) int {
	if pb == nil {
		return 0
	}
	return pb.TotalNum
}

// ─── 四个命令的模式配置 ───
//
// 四个 mode 的全部差异就是「我是哪个写实列表类型」与两处文案。SDK 侧提供
// 按类型分发的统一入口后，这里从 52 行转发闭包塌缩为 4 行声明——新增一个
// 列表类型只需在此加一行，不必再写三份转发。

var publicCircleListMode = circleListMode{
	label:    "公示",
	listType: client.CircleListPublic,
}

var teacherCircleListMode = circleListMode{
	label:    "教师",
	listType: client.CircleListTeacher,
}

var submittedCircleListMode = circleListMode{
	label:           "我发布的",
	countErrorLabel: "记录",
	listType:        client.CircleListSubmitted,
}

var withdrawnCircleListMode = circleListMode{
	label:    "被撤回",
	listType: client.CircleListWithdrawn,
}
