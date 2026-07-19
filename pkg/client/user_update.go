package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

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
//   - GenderName → gender： "男"→1, "女"→2
//   - YouthLeague → youthLeagueFlag： "是"→1, "否"→0
//   - NationName → nation： "汉族"→1, "满族"→2, "维吾尔族"→3, "畲族"→4, "回族"→5, "壮族"→6, "土家族"→7, "苗族"→8
//   - IdCardType → idType： "中国居民身份证"→1 等 7 种证件类型
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

	// GenderName → gender（API 需要的是 gender 数字，非 genderName 字符串）
	if input.GenderName != "" {
		switch input.GenderName {
		case "男":
			updates["gender"] = 1
		case "女":
			updates["gender"] = 2
		default:
			return fmt.Errorf("%w: 不支持的性别值 %q（支持：男/女）", ErrInvalidPayload, input.GenderName)
		}
	}

	// YouthLeague → youthLeagueFlag
	if input.YouthLeague != "" {
		switch input.YouthLeague {
		case "是":
			updates["youthLeagueFlag"] = 1
		case "否":
			updates["youthLeagueFlag"] = 0
		default:
			return fmt.Errorf("%w: 不支持的团员值 %q（支持：是/否）", ErrInvalidPayload, input.YouthLeague)
		}
	}

	// NationName → nation（前端 modifyBox.vue 中的 8 种民族映射）
	if input.NationName != "" {
		switch input.NationName {
		case "汉族":
			updates["nation"] = 1
		case "满族":
			updates["nation"] = 2
		case "维吾尔族":
			updates["nation"] = 3
		case "畲族":
			updates["nation"] = 4
		case "回族":
			updates["nation"] = 5
		case "壮族":
			updates["nation"] = 6
		case "土家族":
			updates["nation"] = 7
		case "苗族":
			updates["nation"] = 8
		default:
			return fmt.Errorf("%w: 不支持的民族值 %q", ErrInvalidPayload, input.NationName)
		}
	}

	// IdCardType → idType（前端 modifyBox.vue 中的 7 种证件类型映射）
	if input.IdCardType != "" {
		switch input.IdCardType {
		case "中国居民身份证":
			updates["idType"] = 1
		case "外国人永久居留身份证":
			updates["idType"] = 2
		case "港澳居民来往内地通行证":
			updates["idType"] = 3
		case "港澳台居民居住证":
			updates["idType"] = 4
		case "台湾居民来往大陆通行证":
			updates["idType"] = 5
		case "护照":
			updates["idType"] = 6
		case "香港永久性居民身份证":
			updates["idType"] = 7
		default:
			return fmt.Errorf("%w: 不支持的证件类型值 %q", ErrInvalidPayload, input.IdCardType)
		}
	}

	return c.UpdateMyInfo(ctx, token, updates)
}
