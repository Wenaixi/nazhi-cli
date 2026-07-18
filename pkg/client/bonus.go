package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// GetMonthBonus 获取当月积分。
// GET /api/bonusInfo/getMonthBonusByStudentId
func (c *Client) GetMonthBonus(ctx context.Context, token string) (json.RawMessage, error) {
	resp, err := c.doBizAndDecode(ctx, token, "GetMonthBonus",
		"/api/bonusInfo/getMonthBonusByStudentId", http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetMonthBonus 失败: %w", err)
	}
	raw := rawObjectBytes(*resp)
	if len(raw) == 0 {
		return []byte("{}"), nil
	}
	return raw, nil
}

// GetHistoryBonus 获取历史积分。
// GET /api/bonusInfo/getHistoryBonusByStudentId?termId=&month=
func (c *Client) GetHistoryBonus(ctx context.Context, token string, termID int64, month string) (json.RawMessage, error) {
	path := "/api/bonusInfo/getHistoryBonusByStudentId?termId=" + strconv.FormatInt(termID, 10) + "&month=" + month
	resp, err := c.doBizAndDecode(ctx, token, "GetHistoryBonus", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetHistoryBonus 失败: %w", err)
	}
	raw := rawObjectBytes(*resp)
	if len(raw) == 0 {
		return []byte("{}"), nil
	}
	return raw, nil
}

// GetBonusRank 获取班级积分排行。
// GET /api/bonusInfo/getMonthBonusRankByClassId?limitNum=
func (c *Client) GetBonusRank(ctx context.Context, token string, limit int) (json.RawMessage, error) {
	path := "/api/bonusInfo/getMonthBonusRankByClassId?limitNum=" + strconv.Itoa(limit)
	resp, err := c.doBizAndDecode(ctx, token, "GetBonusRank", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetBonusRank 失败: %w", err)
	}
	raw := rawListBytes(*resp)
	if len(raw) == 0 {
		return []byte("[]"), nil
	}
	return raw, nil
}

// GetBonusDetail 获取积分明细。
// GET /api/bonusInfo/getMonthBonusDetailByStudentId?limitNum=
func (c *Client) GetBonusDetail(ctx context.Context, token string, limit int) (json.RawMessage, error) {
	path := "/api/bonusInfo/getMonthBonusDetailByStudentId?limitNum=" + strconv.Itoa(limit)
	resp, err := c.doBizAndDecode(ctx, token, "GetBonusDetail", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetBonusDetail 失败: %w", err)
	}
	raw := rawListBytes(*resp)
	if len(raw) == 0 {
		return []byte("[]"), nil
	}
	return raw, nil
}
