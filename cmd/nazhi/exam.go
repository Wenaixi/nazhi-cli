package main

import (
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"github.com/spf13/cobra"
)

// examCmd 表示 nazhi exam 父命令
var examCmd = &cobra.Command{
	Use:   "exam",
	Short: "成绩管理",
	Long:  `查询学生成绩信息。`,
}

// examInitCmd 表示 nazhi exam init 命令
var examInitCmd = &cobra.Command{
	Use:     "init",
	Short:   "获取成绩初始化数据",
	Long:    `获取学期、考试类型、课程列表等初始化数据。`,
	Example: "  nazhi exam init --term-id 18 --token xxx",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		termID, _ := cmd.Flags().GetInt64("term-id")
		if termID <= 0 {
			printEnvelope(envelope.Error(400, "--term-id 为必填且必须 > 0"))
			return
		}

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		printVerbose("正在获取成绩初始化数据...")
		info, err := c.GetExamInitInfo(cmd.Context(), token, termID)
		if err != nil {
			printError(fmt.Errorf("获取成绩初始化数据失败: %w", err))
			return
		}
		printEnvelope(envelope.Success(info))
	},
}

// examQueryCmd 表示 nazhi exam query 命令
var examQueryCmd = &cobra.Command{
	Use:     "query",
	Short:   "查询学生成绩",
	Long:    `查询学生成绩。需先通过 exam init 获取学期/考试/课程参数。`,
	Example: "  nazhi exam query --term-id 18 --exam-id 1 --course-id 1 --token xxx",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		termID, _ := cmd.Flags().GetInt64("term-id")
		if termID <= 0 {
			printEnvelope(envelope.Error(400, "--term-id 为必填且必须 > 0"))
			return
		}

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		// 获取完整参数列表
		printVerbose("正在获取学期/考试/课程数据...")
		initInfo, err := c.GetExamInitInfo(cmd.Context(), token, termID)
		if err != nil {
			printError(fmt.Errorf("获取初始化数据失败: %w", err))
			return
		}

		examID, _ := cmd.Flags().GetInt64("exam-id")
		courseID, _ := cmd.Flags().GetInt64("course-id")

		examList := initInfo.ExamList
		if examID > 0 {
			examList = []types.ExamType{{ID: examID, Name: ""}}
		}
		courseList := initInfo.CourseList
		if courseID > 0 {
			courseList = []types.Course{{CourseID: courseID, CourseName: ""}}
		}

		params := types.QueryExamParams{
			TermID:     termID,
			ExamList:   examList,
			CourseList: courseList,
		}

		printVerbose("正在查询成绩...")
		results, err := c.QueryStudentExam(cmd.Context(), token, params)
		if err != nil {
			printError(fmt.Errorf("查询成绩失败: %w", err))
			return
		}

		printEnvelope(envelope.Success(results))
	},
}

func init() {
	rootCmd.AddCommand(examCmd)
	examCmd.AddCommand(examInitCmd)
	examCmd.AddCommand(examQueryCmd)

	examInitCmd.Flags().Int64("term-id", 0, "学期 ID（必填）")
	registerBizFlags(examInitCmd)

	examQueryCmd.Flags().Int64("term-id", 0, "学期 ID（必填）")
	examQueryCmd.Flags().Int64("exam-id", 0, "考试 ID（可选）")
	examQueryCmd.Flags().Int64("course-id", 0, "课程 ID（可选）")
	registerBizFlags(examQueryCmd)
}
