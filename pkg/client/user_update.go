package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// 性别映射：中文名称 → API 数字代码
// 服务端新增类型时，在此添加一行即可
var genderMap = map[string]int{
	"男": 1,
	"女": 2,
}

// 团员状态映射：中文名称 → API 数字代码
var youthLeagueMap = map[string]int{
	"是": 1,
	"否": 0,
}

// 民族映射：中文名称 → API 数字代码
// 服务端新增民族时，在此添加一行即可
var nationMap = map[string]int{
	"汉族":   1,
	"满族":   2,
	"维吾尔族": 3,
	"畲族":   4,
	"回族":   5,
	"壮族":   6,
	"土家族":  7,
	"苗族":   8,
}

// 证件类型映射：中文名称 → API 数字代码
// 服务端新增证件类型时，在此添加一行即可
var idCardTypeMap = map[string]int{
	"中国居民身份证":     1,
	"外国人永久居留身份证":  2,
	"港澳居民来往内地通行证": 3,
	"港澳台居民居住证":    4,
	"台湾居民来往大陆通行证": 5,
	"护照":          6,
	"香港永久性居民身份证":  7,
}

// UpdateMyInfo 更新当前用户个人信息。
// POST /api/studentInfo/updateMyInfo
// updates 是 map，只传需要修改的字段，如：
//
//	{"telephone": "138xxx", "familyAddress": "福建省福州市", "hobbies": "阅读"}
//
// 可写字段为更新接口的线格式键名（studentName/studentNumber/telephone/gender
// 数字码/youthLeagueFlag/nation/idType/idCard/birthday/seat/studentUuid 等）——
// 注意与读接口 UserInfo 的 json tag 不同形（读侧是 name/genderName，写侧是
// studentName/gender 数字），照抄 UserInfo tag 会静默无效。中文枚举转数字码
// 请改用 UpdateMyInfoStructured。
//
// 成功后会失效 sm.cachedUserInfo：ActivateSession 步骤 4 缓存的 UserInfo
// 不再对 GetMyInfo 的持锁 fast path 可见，下次 GetMyInfo 会重新拉取。
func (c *Client) UpdateMyInfo(ctx context.Context, token string, updates map[string]any) error {
	if err := c.doBizVoid(ctx, token, "UpdateMyInfo",
		"/api/studentInfo/updateMyInfo", http.MethodPost, updates); err != nil {
		return err
	}
	c.InvalidateCachedUserInfo()
	return nil
}

// InvalidateCachedUserInfo 清空 session 缓存的 UserInfo。
//
// 供调用方在绕过 UpdateMyInfo 直接改服务端用户资料后，主动让本地缓存失效。
// UpdateMyInfo / UpdateMyInfoStructured 成功路径已自动调用本方法。
func (c *Client) InvalidateCachedUserInfo() {
	c.sm.InvalidateCachedUserInfo()
}

// UpdateMyInfoStructured 使用面向用户的字段名更新个人信息。
//
// 与 UpdateMyInfo 的区别：接收友好字段名（如 GenderName="男"），
// SDK 内部自动转换为 API 数字代码（如 gender=1）。
// 零值/空串字段跳过，不会覆盖服务端已有值。
//
// 支持的字段转换：
//   - GenderName → gender
//   - YouthLeague → youthLeagueFlag
//   - NationName → nation
//   - IdCardType → idType
//
// 用户可编辑透传：Telephone / FamilyAddress / Hobbies / IDCard / Birthday / BirthdayStr / Seat / StudentUuid
// Birthday 优先对应前端 birthday 键；BirthdayStr 仅作为旧调用兼容字段。
// 可选高级：Name→studentName、StudentNumber
// 不写入：NationalStudentNumber（前端只读，Structured 忽略以免误改学籍）
func (c *Client) UpdateMyInfoStructured(ctx context.Context, token string, input types.UserUpdateInput) error {
	// USER-1：全零输入视为 no-op——不组装、不发仅含 studentUuid 的空 POST、
	// 不失效本地缓存（CLI --payload '{}' 即触发此路径；前端无零字段提交入口）。
	if input == (types.UserUpdateInput{}) {
		return nil
	}
	updates := make(map[string]any, 16)

	// 基础身份（可选高级回传；前端主路径不编辑）
	if input.Name != "" {
		updates["studentName"] = input.Name // 前端 API 键 studentName
	}
	if input.StudentNumber != "" {
		updates["studentNumber"] = input.StudentNumber
	}
	// 故意忽略 input.NationalStudentNumber（modifyBox 中 :disabled="true"）

	// 联系方式（直接透传）
	if input.Telephone != "" {
		updates["telephone"] = input.Telephone
	}
	if input.FamilyAddress != "" {
		updates["familyAddress"] = input.FamilyAddress
	}
	if input.Hobbies != "" {
		updates["hobbies"] = input.Hobbies
	}

	// 证件号与生日（直接透传）。Birthday 优先，BirthdayStr 兼容旧调用。
	if input.IDCard != "" {
		updates["idCard"] = input.IDCard
	}
	birthday := input.Birthday
	if birthday == "" {
		birthday = input.BirthdayStr
	}
	if birthday != "" {
		updates["birthday"] = birthday
	}

	// 座号
	if input.Seat > 0 {
		updates["seat"] = input.Seat
	}

	// StudentUuid（密码）：直接透传，不跳过空串——空串表示不修改密码
	updates["studentUuid"] = input.StudentUuid

	// ── 字段转换：面向用户的中文名称 → API 数字代码 ──

	// GenderName → gender
	if input.GenderName != "" {
		code, ok := genderMap[input.GenderName]
		if !ok {
			return fmt.Errorf("%w: 不支持的性别值 %q", ErrInvalidPayload, input.GenderName)
		}
		updates["gender"] = code
	}

	// YouthLeague → youthLeagueFlag
	if input.YouthLeague != "" {
		code, ok := youthLeagueMap[input.YouthLeague]
		if !ok {
			return fmt.Errorf("%w: 不支持的团员值 %q", ErrInvalidPayload, input.YouthLeague)
		}
		updates["youthLeagueFlag"] = code
	}

	// NationName → nation
	if input.NationName != "" {
		code, ok := nationMap[input.NationName]
		if !ok {
			return fmt.Errorf("%w: 不支持的民族值 %q", ErrInvalidPayload, input.NationName)
		}
		updates["nation"] = code
	}

	// IdCardType → idType
	if input.IdCardType != "" {
		code, ok := idCardTypeMap[input.IdCardType]
		if !ok {
			return fmt.Errorf("%w: 不支持的证件类型值 %q", ErrInvalidPayload, input.IdCardType)
		}
		updates["idType"] = code
	}

	return c.UpdateMyInfo(ctx, token, updates)
}
