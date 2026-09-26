package main

import (
	"encoding/json"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"github.com/spf13/cobra"
)

// honorCmd 表示 nazhi honor 父命令，下辖 8 个子命令：
//
//	types / list / add / delete / update / levels / type-options / level-options
var honorCmd = &cobra.Command{
	Use:   "honor",
	Short: "荣誉申报管理",
	Long:  `管理综合评价荣誉申报：查看荣誉类型、查看已申报记录、申报荣誉。`,
}

// honorTypesCmd 表示 nazhi honor types 命令
//
//	nazhi honor types --token <token> [--base-url <url>] [--timeout <秒>]
//
// envelope.data 直接透传 SDK GetHonorTypesJSON 的原始 JSON 数组，
// 与平台响应 1:1（dataList 优先 / returnData 兜底）。
var honorTypesCmd = &cobra.Command{
	Use:   "types",
	Short: "获取所有荣誉类型",
	Long:  `获取当前平台可申报的所有荣誉类型列表及级别信息。`,
	Example: `  nazhi honor types --token eyJhbGciOiJIUzI1NiJ9.xxx
  nazhi honor types --token eyJhbGciOiJIUzI1NiJ9.xxx --base-url http://139.159.205.146:8280`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}

		printVerbose("正在获取荣誉类型...")
		raw, err := c.GetHonorTypesJSON(cmd.Context(), token)
		if err != nil {
			printError(fmt.Errorf("获取荣誉类型失败: %w", err))
			return
		}

		printEnvelope(envelope.Success(json.RawMessage(raw)))
	},
}

// honorUpdateKeys 是 honor update payload 顶层 JSON 的允许键，按用户书写形态
// 声明：AddHonorPayload 出站 json 键 + SDK UpdateHonor 消费键 + 编辑记录 id。
// 未知键（如 evaluationAgencyX 拼错）此前静默透传服务端被忽略，
// 对齐 user update 用 unknownUpdatePayloadKeys 拒绝。
var honorUpdateKeys = newPayloadKeySet(
	"id", "name", "typeId", "typeName", "level", "evaluationAgency",
	"getDate", "certImgAttachmentId", "score",
)

// honorAddKeys 是 honor add payload 顶层 JSON 的允许键
// （AddHonorPayload 出站 json 键全集）。honor add 此前按 struct
// 反序列化静默丢弃未知顶层键，拼错键名（如 typeid → 服务端忽略）会
// 204 报成功但零申报——与 update 同款拒绝（400/exit3）。
var honorAddKeys = newPayloadKeySet(
	"name", "typeId", "typeName", "level", "evaluationAgency",
	"getDate", "certImgAttachmentId", "score",
)

// 比较用小写允许集由 keySet 派生，不手工维护第二份。
var (
	honorUpdateAllowedKeys = honorUpdateKeys.allowed()
	honorAddAllowedKeys    = honorAddKeys.allowed()
)

// honorListCmd 表示 nazhi honor list 命令
//
//	nazhi honor list --token <token> [--page <页>] [--page-size <条>] [--base-url <url>] [--timeout <秒>]
//
// envelope.data 直接透传 SDK GetHonorListJSON 的拼装 JSON
// （内部将 records + pageBean 拼为 {records,page}），
// records 与 page 字段值都是平台原始字节。
var honorListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取已申报荣誉记录",
	Long:  `获取当前用户已申报的全部荣誉记录（分页）。支持 --key 关键字筛选。`,
	Example: `  nazhi honor list --token eyJhbGciOiJIUzI1NiJ9.xxx
  nazhi honor list --token eyJhbGciOiJIUzI1NiJ9.xxx --page 1 --page-size 10`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// 先校后建——分页参数校验必须在 buildBizClient 前，
		// 否则缺 token + 坏分页参数时首报「--token 必填」而非分页错误，
		// 脚本拿到错提示自查困难（typical_case.go 同款收敛，对齐 output.go
		// 披露的「先校后建」派）。
		pageNo, _ := cmd.Flags().GetInt("page")
		pageSize, _ := cmd.Flags().GetInt("page-size")
		// 分页参数非负守卫：负值透传会发出 pageNo=-1 等异常请求；
		// 0 同样非法——circle_metadata.go:83-89 同形状参数要求 >0，
		// 对齐为 ≤0 拒绝（400/exit3），与正数契约统一。
		if pageNo <= 0 || pageSize <= 0 {
			printEnvelope(envelope.Error(400, "--page 与 --page-size 必须为正整数"))
			return
		}
		// --page-size 上钳 500（对齐 SDK defaultSubmittedPageSize，
		// 实测服务端 pageSize 上限 500）。超限透传会被服务端静默截断为 500，
		// 分页脚本以错误的 pageSize 计算页数拿到截断数据却不自知——以参数
		// 错误拒绝（400/exit3），与 ≤0 守卫同族。
		if pageSize > maxPageSize {
			printEnvelope(envelope.Error(400, "--page-size 不能超过 500（服务端单页上限）"))
			return
		}

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}

		key, _ := cmd.Flags().GetString("key")

		printVerbose("正在获取荣誉记录...")
		raw, err := c.GetHonorListJSON(cmd.Context(), token, pageNo, pageSize, key)
		if err != nil {
			printError(fmt.Errorf("获取荣誉记录失败: %w", err))
			return
		}

		printEnvelope(envelope.Success(json.RawMessage(raw)))
	},
}

// honorAddCmd 表示 nazhi honor add 命令
//
//	nazhi honor add --token <token> --payload '<json>' [--base-url <url>] [--timeout <秒>]
var honorAddCmd = &cobra.Command{
	Use:   "add",
	Short: "申报荣誉",
	Long: `申报一条荣誉。payload 是 addHonor 请求体 JSON，可用 @file.json 从文件读取，或 - 从 stdin 读取。
getDate 示例用纯日期仅为可读性；前端实际提交 ISO 8601 时间戳（el-date-picker 无 value-format），服务端对纯日期是否接受以平台裁决为准。`,
	Example: `  nazhi honor add --token eyJhbGciOiJIUzI1NiJ9.xxx --payload '{"typeId":1147,"level":5,"evaluationAgency":"示例中学","getDate":"2026-06-30"}'
  nazhi honor add --token eyJhbGciOiJIUzI1NiJ9.xxx --payload @honor.json
  echo '{"typeId":1147,"level":5}' | nazhi honor add --token "xxx" --payload -`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		runWriteOp(cmd, honorAddWriteOp, nil)
	},
}

// honorDeleteCmd 表示 nazhi honor delete 命令
//
//	nazhi honor delete --token <token> --id <id> [--base-url <url>] [--timeout <秒>]
var honorDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "删除一条荣誉记录",
	Long:  `按 ID 删除一条未审核的荣誉记录（已审核记录由服务端拒绝操作）。`,
	Example: `  nazhi honor delete --token eyJhbGciOiJIUzI1NiJ9.xxx --id 123
  nazhi honor delete --token eyJhbGciOiJIUzI1NiJ9.xxx --id 123 --base-url http://139.159.205.146:8280`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		honorID, _ := cmd.Flags().GetInt64("id")
		if honorID <= 0 {
			printEnvelope(envelope.Error(400, "--id 必须为正整数"))
			return
		}

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}

		printVerbose("正在删除荣誉记录...")
		if err := c.DeleteHonor(cmd.Context(), token, honorID); err != nil {
			printError(fmt.Errorf("删除荣誉记录失败: %w", err))
			return
		}

		// DeleteHonor SDK 成功路径返回 nil —— 用 envelope.Empty (HTTP 204) 表达
		// "成功无业务负载"，与 SDK 语义 1:1。
		printEnvelope(envelope.Empty("荣誉记录已删除"))
	},
}

// honorUpdateCmd 表示 nazhi honor update 命令。
var honorUpdateCmd = &cobra.Command{
	Use:     "update",
	Short:   "更新荣誉记录",
	Long:    "更新一条未审核的荣誉记录。payload 必须是 updateHonor 请求体对象，可用 @file.json 或 - 读取。",
	Example: "  nazhi honor update --token eyJhbGciOiJIUzI1NiJ9.xxx --payload @honor-update.json",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		runWriteOp(cmd, honorUpdateWriteOp, nil)
	},
}

// honorLevelsCmd 表示 nazhi honor levels 命令。
//
// 对齐前端 performanceM.vue：用户先选手类型，再按 typeId 联动加载级别。
var honorLevelsCmd = &cobra.Command{
	Use:     "levels",
	Short:   "按荣誉类型查询可用级别",
	Long:    "查询指定荣誉类型的级别下拉。对应 SDK GetHonorLevel，前端 getHonorLevel?honorTypeId=。",
	Example: "  nazhi honor levels --token eyJhbGciOiJIUzI1NiJ9.xxx --type-id 1147",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		typeID, _ := cmd.Flags().GetInt64("type-id")
		if typeID <= 0 {
			printEnvelope(envelope.Error(400, "--type-id 必须为正整数"))
			return
		}

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}
		printVerbose("正在获取荣誉级别...")
		opts, err := c.GetHonorLevel(cmd.Context(), token, typeID)
		if err != nil {
			printError(fmt.Errorf("获取荣誉级别失败: %w", err))
			return
		}
		if opts == nil {
			opts = []types.HonorSelectOption{}
		}
		printEnvelope(envelope.Success(opts))
	},
}

func init() {
	// honor 父命令
	rootCmd.AddCommand(honorCmd)

	// honor types
	honorCmd.AddCommand(honorTypesCmd)
	registerBizFlags(honorTypesCmd)

	// honor list
	honorCmd.AddCommand(honorListCmd)
	honorListCmd.Flags().Int("page", 1, "页码（从 1 开始）")
	honorListCmd.Flags().Int("page-size", 10, "每页条数")
	honorListCmd.Flags().String("key", "", "搜索关键字（可空，对应 getHonorByStudentId 的 key）")
	registerBizFlags(honorListCmd)

	// honor add
	honorCmd.AddCommand(honorAddCmd)
	honorAddCmd.Flags().String("payload", "", "荣誉 JSON（必填，可用 @file.json 从文件读取，或 - 从 stdin 读取）")
	registerBizFlags(honorAddCmd)
	attachAllowedKeysHelp(honorAddCmd, honorAddKeys.display(), true)

	// honor delete
	honorCmd.AddCommand(honorDeleteCmd)
	honorDeleteCmd.Flags().Int64("id", 0, "荣誉记录 ID（必填）")
	registerBizFlags(honorDeleteCmd)

	// honor update
	honorCmd.AddCommand(honorUpdateCmd)
	honorUpdateCmd.Flags().String("payload", "", "荣誉更新 JSON（必填，可用 @file.json 从文件读取，或 - 从 stdin 读取）")
	registerBizFlags(honorUpdateCmd)
	attachAllowedKeysHelp(honorUpdateCmd, honorUpdateKeys.display(), false)

	// honor levels
	honorCmd.AddCommand(honorLevelsCmd)
	honorLevelsCmd.Flags().Int64("type-id", 0, "荣誉类型 ID（必填，对应前端 getHonorLevel?honorTypeId=）")
	registerBizFlags(honorLevelsCmd)
}
