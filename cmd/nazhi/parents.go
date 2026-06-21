package main

import "github.com/spf13/cobra"

// 父命令：task
var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "任务管理",
	Long:  `管理综合评价任务：查看任务列表、提交任务。`,
}

// 父命令：self-eval
var selfEvalCmd = &cobra.Command{
	Use:   "self-eval",
	Short: "自我评价管理",
	Long:  `管理自我评价：提交评价、查询状态。`,
}

// 父命令：file
var fileCmd = &cobra.Command{
	Use:   "file",
	Short: "文件上传",
	Long:  `上传图片文件到文件服务器。`,
}
