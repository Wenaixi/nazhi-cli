# 前端 API 对照 TodoList

> 对比 `E:\newCC\life-new2026\nazhi\src\components\` 所有 `.vue` 中的 API 调用
> 与 `E:\newCC\life-new2026\nazhi-cli\pkg\client\` SDK 实现的差异

---

## P0 — type 映射修复（已确认为 bug）

`getStudentCircle` 的 type 参数映射与前端完全错位：

| type | 服务器实际行为 | 前端管理页标签 | SDK 当前标称 | 需要改为 | 影响命令 |
|------|---------------|---------------|-------------|----------|---------|
| 1 | 全班所有记录 | **公示查看** | "已发布"→`GetSubmittedCircles` | "公示/全部"→`GetPublicCircles` | `task public`, `task submitted` |
| 2 | 教师写实 | 教师写实 | 教师写实 `GetTeacherCircles` ✅ | 不变 | ✅ |
| 3 | 仅当前用户自己的记录 | **我发布的写实** | "被撤回"→`GetWithdrawnCircles` ❌ | "我发布的"→`GetSubmittedCircles` | `task submitted`, `task withdrawn` |
| 4 | 返回 0（被撤回） | **被撤回的写实** | "公示"→`GetPublicCircles` ❌ | "被撤回"→`GetWithdrawnCircles` | `task withdrawn`, `task public` |

**方案 A（推荐，破坏性变更，需要改 CLI 命令名/别名来匹配语义）**：
- `task submitted` → type=3（我发布的写实 — 这才是 "submitted" 的语义）
- `task withdrawn` → type=4（被撤回）
- `task public` → type=1（公示/全部）

**方案 B（保守，不破坏 CLI 接口，但 CLI 输出与命令名不符）**：
- 保持现有 CLI 命令名不变，SDK 层直接修正 type 映射
- 但这意味着 `task submitted` 输出的数据变少（从 2533 降到 48），`task public` 从 0 变多

---

## P1 — 缺失的 SDK 方法（前端有调用，SDK 未实现）

### 1. 写实管理 Circle

| 前端 API | 方法 | 用途 | SDK 状态 | 优先级 |
|----------|------|------|----------|--------|
| `/api/studentCircleNew/deleteCircle?id=` | GET | 删除写实 | ❌ 缺失 | P1 |
| `/api/studentCircleNew/addCircleComment` | POST | 添加评论 | ❌ 缺失 | P1 |
| `/api/studentCircleNew/setCircleLikeById` | GET | 点赞/取消 | ❌ 缺失 | P1 |
| `/api/studentCircleNew/getCircleImg` | GET | 获取图片 | ❌ 缺失 | P1 |
| `/api/studentCircleNew/getCircleType` | GET | 获取类别 | ❌ 缺失 | P2 |
| `/api/studentCircleNew/getCircleTask` | GET | 获取任务 | ❌ 缺失 | P2 |
| `/api/studentCircleNew/getCircleTypeByTaskId` | GET | 任务元数据 | ✅ `GetCircleTypeByTaskID` | ✅ |
| `/api/studentCircleNew/getCircleTypeByDimensionId` | GET | 维度下类别 | ❌ 缺失 | P2 |
| `/api/studentCircleNew/getCircleStatisticsByTypeId` | GET | 类别下任务统计 | ❌ 缺失（有 getCircleStatistics） | P2 |
| `/api/studentCircleNew/getRecentlyCircleTask` | GET | 首页近期任务 | ❌ 缺失 | P2 |
| `/api/studentCircleNew/getDimensions` | GET | 维度列表 | ✅ `GetDimensions` | ✅ |
| `/api/studentCircleNew/addCircle` | POST | 新增写实 | ✅ `SubmitTask` | ✅ |
| `/api/studentCircleNew/editCircle` | POST | 编辑写实 | ✅ `EditCircle` | ✅ |
| `/api/studentCircleNew/getCircleImgByDimensionId` | GET | 维度下图片 | ❌ 缺失 | P2 |

### 2. 典型案例 TypicalCase

| 前端 API | 方法 | 用途 | SDK 状态 |
|----------|------|------|----------|
| `/api/studentCircleNew/getTypicalCase` | GET | 查案例列表 | ✅ `GetTypicalCaseList` |
| `/api/studentCircleNew/addTypicalCase` | POST | 新增案例 | ✅ `AddTypicalCase` |
| `/api/studentCircleNew/updateTypicalCase` | POST | 更新案例 | ❌ 缺失 |
| `/api/studentCircleNew/deleteTypicalCase?id=` | GET | 删除案例 | ❌ 缺失 |
| `/api/studentCircleNew/deleteBatchTypicalCase` | POST | 批量删除 | ❌ 缺失 |

### 3. 民主评价 Democratic（完全缺失）

| 前端 API | 方法 | 用途 |
|----------|------|------|
| `/api/studentDemocraticNew/getActivity` | GET | 活动列表 |
| `/api/studentDemocraticNew/getDemocraticActivityById` | GET | 单个活动 |
| `/api/studentDemocraticNew/getSelfEvaluation` | GET | 自评数据 |
| `/api/studentDemocraticNew/getMutualPersonInfo` | GET | 互评人员 |
| `/api/studentDemocraticNew/getDemocraticResult` | GET | 评价结果 |
| `/api/studentDemocraticNew/getMutualEvaluationDetail` | POST | 互评详情 |
| `/api/studentDemocraticNew/addOrUpdateSelfEvaluation` | POST | 提交自评 |
| `/api/studentDemocraticNew/addOrUpdateMutualEvaluation` | POST | 提交互评 |

### 4. 德育表现 MoralEdu

| 前端 API | 方法 | 用途 | SDK 状态 |
|----------|------|------|----------|
| `/api/studentMoralEduNew/getViolation` | GET | 违规记录 | ❌ 缺失 |
| `/api/studentMoralEduNew/getViolationType` | GET | 违规事由 | ❌ 缺失 |
| `/api/studentMoralEduNew/getHonorType` | GET | 荣誉类型 | ✅ |
| `/api/studentMoralEduNew/getHonorTypeForSelect` | GET | 荣誉下拉 | ✅ |
| `/api/studentMoralEduNew/getHonorLevel` | GET | 荣誉级别 | ✅ |
| `/api/studentMoralEduNew/getHonorByStudentId` | GET | 荣誉列表 | ✅ `GetHonorList` |
| `/api/studentMoralEduNew/addHonor` | POST | 新增荣誉 | ✅ `AddHonor` |
| `/api/studentMoralEduNew/updateHonor` | POST | 更新荣誉 | ❌ 缺失 |
| `/api/studentMoralEduNew/deleteHonorById` | GET | 删除荣誉 | ✅ `DeleteHonor` |
| `/api/studentMoralEduNew/querySelfEvaluation` | GET | 查自评 | ✅ `QuerySelfEvaluation` |
| `/api/studentMoralEduNew/addSelfEvaluation` | POST | 提交自评 | ✅ `SubmitSelfEvaluation` |
| `/api/studentMoralEduNew/querySelfGradEvaluation` | GET | 毕业状态 | ✅ `QuerySelfGradEvaluation` |
| `/api/studentMoralEduNew/addSelfGradEvaluation` | POST | 毕业自评 | ❌ 缺失 |

### 5. 学生信息 StudentInfo

| 前端 API | 方法 | 用途 | SDK 状态 |
|----------|------|------|----------|
| `/api/studentInfo/getMenu` | GET | 菜单权限 | ❌ 缺失 |
| `/api/studentInfo/getTermFace` | GET | 学期封面 | ❌ 缺失 |
| `/api/studentInfo/getMyInfo` | GET | 用户信息 | ✅ `GetMyInfo` |
| `/api/studentInfo/updateMyInfo` | POST | 更新资料 | ❌ 缺失 |
| `/api/studentInfo/showShopInfo` | GET | 商城信息（拼接 baseUrl） | ❌ 缺失 |

### 6. 通知公告 Announcement（完全缺失）

| 前端 API | 方法 | 用途 |
|----------|------|------|
| `/api/uiAnnouncement/queryUnreadNotificationByTeacher` | GET | 未读通知数 |
| `/api/uiAnnouncement/getAnnouncementById` | GET | 通知详情 |
| `/api/uiAnnouncement/readUnreadNotificationByTeacher` | GET | 标为已读 |
| `/api/uiAnnouncement/queryAllNotificationByTeacher` | GET | 通知列表 |

### 7. 积分 Bonus（完全缺失）

| 前端 API | 方法 | 用途 |
|----------|------|------|
| `/api/bonusInfo/getMonthBonusByStudentId` | GET | 月积分 |
| `/api/bonusInfo/getHistoryBonusByStudentId` | GET | 历史积分 |
| `/api/bonusInfo/getMonthBonusRankByClassId` | GET | 班级排行 |
| `/api/bonusInfo/getMonthBonusDetailByStudentId` | GET | 积分明细 |

### 8. 成绩管理 Exam（完全缺失）

| 前端 API | 方法 | 用途 |
|----------|------|------|
| `/api/studentExamNew/getInitInfo` | GET | 初始化学期/考试/课程 |
| `/api/studentExamNew/queryStudentExam` | POST | 查询成绩 |

### 9. 档案/报告（需拼接 baseUrl）

| 前端 API | 方法 | 用途 |
|----------|------|------|
| `{baseUrl}/teacher/school/use/pageQueryTermBySchoolId` | GET | 学期列表 |
| `{baseUrl}/teacher/school/studentReport/getStudentInfoForTermId` | GET | 学生档案 |

### 10. 通用工具

| 前端 API | 方法 | 用途 |
|----------|------|------|
| `/api/common/sys/dict/list?cateCode=23` | GET | 等级字典 |
| `/api/teacher/circle/circleType/queryDimensionBySchoolIdAndStateType` | GET | 学校维度配置 |

### 11. 前端 updateMyInfo

| 前端 API | 方法 | 用途 |
|----------|------|------|
| `/api/studentInfo/updateMyInfo` | POST | 更新个人信息 |

---

## P2 — 响应字段补全

### CircleRecord 缺失字段

从实际 API 响应与 SDK `CircleRecord` 对照，以下字段在前端被使用但 SDK 未定义：

| 字段名（JSON） | 前端使用文件 | SDK 状态 |
|----------------|-------------|----------|
| `type` | managementRightBottom.vue (item.type) | ❌ |
| `status` | managementRightBottom.vue (`item.status==2`) | ❌ |
| `creator` | 多处模板渲染 | ❌ |
| `studentId` | 提交记录 | ❌ |
| `student_num` | 前端模板 | ❌ |
| `class_name` | 前端模板 | ❌ |
| `grade_name` | 前端模板 | ❌ |
| `creator_name` | 前端模板 | ❌ |
| `creation_time` (毫秒时间戳) | 前端未直接使用 | ❌ |
| `scope_type` | 前端模板 | ❌ |
| `scope_type_name` | 前端模板 | ❌ |
| `state_type` | - | ❌ |
| `circle_type_id` | 编辑用 | ❌ |
| `circle_task_id` | 编辑用 | ❌ |
| `role_id` | - | ❌ |
| `role_name` | - | ❌ |
| `push_status` | - | ❌ |
| `push_num` | - | ❌ |
| `operator_id` | - | ❌ |
| `class_id` | - | ❌ |
| `school_id` | - | ❌ |
| `grade_id` | - | ❌ |
| `start_date` | - | ❌ |
| `end_date` | - | ❌ |
| `audit_start_date` | - | ❌ |
| `audit_end_date` | - | ❌ |
| `area_id` | - | ❌ |
| `area_task_id` | - | ❌ |
| `dimension_id` | - | ❌ |
| `likeList` | - | ❌ |
| `show_type` | - | ❌ |
| `score_num` | - | ❌ |
| `up_pic` | - | ❌ |

### UserInfo 对照

前端 `userBox.vue` 和 `modifyBox.vue` 使用的字段：

| 字段 | 前端使用 | SDK 状态 |
|------|---------|----------|
| `name` | userBox, modifyBox | ✅ |
| `className` | userBox, modifyBox (`replace("级","")`) | ✅ （postProcess 清理年级前缀） |
| `studentNumber` | userBox, modifyBox | ✅ |
| `telephone` | userBox | ✅ |
| `genderName` | userBox, modifyBox | ✅ |
| `birthdayStr` | userBox, modifyBox (`birthday`) | ✅ |
| `youthLeagueFlag` | userBox, modifyBox | ✅ |
| `nationalStudentNumber` | userBox, modifyBox | ✅ |
| `familyAddress` | userBox, modifyBox | ✅ |
| `hobbies` | userBox, modifyBox | ✅ |
| `idCard` | modifyBox | ✅ |
| `idType` | modifyBox (`data.idType == 1`) | ✅ (已改为 int) |
| `seat` | modifyBox | ✅ |
| `nation` | modifyBox | ✅ (已改为 int) |

---

## P3 — 类型定义文件（已有但需核查）

| 文件 | 状态 | 说明 |
|------|------|------|
| `pkg/types/circle.go` | ✅ | CircleRecord 已定义，但缺 `type`/`status`/`creator` 等字段 |
| `pkg/types/user.go` | ✅ | UserInfo 字段较完整 |
| `pkg/types/task.go` | ✅ | Task 定义完整 |
| `pkg/types/exam.go` | ✅ | ExamResult / TermInfo / ExamInitInfo 已定义 |
| `pkg/types/violation.go` | ✅ | ViolationRecord / ViolationType 已定义 |
| `pkg/types/democratic.go` | ✅ | DemocraticActivity / SelfEvaluationItem / MutualEvaluation 等已定义 |
| `pkg/types/notification.go` | ✅ | 已有（但需确认字段完整性） |
| `pkg/types/bonus.go` | ✅ | 已有（但需确认字段完整性） |

---

## 优先级说明

- **P0**: 必须立即修（数据返回错误）
- **P1**: 高频操作，用户大概率需要（删除/评论/点赞/违规查询等）
- **P2**: 中频操作，可分批实现
- **P3**: 已有代码但需补充验证

---

## 如何推进

建议按这个顺序：

1. **P0 — type 映射修正**：你决定方案 A 还是 B
2. **P0 — 写实操作基础**：DeleteCircle / AddCircleComment / SetCircleLike（高频日常操作）
3. **P1 — 违规查询**：GetViolationList / GetViolationTypes
4. **P1 — 成绩查询**：GetExamInitInfo / QueryStudentExam
5. **P2 — 通知/积分/档案**
6. **P2 — 民主评价**
