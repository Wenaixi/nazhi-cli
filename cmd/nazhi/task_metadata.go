package main

import (
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"github.com/spf13/cobra"
)

// taskDimensionsCmd 获取平台写实维度列表。
var taskDimensionsCmd = &cobra.Command{
	Use:     "dimensions",
	Short:   "获取写实维度列表",
	Long:    "获取目标平台的写实维度列表，供后续类别和任务查询使用。",
	Example: "  nazhi task dimensions --token eyJhbGciOiJIUzI1NiJ9.xxx",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}
		printVerbose("正在获取写实维度...")
		dimensions, err := c.GetDimensions(cmd.Context(), token)
		if err != nil {
			printError(fmt.Errorf("获取写实维度失败: %w", err))
			return
		}
		if dimensions == nil {
			dimensions = []types.Dimension{}
		}
		printEnvelope(envelope.Success(dimensions))
	},
}

// taskCircleTypeCmd 获取任务提交所需的 circleTypeId、dimensionId、hours 等元数据。
var taskCircleTypeCmd = &cobra.Command{
	Use:     "circle-type",
	Short:   "获取任务写实元数据",
	Long:    "按任务 ID 获取提交写实记录所需的类别、维度、学时等平台元数据。",
	Example: "  nazhi task circle-type --token eyJhbGciOiJIUzI1NiJ9.xxx --task-id 18154",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		taskID, _ := cmd.Flags().GetInt64("task-id")
		if taskID <= 0 {
			printParamError(fmt.Errorf("--task-id 必须为正整数"))
			return
		}
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}
		printVerbose("正在获取任务写实元数据 taskId=%d...", taskID)
		info, err := c.GetCircleTypeByTaskID(cmd.Context(), token, taskID)
		if err != nil {
			printError(fmt.Errorf("获取任务写实元数据失败: %w", err))
			return
		}
		if info == nil {
			printEnvelope(envelope.Empty("未找到任务写实元数据"))
			return
		}
		printEnvelope(envelope.Success(info))
	},
}

func init() {
	taskCmd.AddCommand(taskDimensionsCmd)
	registerBizFlags(taskDimensionsCmd)

	taskCmd.AddCommand(taskCircleTypeCmd)
	taskCircleTypeCmd.Flags().Int64("task-id", 0, "平台任务 ID（必填）")
	registerBizFlags(taskCircleTypeCmd)
}
