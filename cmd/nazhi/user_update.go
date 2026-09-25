package main

import (
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
		runWriteOp(cmd, userUpdateWriteOp, nil)
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
// C2 收敛：键集统一小写存储（unknownUpdatePayloadKeys 对用户键 ToLower 后比较，
// 与 task/honor/typical-case 的允许集契约一致）——此前 camelCase 存储 +
// 大小写敏感查询导致 {"Telephone":...} 误判未知（drift），已修。
var userUpdateAllowedKeys = map[string]struct{}{
	"name": {}, "studentnumber": {}, "nationalstudentnumber": {},
	"telephone": {}, "familyaddress": {}, "hobbies": {},
	"gendername": {}, "youthleague": {}, "nationname": {}, "idcardtype": {},
	"idcard": {}, "birthday": {}, "birthdaystr": {}, "studentuuid": {}, "seat": {},
}
// userUpdateAllowedKeys 的未知键拒绝已收敛到 write_op_runner.go 的
// userUpdateWriteOp.rejectUnknown（unknownUpdatePayloadKeys + ToLower 折叠）。
// 原 unknownUserUpdateKeys 函数已删除（未导出、无生产引用，属死代码）。

func init() {
	rootCmd.AddCommand(userCmd)
	userCmd.AddCommand(userUpdateCmd)
	userCmd.AddCommand(userInfoCmd)

	userUpdateCmd.Flags().String("payload", "", "用户信息 JSON（必填，可用 @file.json 从文件读取，或 - 从 stdin 读取；支持 genderName/youthLeague 等友好字段）")
	registerBizFlags(userUpdateCmd)
	registerBizFlags(userInfoCmd)
}
