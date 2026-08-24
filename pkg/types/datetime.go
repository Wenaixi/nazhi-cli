// datetime.go 为历史占位文件，当前无导出类型（说明见下方）。
package types

// datetime.go 历史占位：曾计划提供 DateOnly（type DateOnly time.Time）用于 YYYY-MM-DD / RFC3339 双兼容解码。
// 实际演进中所有日期字段已退化为 string 透传（保留服务端原始格式，如 "2026-01-12"），
// 不再使用 time.Time / DateOnly，避免时区与格式兼容负担。
// 本文件保留仅为消除“空包误导”审计告警；请勿在此新增 DateOnly 类型。
// 如需日期解析，由调用方按需自行解析字符串。
