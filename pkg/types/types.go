// Package types 定义 nazhi-cli SDK 的全部公共类型。
//
// 设计原则（v1.0.0）：
//   - 全部响应字段统一 camelCase JSON tag
//   - 状态字段重命名为业务相关名（submitted / approved）以避免歧义
//   - 布尔字段全部 bool 类型（needPic / submitted / approved）
//   - 时间字段全部 time.Time（自动序列化为 ISO 8601 + 时区）
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
