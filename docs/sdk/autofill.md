# SDK 自动补全与默认行为总表

本文对照 `pkg/client` 源码，列出**所有**「调用方可不填 / 由 SDK 自动完成」的行为。  
原则：前端用户 v-model 才暴露；前端/SDK 能自动填的不要求调用方填；**禁止**发明前端没有的默认（如空地址→学校名）。

> 写入口细则见各域分册；本页是索引与对照表。

---

## 总览

| 域 | 自动行为摘要 | 分册 |
|----|--------------|------|
| 认证 | 空 `SchoolID`→按学号查学校；验证码 OCR | [auth.md](./auth.md) |
| Session | HAR 4 步；缓存 UserInfo；同 token 快速路径 / 失败 backoff | [session.md](./session.md) |
| 用户读 | Session 预热复用；`schoolId`/`schoolName` 用**学号**走 SSO 补全；班级名去年级前缀 | [user.md](./user.md) |
| 用户写 | 中文性别/团员/民族/证件→数字；忽略全国学籍号；更新后清缓存 | [user.md](./user.md) |
| 写实提交 | 任务元数据 id；学时半自动；本地图上传；**不**填学校名/默认 level | [task.md](./task.md) |
| 写实列表 | 自动翻页合并；`key` 透传 | [circle-list.md](./circle-list.md) |
| 荣誉 | typeName 反查；name 回落 typeName；score 默认 0 | [honor.md](./honor.md) |
| 典型案例 | typeName/roleName/levelName 按 code；Update 支持 number code | [typical-case.md](./typical-case.md) |
| 自评 | 结构化 form 双层 `JSON.stringify` | [self-eval.md](./self-eval.md) |
| 文件 | 转 JPG、压≤5MB、剥鉴权头、filename 仅 basename | [file.md](./file.md) |
| 业务 HTTP | 多数业务方法先 `ActivateSession`；`X-Auth-Token` Cookie+Header | 各域 |

---

## 认证 Login

| 条件 | SDK 行为 | 源码 |
|------|----------|------|
| `LoginRequest.SchoolID == ""` | `GetSchoolID(ctx, Username)`，用返回的 `schoolId` 登录 | `auth.go` Login |
| 始终 | `InitSession` → 并发 OCR 验证码（调用方**无** Captcha 字段） | 同上 |
| `c.ocr == nil` | 直接 `ErrOCRNotConfigured` | 同上 |

`Username` 即登录学号，**不会**被 SDK 改成别的默认学号；学校 ID 才是「按学号自动查」。

---

## 用户 GetMyInfo / 后处理

| 条件 | SDK 行为 | 源码 |
|------|----------|------|
| 调用 `GetMyInfo` | 先 `ActivateSession`；若本次激活拿到 UserInfo 则**不再**打第二遍 getMyInfo | `user.go` |
| `StudentNumber != ""` 且 (`SchoolID==0` **或** `SchoolName==""`) | `GetSchoolID(ctx, StudentNumber)` 补 `schoolId` / `schoolName` | `postProcessUserInfo` |
| `ClassName` 以 `GradeName` 为前缀 | 去掉年级前缀（如「高一(8)班」在已有 gradeName 时整理 className） | 同上 |

说明：

- **学号本身**来自平台 `getMyInfo`，不是 SDK 编造。  
- **用学号自动补的是学校信息**（SSO 公开接口，无需 token）。  
- 写实提交**不会**把学号/学校名写进 `addCircle` body（身份靠 token）。

---

## Session

| 行为 | 说明 |
|------|------|
| 4 步 HAR | `/` → getMenu×2 → getMyInfo |
| 缓存 | 同 Client + 同 token 已激活 → 直接返回缓存 UserInfo |
| 失败 | backoff 窗口内 `ErrSessionBackoff` |
| 与 GetMyInfo | 步骤 4 与 GetMyInfo 共享用户资料路径 |

---

## 写实 SubmitTask / EditCircle

| 字段/步骤 | 用户 | SDK |
|-----------|------|-----|
| `circleTaskId` / `circleTypeId` / `dimensionId` | 只传 `TaskID` | `GetCircleTypeByTaskID` |
| `hours` | 可空字符串 | meta.hours>0 → 用预设；meta≤0 且空 → `ErrInvalidPayload`；非空优先用户 |
| `pictureList` | `ImagePaths` 和/或 `ImageIDs` | 路径逐个 `UploadFile` 合并 id |
| 备注要求图 | — | remark 含「照片/图片/pdf」且无图 → `ErrInvalidPayload` |
| `address` / `orgName` / `level` / 其它活动字段 | 手填；空串**原样** | **不**填学校名、**不**默认 `"5"` |
| `circleDate` / `termName` | 兼容可空 | 不自动造值（前端无 v-model） |
| 学号 / 姓名 / 学校 | 不出现在 Input | 不写入 payload（服务端靠 token） |

---

## 荣誉 AddHonor / UpdateHonor

| 字段 | 用户 | SDK |
|------|------|-----|
| typeId / level / evaluationAgency / getDate / certImgAttachmentId | 填 | — |
| typeName | 可空 | `GetHonorTypeOptions` 按 typeId 反查；找不到 → `ErrInvalidPayload` |
| name | 可空（前端表单项已注释） | 空则 = typeName |
| score | 可显式 | 零值也序列化，默认 **0** |
| Update map | typeId 有、typeName 空 | 同样反查 typeName（不自动补 name） |

---

## 典型案例

| 字段 | 用户 | SDK |
|------|------|-----|
| title / type / role / level / teacher… / content / 附件 id | 填 code 与正文 | — |
| typeName / roleName / levelName | 可空 | 按 code 映射（见下表）；已填不覆盖 |
| Update `type`/`role`/`level` | string 或 number | 均能补 *Name |

| code | typeName | roleName | levelName |
|------|----------|----------|-----------|
| 1 | 研究性学习报告 | 负责人 | 国际 |
| 2 | 社会调查报告 | 参与者 | 省 |
| 3 | 艺术创作作品 | — | 市 |
| 4 | 其他 | — | 区县 |
| 5 | — | — | 学校 |

列表查询 `status` 变参默认 **3（全部）**。

---

## 用户 UpdateMyInfoStructured

| 输入 | SDK |
|------|-----|
| GenderName 男/女 | gender 1/2 |
| YouthLeague 是/否 | youthLeagueFlag 1/0 |
| NationName | nation 1…8 |
| IdCardType 中文 | idType 1…7 |
| Name | API key `studentName` |
| NationalStudentNumber | **故意不发送** |
| 空串字段 | 跳过（不覆盖服务端） |
| StudentUuid | 始终带 key；空串=不改密码 |
| 成功后 | `InvalidateCachedUserInfo` |

---

## 自评 Structured

| 用户 | SDK |
|------|-----|
| `map` 各字段（bxqhzr…） | `JSON.stringify(form)` 再包 `{"studentComment":"..."}` |

纯文本路径只包一层 `studentComment`。

---

## 文件 UploadFile

| 用户 | SDK |
|------|-----|
| 本地路径 | 解码→白底→JPEG→≤5MB；multipart 文件名 `basename.jpg`；**不带** token/Cookie；`bussinessType=12&groupName=other` |

---

## 列表与其它

| 行为 | 说明 |
|------|------|
| Get*Circles | 自动翻页合并全量；`key` 原样透传 |
| Peek* | 只取 total |
| GetCircleTypes `pid` | `url.QueryEscape` |
| doBiz* | 内部按需 ActivateSession |

---

## 明确不会自动做的事

| 误区 | 实际 |
|------|------|
| 空 address → 学校名 | 否，空串原样 |
| 空 level → `"5"` | 否 |
| 提交写实时自动带学号字段 | 否，靠 token |
| 典型案例 type2 →「社会实践报告」 | 否，是「社会调查报告」 |
| 典型案例 level1 →「国家」 | 否，是「国际」 |
| 荣誉必须手填 typeName/name/score | 否，可全自动 |

---

## 相关

- 分册索引：[README.md](./README.md)  
- CLI 侧行为简述：[../cli/README.md](../cli/README.md#sdk-自动补全对照)  
