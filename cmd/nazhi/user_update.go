package main

import (
	"encoding/json"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"github.com/spf13/cobra"
)

// userCmd 表示 nazhi user 父命令
var userCmd = &cobra.Command{
	Use:   "user",
	Short: "用户信息管理",
	Long:  `查看和更新用户个人信息。`,
}

// userUpdateCmd 表示 nazhi user update 命令
//
// payload 解析为 types.UserUpdateInput，经 UpdateMyInfoStructured 提交：
// 友好键（genderName/youthLeague/nationName/idCardType）自动 remap 为 API 数字代码。
// 禁止裸 map 直接调 UpdateMyInfo，否则友好键会原样发给服务端被忽略。
var userUpdateCmd = &cobra.Command{
	Use:     "update",
	Short:   "更新个人信息",
	Long:    `更新当前用户的个人信息。payload 可用 @file.json 读取或 - 从 stdin 读取。支持友好字段名（如 genderName="男"），SDK 内部转换为 API 代码。`,
	Example: `  nazhi user update --token xxx --payload '{"telephone":"13800138000","familyAddress":"福建省福州市","genderName":"男"}'`,
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		payloadRaw, _ := cmd.Flags().GetString("payload")
		if payloadRaw == "" {
			printEnvelope(envelope.Error(400, "--payload 为必填"))
			return
		}

		payloadBytes, err := parseJSONObjectPayload(cmd.Context(), payloadRaw)
		if err != nil {
			printParamError(fmt.Errorf("读取 payload 失败: %w", err))
			return
		}
		var input types.UserUpdateInput
		if err := json.Unmarshal(payloadBytes, &input); err != nil {
			printParamError(fmt.Errorf("解析 payload JSON 失败: %w", err))
			return
		}
		// P2-3（19 轮审计 user-info）：未知顶层键静默丢弃会让拼错键名（如 telephoneX）
		// 走全零 no-op 分支输出 204 成功但服务端零修改。解码后校验键集合，
		// 未知键以参数错误拒绝（400/exit3），与 --payload 顶层对象校验互补。
		if unknown := unknownUserUpdateKeys(payloadBytes); len(unknown) > 0 {
			printParamError(fmt.Errorf("payload 含未知键: %v（允许键见 nazhi user update --help）", unknown))
			return
		}

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}

		printVerbose("正在更新个人信息...")
		if err := c.UpdateMyInfoStructured(cmd.Context(), token, input); err != nil {
			printError(fmt.Errorf("更新个人信息失败: %w", err))
			return
		}
		printEnvelope(envelope.Empty("个人信息更新成功"))
	},
}

// userInfoCmd 表示 nazhi user info 命令（whoami 别名）
var userInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "查看个人信息（whoami 别名）",
	Args:  cobra.NoArgs,
	Run:   whoamiCmd.Run,
}

// userUpdateAllowedKeys 是 UserUpdateInput 的全部顶层 JSON 键（含只读忽略的
// nationalStudentNumber——Structured 忽略它但允许显式传入以对齐前端整表 stringify）。
var userUpdateAllowedKeys = map[string]struct{}{
	"name": {}, "studentNumber": {}, "nationalStudentNumber": {},
	"telephone": {}, "familyAddress": {}, "hobbies": {},
	"genderName": {}, "youthLeague": {}, "nationName": {}, "idCardType": {},
	"idCard": {}, "birthday": {}, "birthdayStr": {}, "studentUuid": {}, "seat": {},
}

// unknownUserUpdateKeys 返回 payload 顶层 JSON 中不在允许键集合内的键名（含重复/空串归一）。
// 收敛后（C2 架构深化）：直接复用 unknownUpdatePayloadKeys（ToLower 折叠 +
// 稳定排序）。此前本函数用原始键直查 camelCase 允许集（大小写敏感），
// 注释声称与 task 族 EqualFold 语义对齐实际没对齐——{"Telephone":...} 会
// 被误判为未知键。收敛后大小写变体折叠放行，与 task/honor/typical-case 一致。
func unknownUserUpdateKeys(payloadBytes []byte) []string {
	return unknownUpdatePayloadKeys(payloadBytes, userUpdateAllowedKeys)
}

func init() {
	rootCmd.AddCommand(userCmd)
	userCmd.AddCommand(userUpdateCmd)
	userCmd.AddCommand(userInfoCmd)

	userUpdateCmd.Flags().String("payload", "", "用户信息 JSON（必填，可用 @file.json 从文件读取，或 - 从 stdin 读取；支持 genderName/youthLeague 等友好字段）")
	registerBizFlags(userUpdateCmd)
	registerBizFlags(userInfoCmd)
}
