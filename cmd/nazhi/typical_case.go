package main

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"github.com/spf13/cobra"
)

// typicalCaseCmd 表示 nazhi typical-case 父命令
//
//	nazhi typical-case submit --token <token> --payload '<json>'
//	nazhi typical-case list --token <token> [--page <页>] [--page-size <条>]
var typicalCaseCmd = &cobra.Command{
	Use:   "typical-case",
	Short: "典型案例管理",
	Long:  `管理综合评价典型案例：提交典型案例、查看已提交记录。`,
}

// typicalCaseSubmitCmd 表示 nazhi typical-case submit 命令
//
//	nazhi typical-case submit --token <token> --payload '<json>'
//	成功时 envelope.Empty("典型案例提交成功")，与 AddHonor 写操作模式一致
var typicalCaseSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "提交典型案例",
	Long: `提交一条典型案例。payload 是 addTypicalCase 请求体 JSON，
可用 @file.json 从文件读取，或 - 从 stdin 读取。`,
	Example: `  nazhi typical-case submit --token eyJhbGciOiJIUzI1NiJ9.xxx --payload '{"title":"...","type":"1","teacherName":"王隆滨","partnerName":"合作者","remark":"任务描述","content":"..."}'
  nazhi typical-case submit --token eyJhbGciOiJIUzI1NiJ9.xxx --payload @case.json
  echo '{"title":"..."}' | nazhi typical-case submit --token "xxx" --payload -`,
	Args: cobra.NoArgs, // 输入全走 flag，位置参数无语义；与 delete-batch 及全仓多数派对齐
	Run: func(cmd *cobra.Command, args []string) {
		payloadRaw, _ := cmd.Flags().GetString("payload")
		if payloadRaw == "" {
			printEnvelope(envelope.Error(400, "--payload 为必填"))
			return
		}

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}

		payloadBytes, err := parseJSONObjectPayload(payloadRaw)
		if err != nil {
			printParamError(fmt.Errorf("读取 payload 失败: %w", err))
			return
		}

		var payload types.AddTypicalCasePayload
		if err := json.Unmarshal(payloadBytes, &payload); err != nil {
			printParamError(fmt.Errorf("解析 payload JSON 失败: %w", err))
			return
		}

		printVerbose("正在提交典型案例...")
		if err := c.AddTypicalCase(cmd.Context(), token, payload); err != nil {
			printError(fmt.Errorf("提交典型案例失败: %w", err))
			return
		}

		printEnvelope(envelope.Empty("典型案例提交成功"))
	},
}

// typicalCaseListCmd 表示 nazhi typical-case list 命令
//
//	nazhi typical-case list --token <token> [--page <页>] [--page-size <条>] [--status <状态>]
//	envelope.data 透传 SDK GetTypicalCaseListJSON 的原始 JSON（含 records + page）
//	status：0 未审 / 1 通过 / 2 驳回 / 3 全部（默认 3，与前端一致）
var typicalCaseListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取典型案例列表",
	Long: `获取当前用户的典型案例记录（分页）。
--status 筛选审核状态：0 未审核 / 1 通过 / 2 驳回 / 3 全部（默认）。`,
	Example: `  nazhi typical-case list --token eyJhbGciOiJIUzI1NiJ9.xxx
  nazhi typical-case list --token eyJhbGciOiJIUzI1NiJ9.xxx --status 1
  nazhi typical-case list --token eyJhbGciOiJIUzI1NiJ9.xxx --page 1 --page-size 10`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}

		pageNo, _ := cmd.Flags().GetInt("page")
		pageSize, _ := cmd.Flags().GetInt("page-size")
		status, _ := cmd.Flags().GetInt("status")
		// 分页参数非负守卫：与 honor list / circle_metadata.go:83-89 纪律对齐。
		// status 合法值为 0/1/2/3（0 未审核 / 1 通过 / 2 驳回 / 3 全部·默认，前端 classiccanter.vue el-option 相同）。
		// status=-1 虽非法，但为避免破坏现有用户脚本（可能用 -1 表达「全部」），
		// 此处仅拒绝 pageNo/pageSize 负值；status 校验留待服务端。
		if pageNo < 0 || pageSize < 0 {
			printEnvelope(envelope.Error(400, "--page 与 --page-size 必须为非负整数"))
			return
		}

		printVerbose("正在获取典型案例列表...")
		raw, err := c.GetTypicalCaseListJSON(cmd.Context(), token, pageNo, pageSize, status)
		if err != nil {
			printError(fmt.Errorf("获取典型案例列表失败: %w", err))
			return
		}

		printEnvelope(envelope.Success(json.RawMessage(raw)))
	},
}

func init() {
	// typical-case submit
	typicalCaseCmd.AddCommand(typicalCaseSubmitCmd)
	typicalCaseSubmitCmd.Flags().String("payload", "", "典型案例 JSON（必填，可用 @file.json 从文件读取，或 - 从 stdin 读取）")
	registerBizFlags(typicalCaseSubmitCmd)

	// typical-case list
	typicalCaseCmd.AddCommand(typicalCaseListCmd)
	typicalCaseListCmd.Flags().Int("page", 1, "页码（从 1 开始）")
	typicalCaseListCmd.Flags().Int("page-size", 10, "每页条数")
	typicalCaseListCmd.Flags().Int("status", 3, "审核状态：0 未审 / 1 通过 / 2 驳回 / 3 全部（默认）")
	registerBizFlags(typicalCaseListCmd)

	// typical-case update
	typicalCaseCmd.AddCommand(typicalCaseUpdateCmd)
	typicalCaseUpdateCmd.Flags().String("payload", "", "典型案例 JSON（必填，可用 @file.json）")
	registerBizFlags(typicalCaseUpdateCmd)

	// typical-case delete
	typicalCaseCmd.AddCommand(typicalCaseDeleteCmd)
	typicalCaseDeleteCmd.Flags().String("id", "", "案例 ID（必填）")
	registerBizFlags(typicalCaseDeleteCmd)

	// typical-case delete-batch
	typicalCaseCmd.AddCommand(typicalCaseDeleteBatchCmd)
	typicalCaseDeleteBatchCmd.Flags().String("payload", "", "典型案例 ID 数组（必填，可用 @file.json 从文件读取，或 - 从 stdin 读取）")
	registerBizFlags(typicalCaseDeleteBatchCmd)
}

// typicalCaseDeleteBatchCmd 表示 nazhi typical-case delete-batch 命令。
var typicalCaseDeleteBatchCmd = &cobra.Command{
	Use:     "delete-batch",
	Short:   "批量删除典型案例",
	Long:    "批量删除典型案例。payload 必须是正整数 ID 数组，可用 @file.json 或 - 读取。",
	Example: "  nazhi typical-case delete-batch --token eyJhbGciOiJIUzI1NiJ9.xxx --payload '[1,2,3]'",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		payloadRaw, _ := cmd.Flags().GetString("payload")
		if payloadRaw == "" {
			printEnvelope(envelope.Error(400, "--payload 为必填"))
			return
		}
		ids, err := parseTypicalCaseBatchIDs(payloadRaw)
		if err != nil {
			printParamError(fmt.Errorf("读取 payload 失败: %w", err))
			return
		}

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}
		printVerbose("正在批量删除典型案例...")
		if err := c.DeleteBatchTypicalCase(cmd.Context(), token, ids); err != nil {
			printError(fmt.Errorf("批量删除典型案例失败: %w", err))
			return
		}
		printEnvelope(envelope.Empty("典型案例已批量删除"))
	},
}

// parseTypicalCaseBatchIDs 读取并校验批量删除的纯 ID 数组。
func parseTypicalCaseBatchIDs(raw string) ([]int64, error) {
	payload, err := parsePayloadFromArg(raw)
	if err != nil {
		return nil, err
	}
	var ids []int64
	if err := json.Unmarshal(payload, &ids); err != nil {
		return nil, fmt.Errorf("顶层 JSON 必须是正整数数组: %w", err)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("批量 ID 数组不能为空")
	}
	for _, id := range ids {
		if id <= 0 {
			return nil, fmt.Errorf("批量 ID 必须为正整数，实际 %d", id)
		}
	}
	return ids, nil
}

// typicalCaseUpdateCmd 表示 nazhi typical-case update 命令。
var typicalCaseUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "更新典型案例",
	Long:  `按 ID 更新典型案例内容。payload 为 updateTypicalCase 请求体对象，必填 id 字段。`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		payloadRaw, _ := cmd.Flags().GetString("payload")
		if payloadRaw == "" {
			printEnvelope(envelope.Error(400, "--payload 为必填"))
			return
		}
		payloadBytes, err := parseJSONObjectPayload(payloadRaw)
		if err != nil {
			printParamError(fmt.Errorf("读取 payload 失败: %w", err))
			return
		}
		var payload map[string]any
		if err := json.Unmarshal(payloadBytes, &payload); err != nil {
			printParamError(fmt.Errorf("解析 payload JSON 失败: %w", err))
			return
		}
		// 前端编辑提交必然注入记录 id（classiccanter.vue:327），此处对齐契约：
		// 缺 id 或非正数时拒绝，不发业务请求（与同文件 delete/delete-batch 口径一致）。
		if !PayloadPositiveIDValid(payload) {
			printEnvelope(envelope.Error(400, "payload 必须包含正数 id 字段"))
			return
		}

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}

		printVerbose("正在更新典型案例...")
		if err := c.UpdateTypicalCase(cmd.Context(), token, payload); err != nil {
			printError(fmt.Errorf("更新典型案例失败: %w", err))
			return
		}
		printEnvelope(envelope.Empty("典型案例更新成功"))
	},
}

// typicalCaseDeleteCmd 表示 nazhi typical-case delete 命令
var typicalCaseDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "删除典型案例",
	Long:  `按 ID 删除一条典型案例记录。--id 必填且必须为正整数。`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		idStr, _ := cmd.Flags().GetString("id")
		if idStr == "" {
			printEnvelope(envelope.Error(400, "--id 为必填"))
			return
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			printEnvelope(envelope.Error(400, "--id 必须为正整数"))
			return
		}

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}

		printVerbose("正在删除典型案例...")
		if err := c.DeleteTypicalCase(cmd.Context(), token, id); err != nil {
			printError(fmt.Errorf("删除典型案例失败: %w", err))
			return
		}
		printEnvelope(envelope.Empty("典型案例已删除"))
	},
}
