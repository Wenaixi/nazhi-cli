# SDK API 字段参考 (v1.0.0)

本文档列出所有 SDK 公开响应的字段。完整字段定义见 `pkg/types/` 目录源码。

## 目录

- [UnifiedResponse](#unifiedresponse) — 平台通用响应包
- [LoginResponse](#loginresponse)
- [UserInfo](#userinfo)
- [Task](#task)
- [TaskSubmitPayload](#tasksubmitpayload)
- [TaskResult](#taskresult)
- [CircleRecord](#circlerecord)
- [CircleImage](#circleimage)
- [PageBean](#pagebean)
- [HonorType](#honortype)
- [HonorRecord](#honorrecord)
- [AddHonorPayload](#addhonorpayload)
- [HonorSelectOption](#honorselectoption)
- [SelfEvalStatus](#selfevalstatus)
- [Dimension](#dimension)
- [BusinessError](#businesserror)

---

## UnifiedResponse

平台所有业务接口的通用响应包。完整定义 `pkg/types/response.go`。

| 字段 | Go 类型 | JSON tag | 必选 | 说明 |
|------|---------|----------|------|------|
| Code | `int` | `code` | 是 | 业务 code: 1=成功, 其他=错误 |
| Msg | `*string` | `msg` | 否 | 业务消息; nil 或空字符串时回落到 "未知错误" |
| ReturnData | `*json.RawMessage` | `returnData` | 否 | 主业务数据 (解码延迟) |
| DataList | `*json.RawMessage` | `dataList` | 否 | 列表数据 |
| DataMap | `*json.RawMessage` | `dataMap` | 否 | 字典数据 |
| PageBean | `*json.RawMessage` | `pageBean` | 否 | 分页信息 |

JSON 示例:

```json
{
  "code": 1,
  "msg": "成功",
  "returnData": { ... },
  "dataList": [...],
  "dataMap": { ... },
  "pageBean": {
    "pageNo": 1,
    "pageSize": 20,
    "totalNum": 42,
    "totalPage": 3
  }
}
```

辅助方法: `DecodeResponse` / `CheckCode` / `DecodeReturnData[T]` / `DecodeDataList[T]` / `DecodeDataMap[T]` / `DecodePageBean`。

---

## LoginResponse

SSO 登录成功响应。完整定义 `pkg/types/login.go`。

| 字段 | Go 类型 | JSON tag | 必选 | 说明 |
|------|---------|----------|------|------|
| Token | `string` | `token` | 是 | X-Auth-Token 凭证 (后续业务接口必带) |
| ExpiresAt | `time.Time` | `expiresAt` | 是 | token 过期时间 (ISO 8601 + 时区) |
| FallbackUsed | `bool` | `fallbackUsed` | 是 | 本次登录是否降级到 ddddocr OCR (primary 失败后) |

JSON 示例:

```json
{
  "token": "eyJhbGciOiJIUzUxMiJ9...",
  "expiresAt": "2026-07-23T18:38:00+08:00",
  "fallbackUsed": false
}
```

---

## UserInfo

用户个人资料精简核心视图。完整定义 `pkg/types/user.go`。

> v1.0.0 从 51 字段精简到 10 字段, 仅保留业务核心身份/学校/班级字段。
> 联系方式、证件号、积分等敏感或运营字段已移除。

| 字段 | Go 类型 | JSON tag | 必选 | 说明 |
|------|---------|----------|------|------|
| ID | `int64` | `id` | 是 | 用户 ID |
| Name | `string` | `name` | 是 | 姓名 |
| StudentNumber | `string` | `studentNumber` | 是 | 学号 |
| StudentID | `int64` | `studentId` | 是 | 学生 ID (与 id 不同, 服务端逻辑区分) |
| SchoolID | `int64` | `schoolId` | 是 | 学校 ID |
| SchoolName | `string` | `schoolName,omitempty` | 否 | 学校名 (平台返回 null 时省略) |
| GradeID | `int64` | `gradeId` | 是 | 年级 ID |
| GradeName | `string` | `gradeName` | 是 | 年级名 (如"高一") |
| ClassID | `int64` | `classId` | 是 | 班级 ID |
| ClassName | `string` | `className` | 是 | 班级名 (如"八班") |

JSON 示例:

```json
{
  "id": 12345,
  "name": "张三",
  "studentNumber": "G123456789012345678",
  "studentId": 67890,
  "schoolId": 11000001,
  "schoolName": "纳智高中",
  "gradeId": 12,
  "gradeName": "高一",
  "classId": 88,
  "className": "八班"
}
```

---

## Task

任务条目。完整定义 `pkg/types/task.go`。

> v1.0.0 从 18 字段精简到 11 字段。状态由字符串收敛为 bool, 时间字段升级为 time.Time。

| 字段 | Go 类型 | JSON tag | 必选 | 说明 |
|------|---------|----------|------|------|
| ID | `int64` | `id` | 是 | 任务 ID (即 circleTaskId) |
| Name | `string` | `name` | 是 | 任务名称 |
| TypeName | `string` | `typeName` | 是 | 类型名称 |
| DimensionName | `string` | `dimensionName` | 是 | 维度名称 (思想品德/学业水平 等) |
| Hours | `float64` | `hours` | 是 | 学时 |
| Submitted | `bool` | `submitted` | 是 | 是否已提交 (来自服务端 circleTaskStatus) |
| NeedPic | `bool` | `needPic` | 是 | 是否需要图片 (来自服务端 upPic 0/1) |
| StartDate | `DateOnly` | `startDateStr` | 是 | 开始日期 (如 2026-01-12) |
| EndDate | `DateOnly` | `endDateStr` | 是 | 结束日期 (如 2026-02-10) |
| ScopeType | `int` | `scopeType` | 是 | 作用域类型 (参见 ScopeClass/Grade/Stage 常量) |
| ScopeTypeName | `string` | `scopeTypeName` | 是 | 作用域名称 (班级/年级/学段) |

**ScopeType 常量**:

| 常量 | 值 | 说明 |
|------|----|------|
| `ScopeClass` | `1` | 班级任务 |
| `ScopeGrade` | `2` | 年段任务 |
| `ScopeStage` | `3` | 学段任务 |

JSON 示例:

```json
{
  "id": 4567,
  "name": "寒假社会实践",
  "typeName": "志愿服务",
  "dimensionName": "社会实践",
  "hours": 2.5,
  "submitted": false,
  "needPic": true,
  "startDateStr": "2026-01-12",
  "endDateStr": "2026-02-10",
  "scopeType": 2,
  "scopeTypeName": "年级"
}
```

---

## TaskSubmitPayload

addCircle 接口的完整请求体 (29 字段透传)。完整定义 `pkg/types/task.go`。

| 字段 | Go 类型 | JSON tag | 必选 | 说明 |
|------|---------|----------|------|------|
| ID | `*int64` | `id` | 否 | 记录 ID (新建时为空, 更新时填) |
| Name | `string` | `name` | 是 | 写实标题 |
| HostName | `string` | `hostName` | 否 | 主办方 |
| CircleDate | `string` | `circleDate` | 否 | 写实日期 (YYYY-MM-DD) |
| Rank | `string` | `rank` | 否 | 排名 |
| Level | `string` | `level` | 否 | 级别 |
| Content | `string` | `content` | 是 | 写实正文 |
| PictureList | `[]int64` | `pictureList` | 否 | 图片附件 ID 列表 |
| CircleTaskID | `int64` | `circleTaskId` | 是 | 关联任务 ID |
| CircleTypeID | `int64` | `circleTypeId` | 是 | 写实类型 ID |
| DimensionID | `int64` | `dimensionId` | 是 | 维度 ID |
| Hours | `float64` | `hours` | 否 | 实践时长 |
| CircleBeginDate | `string` | `circleBeginDate` | 否 | 开始日期 (YYYY-MM-DD) |
| CircleEndDate | `string` | `circleEndDate` | 否 | 结束日期 (YYYY-MM-DD) |
| CheckResult | `string` | `checkResult` | 否 | 检查结果 |
| PatentType | `string` | `patentType` | 否 | 专利类型 |
| PatentNum | `string` | `patentNum` | 否 | 专利号 |
| Address | `string` | `address` | 否 | 地址 |
| TermName | `string` | `termName` | 否 | 学期名 |
| ActivityName | `string` | `activityName` | 否 | 活动名 |
| SportsName | `string` | `sportsName` | 否 | 体育项目 |
| TeamName | `string` | `teamName` | 否 | 团队名 |
| OrgName | `string` | `orgName` | 否 | 组织名 |
| ResultsName | `string` | `resultsName` | 否 | 成果名 |
| ObtainTime | `string` | `obtainTime` | 否 | 获得时间 |
| SpecialtyTechnology | `string` | `specialtyTechnology` | 否 | 特长技术 |
| PlayRole | `string` | `playRole` | 否 | 扮演角色 |
| LikeSpecialty1/2/3 | `string` | `likeSpecialtyN` | 否 | 兴趣特长 1/2/3 |

JSON 示例:

```json
{
  "name": "图书馆志愿服务",
  "content": "协助整理图书 200 册...",
  "circleTaskId": 4567,
  "circleTypeId": 3,
  "dimensionId": 9,
  "hours": 4.0,
  "circleBeginDate": "2026-01-15",
  "circleEndDate": "2026-01-15",
  "pictureList": [1001, 1002]
}
```

---

## TaskResult

提交任务结果。完整定义 `pkg/types/task.go`。

| 字段 | Go 类型 | JSON tag | 必选 | 说明 |
|------|---------|----------|------|------|
| Code | `int` | `code` | 是 | 业务 code (1=成功) |
| Msg | `string` | `msg` | 是 | 结果描述 |

---

## CircleRecord

已提交的写实记录 (来自 getStudentCircle 接口)。完整定义 `pkg/types/circle.go`。

> v1.0.0 从 15 字段精简到 9 字段。状态由 int 收敛为 bool, 日期升级为 time.Time。

| 字段 | Go 类型 | JSON tag | 必选 | 说明 |
|------|---------|----------|------|------|
| ID | `int64` | `id` | 是 | 写实记录主键 |
| Name | `string` | `name` | 是 | 写实标题 |
| Content | `string` | `content` | 是 | 写实正文 |
| TypeName | `string` | `typeName` | 是 | 类型名 (替代原 type_name) |
| Approved | `bool` | `approved` | 是 | 是否已通过审核 (true=已通过, 替代原 int status) |
| CircleDate | `time.Time` | `circleDate` | 是 | 写实发生日期 (ISO 8601 + 时区) |
| Hours | `float64` | `hours` | 是 | 实践时长 (小时) |
| ImgList | `[]CircleImage` | `imgList` | 是 | 关联图片附件列表 |
| Remark | `string` | `remark` | 否 | 备注 |

JSON 示例:

```json
{
  "id": 7890,
  "name": "图书馆志愿服务",
  "content": "协助整理图书 200 册...",
  "typeName": "志愿服务",
  "approved": true,
  "circleDate": "2026-01-15T00:00:00+08:00",
  "hours": 4.0,
  "imgList": [
    { "attachmentId": 1001 },
    { "attachmentId": 1002 }
  ],
  "remark": ""
}
```

---

## CircleImage

写实记录关联的图片附件。完整定义 `pkg/types/circle.go`。

> v1.0.0 从 5 字段精简到 1 字段, 仅保留 AttachmentID。

| 字段 | Go 类型 | JSON tag | 必选 | 说明 |
|------|---------|----------|------|------|
| AttachmentID | `int64` | `attachmentId` | 是 | 附件 ID (用于查询/下载图片) |

---

## PageBean

平台通用分页信息。完整定义 `pkg/types/circle.go`。

| 字段 | Go 类型 | JSON tag | 必选 | 说明 |
|------|---------|----------|------|------|
| PageNo | `int` | `pageNo` | 是 | 当前页码 (1-based) |
| PageSize | `int` | `pageSize` | 是 | 每页条数 |
| TotalNum | `int` | `totalNum` | 是 | 总条数 |
| TotalPage | `int` | `totalPage` | 是 | 总页数 |

---

## HonorType

可申报的荣誉类型 (来自 getHonorType 接口)。完整定义 `pkg/types/honor.go`。

> v1.0.0 从 8 字段精简到 5 字段, Score/DimensionID/SortNo 等运营字段已移除。

| 字段 | Go 类型 | JSON tag | 必选 | 说明 |
|------|---------|----------|------|------|
| ID | `int64` | `id` | 是 | 荣誉类型 ID |
| Name | `string` | `name` | 是 | 荣誉名称 |
| LevelName | `string` | `levelName` | 是 | 级别名 (校/区县/市/省/国家) |
| Level | `int` | `level` | 是 | 级别代码 (5=校, 4=区县, 3=市, 2=省, 1=国家) |
| DimensionName | `string` | `dimensionName` | 是 | 维度名 |

JSON 示例:

```json
{
  "id": 23,
  "name": "三好学生",
  "levelName": "校级",
  "level": 5,
  "dimensionName": "思想品德"
}
```

---

## HonorRecord

已申报的荣誉记录 (来自 getHonorByStudentId 接口)。完整定义 `pkg/types/honor.go`。

> v1.0.0 从 17 字段精简到 9 字段。Score/TypeID/DimensionID 等冗余 ID 已移除;
> StudentName/ClassName 等用户维度信息已移除 (避免与 UserInfo 重复);
> 原 int status 收敛为 bool approved。

| 字段 | Go 类型 | JSON tag | 必选 | 说明 |
|------|---------|----------|------|------|
| ID | `int64` | `id` | 是 | 荣誉记录 ID |
| TypeName | `string` | `typeName` | 是 | 荣誉类型名 |
| LevelName | `string` | `levelName` | 是 | 级别名 |
| Level | `int` | `level` | 是 | 级别代码 |
| DimensionName | `string` | `dimensionName` | 是 | 维度名 |
| Approved | `bool` | `approved` | 是 | 是否已通过审核 (true=已通过, 替代原 int status) |
| ApprovedName | `string` | `approvedName` | 是 | 审核状态名 (替代原 statusName) |
| GetDate | `time.Time` | `getDate` | 是 | 获得日期 (ISO 8601 + 时区) |
| EvaluationAgency | `string` | `evaluationAgency` | 是 | 颁发机构 |

JSON 示例:

```json
{
  "id": 567,
  "typeName": "三好学生",
  "levelName": "校级",
  "level": 5,
  "dimensionName": "思想品德",
  "approved": true,
  "approvedName": "已通过",
  "getDate": "2025-09-01T00:00:00+08:00",
  "evaluationAgency": "纳智高中"
}
```

---

## AddHonorPayload

addHonor 接口的请求体。完整定义 `pkg/types/honor.go`。

| 字段 | Go 类型 | JSON tag | 必选 | 说明 |
|------|---------|----------|------|------|
| Name | `string` | `name` | 是 | 荣誉名称 |
| TypeID | `int64` | `typeId` | 是 | 荣誉类型 ID |
| TypeName | `string` | `typeName` | 是 | 荣誉类型名 |
| Level | `int` | `level` | 是 | 级别代码 |
| EvaluationAgency | `string` | `evaluationAgency` | 是 | 颁发机构 |
| GetDate | `string` | `getDate` | 是 | 获得日期 (YYYY-MM-DD) |
| CertImgAttachmentID | `string` | `certImgAttachmentId` | 否 | 证书图片附件 ID 或空 |

JSON 示例:

```json
{
  "name": "三好学生",
  "typeId": 23,
  "typeName": "三好学生",
  "level": 5,
  "evaluationAgency": "纳智高中",
  "getDate": "2025-09-01",
  "certImgAttachmentId": "1001"
}
```

---

## HonorSelectOption

下拉选择选项 (label/value 对)。完整定义 `pkg/types/honor.go`。

| 字段 | Go 类型 | JSON tag | 必选 | 说明 |
|------|---------|----------|------|------|
| Label | `string` | `label` | 是 | 显示文本 |
| Value | `int` | `value` | 是 | 选项值 |

---

## SelfEvalStatus

自我评价状态。完整定义 `pkg/types/self_eval.go`。

> v1.0.0 从 10 字段精简到 3 字段, 仅保留 id + 双向评语。

| 字段 | Go 类型 | JSON tag | 必选 | 说明 |
|------|---------|----------|------|------|
| ID | `int64` | `id` | 是 | 自我评价 ID |
| StudentComment | `string` | `studentComment` | 是 | 学生自评 |
| TeacherComment | `string` | `teacherComment` | 是 | 教师评语 |

JSON 示例:

```json
{
  "id": 12,
  "studentComment": "本学期我积极参加志愿服务...",
  "teacherComment": "该生表现优异..."
}
```

---

## Dimension

维度信息 (思想品德/学业水平/身心健康/艺术素养/社会实践/劳动素养)。完整定义 `pkg/types/dimension.go`。

| 字段 | Go 类型 | JSON tag | 必选 | 说明 |
|------|---------|----------|------|------|
| ID | `int64` | `id` | 是 | 维度 ID (0=全部汇总, FetchTasks 会跳过) |
| Name | `string` | `name` | 是 | 维度名 |

JSON 示例:

```json
{
  "id": 9,
  "name": "社会实践"
}
```

---

## BusinessError

业务错误, 保留数值 code 供 errors.As 精细判别。完整定义 `pkg/types/response.go`。

| 字段 | Go 类型 | 必选 | 说明 |
|------|---------|------|------|
| Code | `int` | 是 | 业务 code (非 1) |
| Msg | `string` | 是 | 错误描述 |

调用方:

```go
var bizErr *types.BusinessError
if errors.As(err, &bizErr) {
    switch bizErr.Code {
    case 2:
        // 重试
    case 500:
        // 致命
    }
}
```
