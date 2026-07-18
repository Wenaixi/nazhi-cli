package client

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// GetMonthBonus 获取学生月积分。
//
// 对应前端：mallTop.vue / mainLeft.vue → getMonthBonusByStudentId
// API: GET /api/bonusInfo/getMonthBonusByStudentId
func (c *Client) GetMonthBonus(ctx context.Context, token string) ([]types.BonusInfo, error) {
	resp, err := c.doBizAndDecode(ctx, token, "GetMonthBonus", "/api/bonusInfo/getMonthBonusByStudentId", http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetMonthBonus 失败: %w", err)
	}

	bonusList, err := types.DecodeDataList[types.BonusInfo](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetMonthBonus 解析积分列表失败: %w", err)
	}

	return bonusList, nil
}

// GetHistoryBonus 获取历史积分。
//
// 对应前端：mallTop.vue → getHistoryBonusByStudentId
// API: GET /api/bonusInfo/getHistoryBonusByStudentId?termId={termId}&month={month}
func (c *Client) GetHistoryBonus(ctx context.Context, token string, termID int64, month string) ([]types.BonusInfo, error) {
	path := "/api/bonusInfo/getHistoryBonusByStudentId?termId=" + strconv.FormatInt(termID, 10) + "&month=" + month
	resp, err := c.doBizAndDecode(ctx, token, "GetHistoryBonus", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetHistoryBonus 失败: %w", err)
	}

	bonusList, err := types.DecodeDataList[types.BonusInfo](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetHistoryBonus 解析积分列表失败: %w", err)
	}

	return bonusList, nil
}

// GetBonusRank 查询班级积分排行。
//
// 对应前端：mainLeft.vue / yhmain/mainLeft.vue → getMonthBonusRankByClassId
// API: GET /api/bonusInfo/getMonthBonusRankByClassId?limitNum={limitNum}
func (c *Client) GetBonusRank(ctx context.Context, token string, limitNum int) ([]types.BonusRank, error) {
	path := "/api/bonusInfo/getMonthBonusRankByClassId?limitNum=" + strconv.Itoa(limitNum)
	resp, err := c.doBizAndDecode(ctx, token, "GetBonusRank", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetBonusRank 失败: %w", err)
	}

	ranks, err := types.DecodeDataList[types.BonusRank](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetBonusRank 解析排名列表失败: %w", err)
	}

	return ranks, nil
}

// GetBonusDetail 获取积分明细。
//
// 对应前端：mainMidSearch.vue / yhmain/mainMidSearch.vue → getMonthBonusDetailByStudentId
// API: GET /api/bonusInfo/getMonthBonusDetailByStudentId?limitNum={limitNum}
func (c *Client) GetBonusDetail(ctx context.Context, token string, limitNum int) ([]types.BonusDetail, error) {
	path := "/api/bonusInfo/getMonthBonusDetailByStudentId?limitNum=" + strconv.Itoa(limitNum)
	resp, err := c.doBizAndDecode(ctx, token, "GetBonusDetail", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetBonusDetail 失败: %w", err)
	}

	details, err := types.DecodeDataList[types.BonusDetail](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetBonusDetail 解析积分明细失败: %w", err)
	}

	return details, nil
}
