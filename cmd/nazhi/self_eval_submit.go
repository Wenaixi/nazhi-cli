package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// selfEvalSubmitCmd 表示 nazhi self-eval submit 命令
//
//	nazhi self-eval submit --token <token> --comment "<评价>" [--base-url <url>] [--timeout <秒>]
//	nazhi self-eval submit --token <token> --payload '<json>'    # 结构化提交
var selfEvalSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "提交自我评价",
	Long: `提交自我评价文本。支持两种模式：

  1. 纯文本模式：--comment 参数（默认）。如果 --comment 为空或为 "-"，则从 stdin 读取。
  2. 结构化模式：--payload 参数，接收 JSON 对象（对应前端"诉得失"页面的完整结构化评价）。
     SDK 内部自动做双层 JSON 包装后提交。`,
	Example: `  nazhi self-eval submit --token eyJhbGciOiJIUzI1NiJ9.xxx --comment "很好的学期"
  nazhi self-eval submit --token eyJhbGciOiJIUzI1NiJ9.xxx --comment "-"
  nazhi self-eval submit --token xxx --payload '{"bxqhzr":"会做人目标","bxqbx":"表现","bxqys":"优势"}'`,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		payloadRaw, _ := cmd.Flags().GetString("payload")
		comment, _ := cmd.Flags().GetString("comment")

		// 结构化模式：--payload 优先
		if payloadRaw != "" {
			payloadBytes, err := parsePayloadFromArg(payloadRaw)
			if err != nil {
				printError(fmt.Errorf("读取 payload 失败: %w", err))
				return
			}
			var form map[string]any
			if err := json.Unmarshal(payloadBytes, &form); err != nil {
				printError(fmt.Errorf("解析 payload JSON 失败: %w", err))
				return
			}

			printVerbose("正在提交结构化自我评价...")
			if err := c.SubmitSelfEvaluationStructured(cmd.Context(), token, form); err != nil {
				printError(fmt.Errorf("提交结构化自我评价失败: %w", err))
				return
			}
			printEnvelope(envelope.Empty("结构化自我评价提交成功"))
			return
		}

		// 纯文本模式：--comment
		if comment == "" || comment == "-" {
			if isTerminalStdin() {
				printPrompt("请输入自我评价内容（Ctrl+D 结束）: ")
			}
			var readErr error
			comment, readErr = readStdinWithTimeout(cmd.Context(), 60)
			if readErr != nil {
				printError(fmt.Errorf("读取 stdin 评价内容失败: %w", readErr))
				return
			}
			if comment == "" {
				printEnvelope(envelope.Error(400, "评价内容不能为空"))
				return
			}
		}

		printVerbose("正在提交自我评价...")
		err = c.SubmitSelfEvaluation(cmd.Context(), token, comment)
		if err != nil {
			printError(fmt.Errorf("提交自我评价失败: %w", err))
			return
		}

		printEnvelope(envelope.Empty("自我评价提交成功"))
	},
}

func init() {
	registerBizFlags(selfEvalSubmitCmd)
	selfEvalSubmitCmd.Flags().String("comment", "", "评价文本（空或 - 则从 stdin 读取）")
	selfEvalSubmitCmd.Flags().String("payload", "", "结构化评价 JSON（与 --comment 互斥，可用 @file.json 或 - 读取）")
}

// readStdinWithTimeout 从 stdin 读取一行内容，超过 timeoutSec 秒未完成则返回超时错误。
// 当 stdin 是管道且对端未关闭时，原始 reader.ReadString(0) 会无限阻塞，
// 此函数通过 goroutine + select 实现超时保护。
func readStdinWithTimeout(ctx context.Context, timeoutSec int) (string, error) {
	type result struct {
		text string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString(0)
		ch <- result{strings.TrimSpace(input), err}
	}()

	timer := time.NewTimer(time.Duration(timeoutSec) * time.Second)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case res := <-ch:
		if res.err != nil && res.err != io.EOF {
			return "", res.err
		}
		return res.text, nil
	case <-timer.C:
		return "", fmt.Errorf("stdin 读取超时（%d 秒无输入），请通过 --comment 参数直接传入内容", timeoutSec)
	}
}
