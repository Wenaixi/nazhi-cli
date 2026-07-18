package main

import (
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// examCmd 表示 nazhi exam 父命令
var examCmd = &cobra.Command{
	Use:   "exam",
	Short: "成绩管理",
	Long:  `查询学生成绩信息。`,
}

// examQueryCmd 表示 nazhi exam query 命令
//
//	nazhi exam query --token <token> [--base-url <url>] [--timeout <秒>]
var examQueryCmd = &cobra.Command{
	Use:   "query",
	Short: "查询学生成绩",
	Long:  `查询学生成绩信息。`,
	Example: `  nazhi exam query --token eyJhbGciOiJIUzI1NiJ9.xxx
		  nazhi exam query --token eyJhbGciOiJIUzI1NiJ9.xxx --base-url http://139.159.205.146:8280`,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		printVerbose("正在查询成绩...")
		results, err := c.QueryStudentExam(cmd.Context(), token, 0, nil, nil)
		if err != nil {
			printError(fmt.Errorf("查询成绩失败: %w", err))
			return
		}

		printEnvelope(envelope.Success(results))
	},
}

func init() {
	// exam 父命令
	rootCmd.AddCommand(examCmd)

	// exam query
	examCmd.AddCommand(examQueryCmd)
	registerBizFlags(examQueryCmd)
}
