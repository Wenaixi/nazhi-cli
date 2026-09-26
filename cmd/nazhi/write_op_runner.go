package main

// write_op_runner.go：CLI 写操作命令的共享控制流 runner。
//
// 架构深化（C2）：task submit/edit/preview、honor add/update、
// typical-case submit/update、user update 此前各内联一份 6 步骨架
// （读 payload → 判空 → 建客户端 → 解析 → 拒未知键 → 解码 → 覆盖 flag →
// 调用 → envelope），submit 与 edit 33 行逐字重复、preview 内部分叉两份、
// honor/typical-case/user 同形不同序。收敛后写操作只有一处控制流实现：
// 改错误次序、envelope 形状或未知键契约时只改这里，命令不再需要手工同步。
//
// 与 circleListMode（列表族）的关键差异：写操作族保持「先本地校验再建
// 客户端」——缺 --payload / 坏 payload / 未知键等参数错误不应依赖
// token/base-url 配置是否正确（P2-E 十三域审计确立的不变式）。
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"github.com/spf13/cobra"
)

// writeOpMode 描述一个写操作命令（或分支）的领域差异。
type writeOpMode struct {
	// verboseMsg 是调用 SDK 前的进度文案。
	verboseMsg string
	// errorPrefix 是 SDK 调用失败时的错误文案前缀。
	errorPrefix string

	// allowedKeys 是本命令 payload 允许的顶层键集合（小写存储）。
	// 未知键拒绝由 runWriteOp 单点调用 unknownUpdatePayloadKeys 完成，
	// 各实例不再各自包一层同形闭包——9 个实例原本都是同一个 helper 的
	// 同形包装，只差一个允许集。
	allowedKeys map[string]struct{}
	// helpKeys 是本命令 payload 允许键的用户书写形态（驼峰原样），用于
	// 生成 --help 的允许键清单。
	//
	// 为什么与 allowedKeys 分开存两份：allowedKeys 小写存储只为大小写不敏感
	// 比较（unknownUpdatePayloadKeys 对用户键 ToLower 后比对，避免
	// {"Telephone":...} 被误判未知）。若直接把它的键印进 --help，用户会照抄
	// 全小写形态写出 typeid 这类与出站 JSON 不符的键。helpKeys 是给人看的、
	// 与 payload 出站 json 键逐字一致；两者折叠小写后必须完全相等，
	// 由 TestWriteOpMode_HelpKeysCoverAllModes 锁死。
	helpKeys []string
	// decode 将 payload 解码为命令的输入类型。
	// 返回 (输入对象, 错误)；解码失败以参数错误拒绝。
	decode func(payloadBytes []byte) (any, error)
	// validateID 校验解码后输入的 id（update 命令必填正数 id）。
	// 返回错误消息；nil 表示通过。错误以参数错误（400/exit3）拒绝。
	// 无 id 校验需求的命令（submit/add/delete）传 nil。
	validateID func(decoded any) error
	// 无覆盖需求的命令传 nil。
	applyFlags func(cmd *cobra.Command, decoded any)
	// call 调用 SDK 并返回业务结果。返回 (result, error)，result 传给 success。
	call func(ctx context.Context, c *client.Client, token string, decoded any) (any, error)
	// success 将 SDK 成功结果包装为 envelope：有负载走 Success(result)，
	// 无负载（Add*/Update* 返回 nil）走 Empty(msg)。
	success func(result any) *envelope.Envelope
}

// writeOpBranch 是按 flag 分支的命令配置：branch 根据命令状态返回当前分支的
// writeOpMode。task preview 用它在 --edit / 提交分支间切换。
// nil 表示无分支（单模式命令）。
type writeOpBranch func(cmd *cobra.Command) writeOpMode

// applyAddressLevelFlags 统一 --address/--level 覆盖逻辑：读取两个 flag 值，
// 非空时经 apply 回调写入已解码输入。task 写操作族四个分支
// （submit/edit/previewSubmit/previewEdit）共用，此前 4 份逐字重复闭包
// 的 flag 读取与覆盖判定收敛到本函数单点。
//
// flag 未注册时 pflag 的 GetString 返回错误，此处按既有契约忽略并取零值
// 空串——未注册 flag 视为未提供，保留 payload 原值（与测试锁定的行为一致）。
func applyAddressLevelFlags(cmd *cobra.Command, apply func(address, level string)) {
	address, _ := cmd.Flags().GetString("address")
	level, _ := cmd.Flags().GetString("level")
	apply(address, level)
}

// attachAllowedKeysHelp 把命令的 payload 允许键清单写进 Long 帮助文本。
//
// 必须在命令构造期（init 或 var 初始化）调用，而不是在 Run 里：cobra 只在
// 触发 --help 时渲染 Long，Run 根本不会执行。此前的未知键拒绝文案写着
// 「允许键见 nazhi <cmd> --help」，而所有写操作命令的 Long 里都没有键名——
// 用户被指向一条死路，只能去翻 Go 源码或反编译二进制。
//
// allowNoneNote 为真时额外说明键均可省略（honor add 等无必填键的命令）。
func attachAllowedKeysHelp(cmd *cobra.Command, keys []string, allowNoneNote bool) {
	section := helpSection(keys, allowNoneNote)
	if section == "" {
		return
	}
	if cmd.Long == "" {
		cmd.Long = section
		return
	}
	cmd.Long = cmd.Long + "\n\n" + section
}

// runWriteOp 执行写操作命令的完整控制流。
// 错误优先次序（用户可见契约，由 write_op_skeleton_test.go 锁定）：
//  1. 缺 --payload → envelope.Error(400, "--payload 为必填")
//  2. buildBizClient 失败 → printParamError（参数错误）
//  3. payload 非对象 / 解析失败 → printParamError("读取 payload 失败")
//  4. 未知键 → printParamError("payload 含未知键: %v")
//  5. 解码失败 → printParamError("解析 payload JSON 失败")
//  6. SDK 调用失败 → printError（按哨兵映射退出码）

// 未知键/坏 payload/缺 payload 路径不发任何业务请求（含元数据预热）。
func runWriteOp(cmd *cobra.Command, mode writeOpMode, branch writeOpBranch) {
	payloadRaw, _ := cmd.Flags().GetString("payload")
	if payloadRaw == "" {
		printParamError(errors.New("--payload 为必填"))
		return
	}

	c, token, err := buildBizClient(cmd)
	if err != nil {
		printParamError(err)
		return
	}

	payloadBytes, err := parseJSONObjectPayload(cmd.Context(), payloadRaw)
	if err != nil {
		printParamError(fmt.Errorf("读取 payload 失败: %w", err))
		return
	}

	// 分支决策必须在未知键校验之后、解码之前（错误优先次序不变）。
	m := mode
	if branch != nil {
		m = branch(cmd)
	}

	if unknown := unknownUpdatePayloadKeys(payloadBytes, m.allowedKeys); len(unknown) > 0 {
		printParamError(unknownKeyErrorFor(cmd, unknown))
		return
	}

	decoded, err := m.decode(payloadBytes)
	if err != nil {
		printParamError(fmt.Errorf("解析 payload JSON 失败: %w", err))
		return
	}

	// update 命令的 id 校验：缺 id 或非正数以参数错误拒绝，不发业务请求。
	// 位置在未知键之后、applyFlags 之前（与 honor/typical update 原次序一致）。
	if m.validateID != nil {
		if idErr := m.validateID(decoded); idErr != nil {
			printParamError(errors.New(idErr.Error()))
			return
		}
	}
	if m.applyFlags != nil {
		m.applyFlags(cmd, decoded)
	}

	printVerbose("%s", m.verboseMsg)
	result, err := m.call(cmd.Context(), c, token, decoded)
	if err != nil {
		printError(fmt.Errorf("%s: %w", m.errorPrefix, err))
		return
	}

	printEnvelope(m.success(result))
}

// unknownKeyErrorFor 构造未知键拒绝的错误，保证文案里的命令名始终非空。
// 抽出成函数是为了让「承诺指向 --help」这条关系可被测试直接断言，
// 并与 attachAllowedKeysHelp 写进帮助的键清单指向同一份真相源。
func unknownKeyErrorFor(cmd *cobra.Command, unknown []string) error {
	name := cmd.Name()
	if name == "" {
		name = "对应命令"
	}
	return fmt.Errorf("payload 含未知键: %v（允许键见 nazhi %s --help）", unknown, name)
}

// ─── task 写操作族（submit/edit）───

// taskSubmitWriteOp 是 task submit 的写操作配置。
var taskSubmitWriteOp = writeOpMode{
	verboseMsg:  "正在提交任务（自动补全任务元数据/图片上传）...",
	errorPrefix: "提交任务失败",
	allowedKeys: taskInputAllowedKeys,
	helpKeys:    taskInputKeysAll.display(),
	decode: func(payloadBytes []byte) (any, error) {
		input, err := decodeTaskSubmitInput(payloadBytes)
		if err != nil {
			return nil, err
		}
		return &input, nil
	},
	applyFlags: func(cmd *cobra.Command, decoded any) {
		input := decoded.(*types.TaskSubmitInput)
		applyAddressLevelFlags(cmd, func(address, level string) {
			if address != "" {
				input.Address = address
			}
			if level != "" {
				input.Level = level
			}
		})
	},
	call: func(ctx context.Context, c *client.Client, token string, decoded any) (any, error) {
		return c.SubmitTask(ctx, token, *decoded.(*types.TaskSubmitInput))
	},
	success: envelope.Success,
}

// taskEditWriteOp 是 task edit 的写操作配置。
var taskEditWriteOp = writeOpMode{
	verboseMsg:  "正在修改写实记录（自动补全任务元数据/图片上传）...",
	errorPrefix: "修改写实记录失败",
	allowedKeys: taskInputAllowedKeys,
	helpKeys:    taskInputKeysAll.display(),
	decode: func(payloadBytes []byte) (any, error) {
		input, err := decodeTaskEditInput(payloadBytes)
		if err != nil {
			return nil, err
		}
		return &input, nil
	},
	applyFlags: func(cmd *cobra.Command, decoded any) {
		input := decoded.(*types.TaskEditInput)
		applyAddressLevelFlags(cmd, func(address, level string) {
			if address != "" {
				input.Address = address
			}
			if level != "" {
				input.Level = level
			}
		})
	},
	call: func(ctx context.Context, c *client.Client, token string, decoded any) (any, error) {
		return c.EditCircle(ctx, token, *decoded.(*types.TaskEditInput))
	},
	success: envelope.Success,
}

// taskPreviewSubmitBranch / taskPreviewEditBranch 是 task preview 的两个分支：
// 提交分支（--edit 未设）与编辑分支（--edit 设）。
// 分支决策在 runWriteOp 的 branch 中按 --edit 切换。

// taskPreviewBranch 返回 task preview 当前分支的配置。
func taskPreviewBranch(cmd *cobra.Command) writeOpMode {
	if isEdit, _ := cmd.Flags().GetBool("edit"); isEdit {
		return taskPreviewEditWriteOp
	}
	return taskPreviewSubmitWriteOp
}

// taskPreviewSubmitWriteOp 是 task preview 提交分支。
var taskPreviewSubmitWriteOp = writeOpMode{
	verboseMsg:  "正在预览提交 payload（自动补齐任务元数据，不提交）...",
	errorPrefix: "预览提交 payload 失败",
	allowedKeys: taskInputAllowedKeys,
	helpKeys:    taskInputKeysAll.display(),
	decode: func(payloadBytes []byte) (any, error) {
		input, err := decodeTaskSubmitInput(payloadBytes)
		if err != nil {
			return nil, err
		}
		return &input, nil
	},
	applyFlags: func(cmd *cobra.Command, decoded any) {
		input := decoded.(*types.TaskSubmitInput)
		applyAddressLevelFlags(cmd, func(address, level string) {
			if address != "" {
				input.Address = address
			}
			if level != "" {
				input.Level = level
			}
		})
	},
	call: func(ctx context.Context, c *client.Client, token string, decoded any) (any, error) {
		return c.PreviewSubmitPayload(ctx, token, *decoded.(*types.TaskSubmitInput))
	},
	success: envelope.Success,
}

// taskPreviewEditWriteOp 是 task preview 编辑分支。
var taskPreviewEditWriteOp = writeOpMode{
	verboseMsg:  "正在预览编辑 payload（自动补齐任务元数据，不提交）...",
	errorPrefix: "预览编辑 payload 失败",
	allowedKeys: taskInputAllowedKeys,
	helpKeys:    taskInputKeysAll.display(),
	decode: func(payloadBytes []byte) (any, error) {
		input, err := decodeTaskEditInput(payloadBytes)
		if err != nil {
			return nil, err
		}
		return &input, nil
	},
	applyFlags: func(cmd *cobra.Command, decoded any) {
		input := decoded.(*types.TaskEditInput)
		applyAddressLevelFlags(cmd, func(address, level string) {
			if address != "" {
				input.Address = address
			}
			if level != "" {
				input.Level = level
			}
		})
	},
	call: func(ctx context.Context, c *client.Client, token string, decoded any) (any, error) {
		return c.PreviewEditPayload(ctx, token, *decoded.(*types.TaskEditInput))
	},
	success: envelope.Success,
}

// ─── honor add / typical-case submit / user update ───

// honorAddWriteOp 是 honor add 的写操作配置。
// 解码走 json.Unmarshal 到 AddHonorPayload（无 CLI 归一化），
// 成功返回 nil → envelope.Empty("荣誉申报成功")。
var honorAddWriteOp = writeOpMode{
	verboseMsg:  "正在申报荣誉...",
	errorPrefix: "申报荣誉失败",
	allowedKeys: honorAddAllowedKeys,
	helpKeys:    honorAddKeys.display(),
	decode: func(payloadBytes []byte) (any, error) {
		var payload types.AddHonorPayload
		if err := json.Unmarshal(payloadBytes, &payload); err != nil {
			return nil, err
		}
		return &payload, nil
	},
	call: func(ctx context.Context, c *client.Client, token string, decoded any) (any, error) {
		return nil, c.AddHonor(ctx, token, *decoded.(*types.AddHonorPayload))
	},
	success: func(result any) *envelope.Envelope {
		return envelope.Empty("荣誉申报成功")
	},
}

// typicalCaseSubmitWriteOp 是 typical-case submit 的写操作配置。
// 与 honor add 同构：json.Unmarshal 到 AddTypicalCasePayload，成功 Empty。
var typicalCaseSubmitWriteOp = writeOpMode{
	verboseMsg:  "正在提交典型案例...",
	errorPrefix: "提交典型案例失败",
	allowedKeys: typicalCaseAddAllowedKeys,
	helpKeys:    typicalCaseSubmitKeys.display(),
	decode: func(payloadBytes []byte) (any, error) {
		var payload types.AddTypicalCasePayload
		if err := json.Unmarshal(payloadBytes, &payload); err != nil {
			return nil, err
		}
		return &payload, nil
	},
	call: func(ctx context.Context, c *client.Client, token string, decoded any) (any, error) {
		return nil, c.AddTypicalCase(ctx, token, *decoded.(*types.AddTypicalCasePayload))
	},
	success: func(result any) *envelope.Envelope {
		return envelope.Empty("典型案例提交成功")
	},
}

// userUpdateWriteOp 是 user update 的写操作配置。
// 解码走 json.Unmarshal 到 UserUpdateInput，成功 Empty。
var userUpdateWriteOp = writeOpMode{
	verboseMsg:  "正在更新个人信息...",
	errorPrefix: "更新个人信息失败",
	allowedKeys: userUpdateAllowedKeys,
	helpKeys:    userUpdateKeys.display(),
	decode: func(payloadBytes []byte) (any, error) {
		var input types.UserUpdateInput
		if err := json.Unmarshal(payloadBytes, &input); err != nil {
			return nil, err
		}
		return &input, nil
	},
	call: func(ctx context.Context, c *client.Client, token string, decoded any) (any, error) {
		return nil, c.UpdateMyInfoStructured(ctx, token, *decoded.(*types.UserUpdateInput))
	},
	success: func(result any) *envelope.Envelope {
		return envelope.Empty("个人信息更新成功")
	},
}

// honorUpdateWriteOp 是 honor update 的写操作配置。
// 与 typical-case update 孪生：map payload + 未知键拒绝 + 正数 id 校验 +
// Empty 成功，仅允许集/方法/文案不同。
var honorUpdateWriteOp = writeOpMode{
	verboseMsg:  "正在更新荣誉记录...",
	errorPrefix: "更新荣誉记录失败",
	allowedKeys: honorUpdateAllowedKeys,
	helpKeys:    honorUpdateKeys.display(),
	decode: func(payloadBytes []byte) (any, error) {
		var payload map[string]any
		if err := json.Unmarshal(payloadBytes, &payload); err != nil {
			return nil, err
		}
		return &payload, nil
	},
	validateID: func(decoded any) error {
		if !PayloadPositiveIDValid(*decoded.(*map[string]any)) {
			return fmt.Errorf("payload 必须包含正数 id 字段")
		}
		return nil
	},
	call: func(ctx context.Context, c *client.Client, token string, decoded any) (any, error) {
		return nil, c.UpdateHonor(ctx, token, *decoded.(*map[string]any))
	},
	success: func(result any) *envelope.Envelope {
		return envelope.Empty("荣誉记录更新成功")
	},
}

// typicalCaseUpdateWriteOp 是 typical-case update 的写操作配置。
// 与 honor update 孪生，仅允许集/方法/文案不同。
var typicalCaseUpdateWriteOp = writeOpMode{
	verboseMsg:  "正在更新典型案例...",
	errorPrefix: "更新典型案例失败",
	allowedKeys: typicalCaseUpdateAllowedKeys,
	helpKeys:    typicalCaseUpdateKeys.display(),
	decode: func(payloadBytes []byte) (any, error) {
		var payload map[string]any
		if err := json.Unmarshal(payloadBytes, &payload); err != nil {
			return nil, err
		}
		return &payload, nil
	},
	validateID: func(decoded any) error {
		if !PayloadPositiveIDValid(*decoded.(*map[string]any)) {
			return fmt.Errorf("payload 必须包含正数 id 字段")
		}
		return nil
	},
	call: func(ctx context.Context, c *client.Client, token string, decoded any) (any, error) {
		return nil, c.UpdateTypicalCase(ctx, token, *decoded.(*map[string]any))
	},
	success: func(result any) *envelope.Envelope {
		return envelope.Empty("典型案例更新成功")
	},
}
