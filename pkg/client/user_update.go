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
// 可用字段参考 types.UserInfo 中的 json tag 名。
func (c *Client) UpdateMyInfo(ctx context.Context, token string, updates map[string]any) error {
	_, err := c.doBizAndDecode(ctx, token, "UpdateMyInfo",
		"/api/studentInfo/updateMyInfo", http.MethodPost, updates)
	if err != nil {
		return fmt.Errorf("UpdateMyInfo 失败: %w", err)
	}
	return nil
}

// UpdateMyInfoStructured 使用面向用户的字段名更新个人信息（v1.4.0 新增）。
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
// 直接透传的字段（不做转换）：
//   - Name / StudentNumber / NationalStudentNumber
//   - Telephone / FamilyAddress / Hobbies
//   - IDCard / BirthdayStr / Seat
func (c *Client) UpdateMyInfoStructured(ctx context.Context, token string, input types.UserUpdateInput) error {
	updates := make(map[string]any, 16)

	// 基础身份（直接透传）
	if input.Name != "" {
		updates["name"] = input.Name
	}
	if input.StudentNumber != "" {
		updates["studentNumber"] = input.StudentNumber
	}
	if input.NationalStudentNumber != "" {
		updates["nationalStudentNumber"] = input.NationalStudentNumber
	}

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

	// 证件号与生日（直接透传）
	if input.IDCard != "" {
		updates["idCard"] = input.IDCard
	}
	if input.BirthdayStr != "" {
		updates["birthday"] = input.BirthdayStr
	}

	// 座号
	if input.Seat > 0 {
		updates["seat"] = input.Seat
	}

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
