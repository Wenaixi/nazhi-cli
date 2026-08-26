# nazhi-cli 源码指引

> 本文档只是地图：告诉你「哪个功能在哪几个文件」。语义、参数、行为一律以源码为准，
> 本文档不随版本更新维护。纳智前端源码为仓库外本地镜像（默认 `E:\newCC\life-new2026\nazhi\`），
> 不随本仓库分发；下表中「前端参照」均指该本地镜像内的相对路径。

Go SDK 三包：`pkg/client`（Client + 业务方法 + Option）、`pkg/types`（领域类型 + 统一解码）、
`pkg/tokenparse`（SSO token 提取）。CLI 层在 `cmd/nazhi/`。

## 表 A：功能 ↔ Go 源码 ↔ 前端源码

| 功能 | SDK 方法 | Go 源码（pkg/） | 前端参照（本地镜像 src/components/） | 业务接口 |
|------|----------|-----------------|---------------------------------------------|----------|
| 登录 | `Login` | client/auth.go | — （SSO 独立页） | SSO validate |
| 会话激活 | `ActivateSession` | client/session.go | layout.vue /homepage → getMenu，header.vue /home getMenu + getMyInfo（首页加载链） | getMenu ×2 / getMyInfo |
| 我的任务列表 | `FetchTasks` | client/task.go | managementLeftBottom.vue, managementRightTop.vue | getDimensions, getCircleStatistics |
| 类别/任务元数据 | `GetCircleTypes`, `GetCircleTasks` | client/circle.go | managementRightTop.vue | getCircleType, getCircleTask |
| 任务提交元数据 | 内部 `getCircleTypeByTaskId` | client/task.go | managementRightBottom.vue `getCircleTypeByTaskId()` | getCircleTypeByTaskId |
| 写实列表（公示/教师/我发布/撤回） | `GetPublicCircles` `GetTeacherCircles` `GetSubmittedCircles` `GetWithdrawnCircles` | client/submitted.go | managementRightBottom.vue（tab 切换 `change(num)`）, main/mainMidSearch.vue | getStudentCircle?type=1/2/3/4 |
| 写实提交 | `SubmitTask`, `PreviewSubmitPayload` | client/task.go | managementRightBottom.vue `submit()` `checkData()` | addCircle |
| 写实编辑 | `EditCircle`, `PreviewEditPayload` | client/task.go | managementRightBottom.vue `openEdit()` | editCircle |
| 写实删除 | `DeleteCircle` | client/circle.go | managementRightBottom.vue `open2()` | deleteCircle |
| 评论 / 点赞 | `AddCircleComment`, `SetCircleLike` | client/circle.go | managementRightBottom.vue `addComment()` `likeIt()` | addCircleComment, setCircleLikeById |
| 写实图片查询 | `GetCircleImages` | client/circle.go | managementLeftTop.vue（图片轮播区） | getCircleImg |
| 荣誉列表/类型 | `GetHonorList`, `GetHonorTypes` 等 | client/honor.go | performance/performanceM.vue（HonorBox.vue 为静态假数据页） | studentMoralEduNew/getHonor* |
| 荣誉增删改 | `AddHonor`, `UpdateHonor`, `DeleteHonor` | client/honor.go | performance/performanceM.vue `submit()`/`submit2()` | addHonor, updateHonor, deleteHonorById |
| 典型案例 CRUD | `GetTypicalCaseList` 等 5 方法 | client/typical_case.go | classic/classiccanter.vue | studentCircleNew/getTypicalCase 等 |
| 自我评价 | `SubmitSelfEvaluation`, `QuerySelfEvaluation` | client/self_eval.go | main/mainLeft.vue | query/addSelfEvaluation |
| 毕业评价 | `SubmitSelfGradEvaluation` 等 | client/self_eval.go | main/mainLeft.vue | query/addSelfGradEvaluation |
| 用户信息读 | `GetMyInfo` | client/user.go | user/userBox.vue | studentInfo/getMyInfo |
| 用户信息写 | `UpdateMyInfo` | client/user_update.go | user/modifyBox.vue | studentInfo/updateMyInfo |
| 文件上传 | `UploadFile` | client/file.go, image_prep.go | el-upload `beforeAvatarUpload` | common/upload/uploadImage |
| 文件下载 | `DownloadFile` | client/file.go | 卡片图片区 `getImg?id=` | common/attachment/getImg |
| token 提取 | `ExtractFromLocation` 等 | tokenparse/tokenparse.go | — | — |
| 原始 JSON 透传 | `*JSON` 方法族 | client/raw_json.go | — | 同各业务接口 |
| CLI 命令层 | — | cmd/nazhi/*.go | — | 一命令一文件 |

## 表 B：写实提交表单 targetType ↔ 字段

来源：managementRightBottom.vue 的 `v-if="N == targetName"` 分支（约 115-386 行）。target 取自 `getCircleTypeByTaskId` 返回的 `dataMap.type`。所有类型共用 `content`（≤200 字）+ 图片（≤2 张）。

| target | 类型 | 专属字段 |
|--------|------|----------|
| 1 | 活动项目 | name, address, hours, playRole |
| 2 | 学科竞赛成绩 | activityName, hostName, obtainTime, rank, level |
| 3 | 体育项目 | sportsName, hostName, obtainTime, rank, level |
| 4 | 艺术活动 | name, hostName, obtainTime, rank, level |
| 5 | 学生艺术团队 | teamName, orgName, checkResult, rank, circleBeginDate, circleEndDate |
| 6 | 实践创新活动 | orgName, address, hours, checkResult |
| 7 | 科技创新/研究性学习成果 | resultsName, hostName, obtainTime, rank, level |
| 8 | 创造发明成果 | resultsName, patentType, patentNum, obtainTime |
| 9 | 爱好项目 | likeSpecialty1-3 |
| 10 | 劳动实践 | orgName, address, hours, level |
| 11 | 代表性劳动活动 | obtainTime, address |
| 12 | 劳动能力技术 | specialtyTechnology |
| 13 | 劳动成果 | resultsName, address, obtainTime |
| 14 | 劳动竞赛 | sportsName, hostName, obtainTime, address, rank, level |

## 表 C：状态字段速查

| 字段 | 含义 | 判定处（本地镜像 src/components/management/managementRightBottom.vue） |
|------|------|------|
| 记录 `type` | 发布者类型：1=学生，2=教师 | 头像 personType、编辑按钮 `item.type==1` |
| 记录 `status` | 0=正常可编辑删除；1=锁定；2=被撤回 | 第 42/46 行 v-if 条件 |
| `auditRemark` | 撤回原因（status=2 时红字展示） | 第 42 行 |
| `ifMySelf` | 是否本人记录（int 0/1） | 操作按钮显隐 |
| 任务 `circleTaskStatus` | 上传期/已结束 + 未提交/已提交 文案 | managementLeftBottom.vue 任务统计 |
| 任务 `submitted` | 布尔完成标志 | CLI `task list` 输出 |

## 参照库升级

前端源码更新后重新镜像：

```powershell
robocopy <前端源码目录> <本地镜像目录> /E /XD .omc artifacts .git node_modules /XF CLAUDE.md
```
