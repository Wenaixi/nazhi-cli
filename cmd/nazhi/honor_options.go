package main

import (
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"github.com/spf13/cobra"
)

// honorTypeOptionsCmd 获取荣誉类型下拉选项。
//
// SDK GetHonorTypeOptions 读取 getHonorTypeForSelect 响应的 dataList；
// 它和 level-options 使用同一个平台接口，但返回的数据语义不同。
var honorTypeOptionsCmd = &cobra.Command{
	Use:     "type-options",
	Short:   "获取荣誉类型下拉选项",
	Long:    "获取前端荣誉申报表单使用的荣誉类型选项。",
	Example: "  nazhi honor type-options --token eyJhbGciOiJIUzI1NiJ9.xxx",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}
		printVerbose("正在获取荣誉类型下拉...")
		opts, err := c.GetHonorTypeOptions(cmd.Context(), token)
		if err != nil {
			printError(fmt.Errorf("获取荣誉类型下拉失败: %w", err))
			return
		}
		if opts == nil {
			opts = []types.HonorSelectOption{}
		}
		printEnvelope(envelope.Success(opts))
	},
}

// honorLevelOptionsCmd 获取通用荣誉等级下拉选项。
//
// SDK GetHonorTypeForSelect 读取同一接口响应的 returnData；
// 按荣誉类型联动的等级请使用 honor levels --type-id。
var honorLevelOptionsCmd = &cobra.Command{
	Use:     "level-options",
	Short:   "获取通用荣誉等级下拉选项",
	Long:    "获取前端荣誉申报表单使用的通用等级选项；按类型联动的等级请使用 honor levels。",
	Example: "  nazhi honor level-options --token eyJhbGciOiJIUzI1NiJ9.xxx",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}
		printVerbose("正在获取通用荣誉等级下拉...")
		opts, err := c.GetHonorTypeForSelect(cmd.Context(), token)
		if err != nil {
			printError(fmt.Errorf("获取通用荣誉等级下拉失败: %w", err))
			return
		}
		if opts == nil {
			opts = []types.HonorSelectOption{}
		}
		printEnvelope(envelope.Success(opts))
	},
}

func init() {
	honorCmd.AddCommand(honorTypeOptionsCmd)
	registerBizFlags(honorTypeOptionsCmd)

	honorCmd.AddCommand(honorLevelOptionsCmd)
	registerBizFlags(honorLevelOptionsCmd)
}
