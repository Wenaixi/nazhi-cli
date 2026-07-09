// Package internal 包含 SDK 内部的辅助转换函数。
package internal

import (
	"strconv"
	"strings"
	"time"
)

// MapTaskStatus 把 server 返回的 circleTaskStatus 字符串映射成 bool。
// 例："上传期 已提交" → true；"上传期 未提交" / "未开始 未提交" → false。
func MapTaskStatus(serverStatus string) bool {
	return strings.Contains(serverStatus, "已提交")
}

// MapSchoolID 把 string school_id 转 int64。
func MapSchoolID(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// ParseServerDate 解析 server 返回的 "YYYY-MM-DD" 字符串为 time.Time（CST 时区）。
func ParseServerDate(s string) (time.Time, error) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	if loc == nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	return time.ParseInLocation("2006-01-02", s, loc)
}

// MapCircleApproved 把 server int status 转 bool（0=待审, 1=已通过）。
func MapCircleApproved(status int) bool {
	return status == 1
}