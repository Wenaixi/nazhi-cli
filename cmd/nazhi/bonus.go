package main

import (
	"encoding/json"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// bonusCmd 表示 nazhi bonus 父命令
var bonusCmd = &cobra.Command{
	Use:   "bonus",
	Short: "积分查询",
	Long:  `查询当月和历史积分信息。`,
}

// bonusMonthCmd 表示 nazhi bonus month 命令
var bonusMonthCmd = &cobra.Command{
	Use:   "month",
	Short: "获取当月积分",
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		printVerbose("正在获取当月积分...")
		raw, err := c.GetMonthBonus(cmd.Context(), token)
		if err != nil {
			printError(fmt.Errorf("获取当月积分失败: %w", err))
			return
		}
		printEnvelope(envelope.Success(json.RawMessage(raw)))
	},
}

// bonusHistoryCmd 表示 nazhi bonus history 命令
var bonusHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "获取历史积分",
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		termID, _ := cmd.Flags().GetInt64("term-id")
		month, _ := cmd.Flags().GetString("month")

		printVerbose("正在获取历史积分...")
		raw, err := c.GetHistoryBonus(cmd.Context(), token, termID, month)
		if err != nil {
			printError(fmt.Errorf("获取历史积分失败: %w", err))
			return
		}
		printEnvelope(envelope.Success(json.RawMessage(raw)))
	},
}

// bonusRankCmd 表示 nazhi bonus rank 命令
var bonusRankCmd = &cobra.Command{
	Use:   "rank",
	Short: "获取班级积分排行",
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		limit, _ := cmd.Flags().GetInt("limit")
		if limit <= 0 {
			limit = 10
		}

		printVerbose("正在获取积分排行...")
		raw, err := c.GetBonusRank(cmd.Context(), token, limit)
		if err != nil {
			printError(fmt.Errorf("获取积分排行失败: %w", err))
			return
		}
		printEnvelope(envelope.Success(json.RawMessage(raw)))
	},
}

// bonusDetailCmd 表示 nazhi bonus detail 命令
var bonusDetailCmd = &cobra.Command{
	Use:   "detail",
	Short: "获取积分明细",
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		limit, _ := cmd.Flags().GetInt("limit")
		if limit <= 0 {
			limit = 18
		}

		printVerbose("正在获取积分明细...")
		raw, err := c.GetBonusDetail(cmd.Context(), token, limit)
		if err != nil {
			printError(fmt.Errorf("获取积分明细失败: %w", err))
			return
		}
		printEnvelope(envelope.Success(json.RawMessage(raw)))
	},
}

func init() {
	rootCmd.AddCommand(bonusCmd)

	bonusCmd.AddCommand(bonusMonthCmd)
	registerBizFlags(bonusMonthCmd)

	bonusCmd.AddCommand(bonusHistoryCmd)
	bonusHistoryCmd.Flags().Int64("term-id", 0, "学期 ID")
	bonusHistoryCmd.Flags().String("month", "", "月份（如 2026-07）")
	registerBizFlags(bonusHistoryCmd)

	bonusCmd.AddCommand(bonusRankCmd)
	bonusRankCmd.Flags().Int("limit", 10, "限制条数")
	registerBizFlags(bonusRankCmd)

	bonusCmd.AddCommand(bonusDetailCmd)
	bonusDetailCmd.Flags().Int("limit", 18, "限制条数")
	registerBizFlags(bonusDetailCmd)
}
