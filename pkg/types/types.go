// Package types 定义 nazhi-cli SDK 的全部公共类型。
//
// 设计原则：
//   - 全部响应字段统一 camelCase JSON tag（以平台真实键名为准，写实列表存在混用）
//   - 状态字段重命名为业务相关名（submitted / approved）以避免歧义
//   - 布尔字段优先 bool，平台混用 0/1 时用 FlexBool 兼容
//   - 时间/日期字段为 string 透传，保留服务端原始格式（如 "2026-01-12"）；
//     仅 LoginResponse.ExpiresAt 为 time.Time（由 JWT exp 派生）
//   - 范围类型导出常量（ScopeClass / ScopeGrade / ScopeStage）
//
// 域拆分：
//   - login.go      认证（LoginRequest, LoginResponse）
//   - user.go       用户（UserInfo, UserUpdateInput）
//   - task.go       任务（Task, TaskSubmitPayload, TaskResult, ScopeType 常量）
//   - circle.go     写实记录（CircleRecord, CircleImage, PageBean, PlayRoleCode）
//   - honor.go      荣誉（HonorType, HonorRecord, AddHonorPayload, HonorSelectOption）
//   - self_eval.go  自我评价（SelfEvalStatus，解码兼容 snake/camel）
//   - flexjson.go   JSON 宽松类型（PlayRoleCode 等）
//   - dimension.go  维度（Dimension）
package types

// 注：本文件仅保留 package 级别的设计文档。具体类型定义见各域文件。
