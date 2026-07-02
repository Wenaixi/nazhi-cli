package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"github.com/spf13/cobra"
)

// honorCmd 表示 nazhi honor 父命令
//
//	nazhi honor types [--base-url <url>] [--timeout <秒>]
//	nazhi honor list [--page <页>] [--page-size <条>] [--base-url <url>] [--timeout <秒>]
//	nazhi honor add --payload '<json>' [--base-url <url>] [--timeout <秒>]
var honorCmd = &cobra.Command{
	Use:   "honor",
	Short: "荣誉申报管理",
	Long:  `管理综合评价荣誉申报：查看荣誉类型、查看已申报记录、申报荣誉。`,
}

// honorTypesCmd 表示 nazhi honor types 命令
//
//	nazhi honor types --token <token> [--base-url <url>] [--timeout <秒>]
var honorTypesCmd = &cobra.Command{
	Use:   "types",
	Short: "获取所有荣誉类型",
	Long:  `获取当前平台可申报的所有荣誉类型列表及级别信息。`,
	Example: `  nazhi honor types --token eyJhbGciOiJIUzI1NiJ9.xxx
	  nazhi honor types --token eyJhbGciOiJIUzI1NiJ9.xxx --base-url http://139.159.205.146:8280`,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		printVerbose("正在获取荣誉类型...")
		types, err := c.GetHonorTypes(cmd.Context(), token)
		if err != nil {
			printError(fmt.Errorf("获取荣誉类型失败: %w", err))
			return
		}

		printJSON(types)
	},
}

// honorListCmd 表示 nazhi honor list 命令
//
//	nazhi honor list --token <token> [--page <页>] [--page-size <条>] [--base-url <url>] [--timeout <秒>]
var honorListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取已申报荣誉记录",
	Long:  `获取当前用户已申报的全部荣誉记录（分页）。`,
	Example: `  nazhi honor list --token eyJhbGciOiJIUzI1NiJ9.xxx
	  nazhi honor list --token eyJhbGciOiJIUzI1NiJ9.xxx --page 1 --page-size 20`,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		pageNo, _ := cmd.Flags().GetInt("page")
		pageSize, _ := cmd.Flags().GetInt("page-size")

		printVerbose("正在获取荣誉记录...")
		records, pb, err := c.GetHonorList(cmd.Context(), token, pageNo, pageSize)
		if err != nil {
			printError(fmt.Errorf("获取荣誉记录失败: %w", err))
			return
		}

		printJSON(map[string]any{
			"total":     pb.TotalNum,
			"page":      pb.PageNo,
			"pageSize":  pb.PageSize,
			"totalPage": pb.TotalPage,
			"records":   records,
		})
	},
}

// honorAddCmd 表示 nazhi honor add 命令
//
//	nazhi honor add --token <token> --payload '<json>' [--base-url <url>] [--timeout <秒>]
var honorAddCmd = &cobra.Command{
	Use:   "add",
	Short: "申报荣誉",
	Long:  `申报一条荣誉。payload 是 addHonor 请求体 JSON，可用 @file.json 从文件读取。`,
	Example: `  nazhi honor add --token eyJhbGciOiJIUzI1NiJ9.xxx --payload '{"name":"校学生优秀干部","typeId":1147,"typeName":"校学生优秀干部","level":5,"evaluationAgency":"福清一中","getDate":"2026-06-30"}'
	  nazhi honor add --token eyJhbGciOiJIUzI1NiJ9.xxx --payload @honor.json`,
	Run: func(cmd *cobra.Command, args []string) {
		payloadRaw, _ := cmd.Flags().GetString("payload")

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}
		if payloadRaw == "" {
			printError(fmt.Errorf("--payload 为必填"))
			return
		}

		payload, err := parseAddHonorPayload(payloadRaw)
		if err != nil {
			printError(err)
			return
		}

		printVerbose("正在申报荣誉...")
		if err := c.AddHonor(cmd.Context(), token, *payload); err != nil {
			printError(fmt.Errorf("申报荣誉失败: %w", err))
			return
		}

		printJSON(map[string]string{
			"status": "success",
			"msg":    "荣誉申报成功",
		})
	},
}

// parseAddHonorPayload 从命令行参数解析 AddHonorPayload JSON。
// 支持 @file.json 语法从文件读取。
func parseAddHonorPayload(raw string) (*types.AddHonorPayload, error) {
	var payloadBytes []byte
	if strings.HasPrefix(raw, "@") {
		filePath := raw[1:]
		var err error
		payloadBytes, err = os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("读取 payload 文件失败: %w", err)
		}
	} else {
		payloadBytes = []byte(raw)
	}

	var payload types.AddHonorPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("解析 payload JSON 失败: %w", err)
	}
	return &payload, nil
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
	honorListCmd.Flags().Int("page-size", 20, "每页条数")
	registerBizFlags(honorListCmd)

	// honor add
	honorCmd.AddCommand(honorAddCmd)
	honorAddCmd.Flags().String("payload", "", "荣誉 JSON（必填，可用 @file.json 从文件读取）")
	registerBizFlags(honorAddCmd)
}
