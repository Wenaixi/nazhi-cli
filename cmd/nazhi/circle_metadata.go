package main

import (
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// circleTypesCmd 获取指定维度下的写实类别。
var circleTypesCmd = &cobra.Command{
	Use:     "types",
	Short:   "获取写实类别",
	Long:    "按维度获取写实类别。pid 可选，用于透传平台类别树的父节点。",
	Example: "  nazhi circle types --token eyJhbGciOiJIUzI1NiJ9.xxx --dimension-id 14 --pid 0",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		dimensionID, _ := cmd.Flags().GetInt64("dimension-id")
		if dimensionID <= 0 {
			printParamError(fmt.Errorf("--dimension-id 必须为正整数"))
			return
		}
		pid, _ := cmd.Flags().GetString("pid")
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}
		printVerbose("正在获取写实类别 dimensionId=%d...", dimensionID)
		items, err := c.GetCircleTypes(cmd.Context(), token, dimensionID, pid)
		if err != nil {
			printError(fmt.Errorf("获取写实类别失败: %w", err))
			return
		}
		if items == nil {
			items = []map[string]any{}
		}
		printEnvelope(envelope.Success(items))
	},
}

// circleTasksCmd 获取指定类别下的写实任务。
var circleTasksCmd = &cobra.Command{
	Use:     "tasks",
	Short:   "获取类别下的写实任务",
	Long:    "按写实类别 ID 获取可用任务及其平台字段。",
	Example: "  nazhi circle tasks --token eyJhbGciOiJIUzI1NiJ9.xxx --type-id 9274",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		typeID, _ := cmd.Flags().GetInt64("type-id")
		if typeID <= 0 {
			printParamError(fmt.Errorf("--type-id 必须为正整数"))
			return
		}
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}
		printVerbose("正在获取类别下写实任务 typeId=%d...", typeID)
		items, err := c.GetCircleTasks(cmd.Context(), token, typeID)
		if err != nil {
			printError(fmt.Errorf("获取类别下写实任务失败: %w", err))
			return
		}
		if items == nil {
			items = []map[string]any{}
		}
		printEnvelope(envelope.Success(items))
	},
}

// circleImagesCmd 获取当前用户上传的写实图片。
var circleImagesCmd = &cobra.Command{
	Use:     "images",
	Short:   "获取写实图片列表",
	Long:    "分页获取当前用户上传的写实图片及附件字段。",
	Example: "  nazhi circle images --token eyJhbGciOiJIUzI1NiJ9.xxx --page 1 --page-size 20",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		page, _ := cmd.Flags().GetInt("page")
		pageSize, _ := cmd.Flags().GetInt("page-size")
		if page <= 0 {
			printParamError(fmt.Errorf("--page 必须为正整数"))
			return
		}
		if pageSize <= 0 {
			printParamError(fmt.Errorf("--page-size 必须为正整数"))
			return
		}
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}
		printVerbose("正在获取写实图片 page=%d pageSize=%d...", page, pageSize)
		items, err := c.GetCircleImages(cmd.Context(), token, page, pageSize)
		if err != nil {
			printError(fmt.Errorf("获取写实图片失败: %w", err))
			return
		}
		if items == nil {
			items = []map[string]any{}
		}
		printEnvelope(envelope.Success(items))
	},
}

// circleDictCmd 获取系统字典列表。
var circleDictCmd = &cobra.Command{
	Use:     "dict",
	Short:   "获取系统字典",
	Long:    "按字典分类编码获取平台字典项，例如写实等级常用 cateCode=23。",
	Example: "  nazhi circle dict --token eyJhbGciOiJIUzI1NiJ9.xxx --cate-code 23",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cateCode, _ := cmd.Flags().GetInt("cate-code")
		if cateCode <= 0 {
			printParamError(fmt.Errorf("--cate-code 必须为正整数"))
			return
		}
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}
		printVerbose("正在获取系统字典 cateCode=%d...", cateCode)
		items, err := c.GetDictList(cmd.Context(), token, cateCode)
		if err != nil {
			printError(fmt.Errorf("获取系统字典失败: %w", err))
			return
		}
		if items == nil {
			items = []map[string]any{}
		}
		printEnvelope(envelope.Success(items))
	},
}

func init() {
	circleCmd.AddCommand(circleTypesCmd)
	circleTypesCmd.Flags().Int64("dimension-id", 0, "写实维度 ID（必填）")
	circleTypesCmd.Flags().String("pid", "", "类别树父节点 ID（可选）")
	registerBizFlags(circleTypesCmd)

	circleCmd.AddCommand(circleTasksCmd)
	circleTasksCmd.Flags().Int64("type-id", 0, "写实类别 ID（必填）")
	registerBizFlags(circleTasksCmd)

	circleCmd.AddCommand(circleImagesCmd)
	circleImagesCmd.Flags().Int("page", 1, "页码（从 1 开始）")
	circleImagesCmd.Flags().Int("page-size", 10, "每页条数")
	registerBizFlags(circleImagesCmd)

	circleCmd.AddCommand(circleDictCmd)
	circleDictCmd.Flags().Int("cate-code", 0, "字典分类编码（必填）")
	registerBizFlags(circleDictCmd)
}
