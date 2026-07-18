package main

import (
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"github.com/spf13/cobra"
)

// democraticCmd 表示 nazhi democratic 父命令
var democraticCmd = &cobra.Command{
	Use:   "democratic",
	Short: "民主评价管理",
	Long:  `管理民主评价活动：查看活动、自评、互评、查看结果。`,
}

// democraticListCmd 表示 nazhi democratic list 命令
var democraticListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取民主评价活动列表",
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		pageNo, _ := cmd.Flags().GetInt("page")
		pageSize, _ := cmd.Flags().GetInt("page-size")

		printVerbose("正在获取民主评价活动列表...")
		result, err := c.GetDemocraticActivities(cmd.Context(), token, pageNo, pageSize)
		if err != nil {
			printError(fmt.Errorf("获取活动列表失败: %w", err))
			return
		}
		printEnvelope(envelope.Success(result))
	},
}

// democraticSelfEvalCmd 表示 nazhi democratic self-eval 命令
var democraticSelfEvalCmd = &cobra.Command{
	Use:   "self-eval",
	Short: "查看或提交自评",
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		activityID, _ := cmd.Flags().GetInt64("activity-id")
		if activityID <= 0 {
			printEnvelope(envelope.Error(400, "--activity-id 为必填"))
			return
		}

		subPlanID, _ := cmd.Flags().GetInt64("sub-plan-id")

		content, _ := cmd.Flags().GetString("content")
		if content != "" {
			printVerbose("正在提交自评...")
			// 前端发送的是 JSON 数组，简单场景暂用单条提交
			items := []types.SelfEvaluationInput{
				{ActivityID: activityID},
			}
			if err := c.AddOrUpdateSelfEvaluation(cmd.Context(), token, items); err != nil {
				printError(fmt.Errorf("提交自评失败: %w", err))
				return
			}
			printEnvelope(envelope.Empty("自评提交成功"))
			return
		}

		printVerbose("正在获取自评数据...")
		evaluations, err := c.GetSelfEvaluation(cmd.Context(), token, activityID, subPlanID)
		if err != nil {
			printError(fmt.Errorf("获取自评失败: %w", err))
			return
		}
		printEnvelope(envelope.Success(evaluations))
	},
}

// democraticMutualCmd 表示 nazhi democratic mutual 命令
var democraticMutualCmd = &cobra.Command{
	Use:   "mutual",
	Short: "互评管理",
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		activityID, _ := cmd.Flags().GetInt64("activity-id")
		if activityID <= 0 {
			printEnvelope(envelope.Error(400, "--activity-id 为必填"))
			return
		}

		subPlanID, _ := cmd.Flags().GetInt64("sub-plan-id")
		studentID, _ := cmd.Flags().GetInt64("student-id")
		result, _ := cmd.Flags().GetString("result")

		if studentID > 0 && result != "" {
			printVerbose("正在提交互评...")
			items := []types.MutualEvaluationInput{
				{StudentID: studentID, StudentName: ""},
			}
			if err := c.AddOrUpdateMutualEvaluation(cmd.Context(), token, items); err != nil {
				printError(fmt.Errorf("提交互评失败: %w", err))
				return
			}
			printEnvelope(envelope.Empty("互评提交成功"))
			return
		}

		if studentID > 0 {
			printVerbose("正在获取互评详情...")
			students := []map[string]any{
				{"student_id": studentID},
			}
			details, err := c.GetMutualEvaluationDetail(cmd.Context(), token, activityID, subPlanID, students)
			if err != nil {
				printError(fmt.Errorf("获取互评详情失败: %w", err))
				return
			}
			printEnvelope(envelope.Success(details))
			return
		}

		printVerbose("正在获取互评人员信息...")
		info, err := c.GetMutualPersonInfo(cmd.Context(), token, activityID)
		if err != nil {
			printError(fmt.Errorf("获取互评人员信息失败: %w", err))
			return
		}
		printEnvelope(envelope.Success(info))
	},
}

// democraticResultCmd 表示 nazhi democratic result 命令
var democraticResultCmd = &cobra.Command{
	Use:   "result",
	Short: "查看评价结果",
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		activityID, _ := cmd.Flags().GetInt64("activity-id")
		if activityID <= 0 {
			printEnvelope(envelope.Error(400, "--activity-id 为必填"))
			return
		}

		printVerbose("正在获取评价结果...")
		result, err := c.GetDemocraticResult(cmd.Context(), token, activityID)
		if err != nil {
			printError(fmt.Errorf("获取评价结果失败: %w", err))
			return
		}
		printEnvelope(envelope.Success(result))
	},
}

func init() {
	rootCmd.AddCommand(democraticCmd)
	democraticCmd.AddCommand(democraticListCmd)
	democraticCmd.AddCommand(democraticSelfEvalCmd)
	democraticCmd.AddCommand(democraticMutualCmd)
	democraticCmd.AddCommand(democraticResultCmd)

	democraticListCmd.Flags().Int("page", 1, "页码")
	democraticListCmd.Flags().Int("page-size", 10, "每页条数")
	registerBizFlags(democraticListCmd)

	democraticSelfEvalCmd.Flags().Int64("activity-id", 0, "活动 ID（必填）")
	democraticSelfEvalCmd.Flags().Int64("sub-plan-id", 0, "子计划 ID")
	democraticSelfEvalCmd.Flags().String("content", "", "自评内容（不填则查询）")
	registerBizFlags(democraticSelfEvalCmd)

	democraticMutualCmd.Flags().Int64("activity-id", 0, "活动 ID（必填）")
	democraticMutualCmd.Flags().Int64("sub-plan-id", 0, "子计划 ID")
	democraticMutualCmd.Flags().Int64("student-id", 0, "学生 ID（不填查人员列表，填+result 则提交）")
	democraticMutualCmd.Flags().String("result", "", "互评结果（与 --student-id 一起使用提交）")
	registerBizFlags(democraticMutualCmd)

	democraticResultCmd.Flags().Int64("activity-id", 0, "活动 ID（必填）")
	registerBizFlags(democraticResultCmd)
}
