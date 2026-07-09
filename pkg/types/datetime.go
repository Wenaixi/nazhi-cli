package types

import (
	"encoding/json"
	"time"
)

// DateOnly 包装 time.Time，JSON 序列化/反序列化支持 "YYYY-MM-DD" 格式。
//
// 动机：服务端 /api/studentCircleNew/getCircleStatistics 返回的日期是
// "2026-01-12" 字符串，而非 Go time.Time 默认的 RFC3339。DateOnly 在
// UnmarshalJSON 时吃 "YYYY-MM-DD"，MarshalJSON 时输出 RFC3339（满足
// v1.0.0"时间字段全部 ISO 8601 + 时区"约定）。
type DateOnly struct {
	time.Time
}

// UnmarshalJSON 解析 "YYYY-MM-DD" 格式 JSON 字符串。
// 空字符串 "" 视为零值，不返回错误（服务端有些字段可能返回空串)。
func (d *DateOnly) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if s == "" {
		return nil
	}
	t, err := time.ParseInLocation(time.DateOnly, s, time.Local)
	if err != nil {
		return err
	}
	d.Time = t
	return nil
}

// MarshalJSON 输出 RFC3339 格式（ISO 8601 + 时区），零值输出 null。
func (d DateOnly) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(d.Format(time.RFC3339))
}
