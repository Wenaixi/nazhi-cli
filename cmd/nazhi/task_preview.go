package main

import (
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

var taskPreviewCmd = &cobra.Command{
	Use:   "preview",
	Short: "Preview task payload without submitting",
	Long: "Preview the final JSON payload for a task submit/edit without calling addCircle/editCircle.\n\nExposes SDK presets: circleTaskId/circleTypeId/dimensionId from GetCircleTypeByTaskId, hours auto-filled when task preset >0 and user leaves it empty, and pictureList merged from ImageIDs/ImagePaths. Empty address/orgName/level stay empty and are never filled with school name or \"5\". All user fields are Trimmed and sent as-is, matching frontend JSON.stringify(form).",
	Example: "  nazhi task preview --token xxx --payload '{\"taskId\":18154,\"content\":\"heart\"}'\n  nazhi task preview --token xxx --payload @task.json\n  echo '{\"id\":5400001,\"taskId\":18154,\"content\":\"fix\"}' | nazhi task preview --token xxx --payload - --edit",
	Run: func(cmd *cobra.Command, args []string) {
		payloadRaw, _ := cmd.Flags().GetString("payload")
		isEdit, _ := cmd.Flags().GetBool("edit")
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}
		if payloadRaw == "" {
			printEnvelope(envelope.Error(400, "--payload is required"))
			return
		}
		payloadBytes, err := parseJSONObjectPayload(payloadRaw)
		if err != nil {
			printParamError(fmt.Errorf("read payload failed: %w", err))
			return
		}
		if isEdit {
			input, err := decodeTaskEditInput(payloadBytes)
			if err != nil {
				printParamError(fmt.Errorf("parse payload JSON failed: %w", err))
				return
			}
			if v, _ := cmd.Flags().GetString("address"); v != "" {
				input.Address = v
			}
			if v, _ := cmd.Flags().GetString("level"); v != "" {
				input.Level = v
			}
			printVerbose("Previewing edit payload (auto-completing task metadata, not submitting)...")
			payload, err := c.PreviewEditPayload(cmd.Context(), token, input)
			if err != nil {
				printError(fmt.Errorf("preview edit payload failed: %w", err))
				return
			}
			printEnvelope(envelope.Success(payload))
			return
		}
		input, err := decodeTaskSubmitInput(payloadBytes)
		if err != nil {
			printParamError(fmt.Errorf("parse payload JSON failed: %w", err))
			return
		}
		if v, _ := cmd.Flags().GetString("address"); v != "" {
			input.Address = v
		}
		if v, _ := cmd.Flags().GetString("level"); v != "" {
			input.Level = v
		}
		printVerbose("Previewing submit payload (auto-completing task metadata, not submitting)...")
		payload, err := c.PreviewSubmitPayload(cmd.Context(), token, input)
		if err != nil {
			printError(fmt.Errorf("preview submit payload failed: %w", err))
			return
		}
		printEnvelope(envelope.Success(payload))
	},
}

func init() {
	registerBizFlags(taskPreviewCmd)
	taskPreviewCmd.Flags().String("payload", "", "Task JSON (required, use @file.json or - for stdin)")
	taskPreviewCmd.Flags().String("address", "", "Location override for payload.address; empty stays empty, no school name fallback")
	taskPreviewCmd.Flags().String("level", "", "Level code override; empty stays empty, no default 5")
	taskPreviewCmd.Flags().Bool("edit", false, "Preview edit mode (payload must include id)")
}
