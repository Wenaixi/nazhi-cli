package main

import (
	"fmt"

	"github.com/Wenaixi/nazhi-cli/internal/version"
	"github.com/spf13/cobra"
)

// versionCmd 表示 nazhi version 命令
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本信息",
	Long:  `显示 nazhi-cli 当前版本号。`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.Version)
	},
}
