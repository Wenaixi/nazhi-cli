# 真实 API 响应参考（用于脱敏后更新文档）

> 本文件记录从真实纳智平台获取的 JSON 响应，仅用于更新 `docs/sdk/README.md` 文档。
> 更新完文档后此文件可删除。所有信息在写入文档前需完全脱敏。

获取时间：2026-07-11

---

## whoami (GetMyInfo / ActivateSession)

```json
{
  "id": 327053,
  "name": "高博文",
  "studentNumber": "G350181200912110035",
  "studentId": 387020,
  "studyNumber": "2508010404",
  "nationalStudentNumber": "G350181200912110035",
  "schoolId": 173,
  "schoolName": "福清一中",
  "gradeId": 27900,
  "gradeName": "高一",
  "classId": 162647,
  "className": "八班",
  "seat": 29
}
```

## task list (FetchTasks)

第1条（艺术素养）：

```json
{
  "id": 18160,
  "name": "2026年\"青春唱响逐新章，美育涵养润芳华\"班班有歌声",
  "typeName": "参加的艺术活动项目",
  "dimensionName": "艺术素养",
  "hours": 4,
  "score": 1,
  "remark": "2026年\"青春唱响逐新章，美育涵养润芳华\"班班有歌声4个小时",
  "submitted": false,
  "needPic": true,
  "circleTaskStatus": "上传期 未提交",
  "upPic": 1,
  "startDateStr": "2026-06-30T00:00:00+08:00",
  "endDateStr": "2026-07-30T00:00:00+08:00",
  "auditStartDateStr": "2026-07-31T00:00:00+08:00",
  "auditEndDateStr": "2026-09-30T00:00:00+08:00",
  "creatorName": "管理员",
  "roleName": "班主任",
  "creationTime": [2026, 6, 30, 11, 39, 19],
  "creationTimeStr": "2026-06-30T00:00:00+08:00",
  "termId": 18,
  "pushNum": 1,
  "scopeType": 2,
  "scopeTypeName": "年段任务"
}
```

社会实践 8h：

```json
{
  "id": 18162,
  "name": "2025-2026第二学期调查表8小时",
  "typeName": "社会调查",
  "dimensionName": "社会实践",
  "hours": 8,
  "score": 1,
  "remark": "2025-2026第二学期调查表8小时",
  "submitted": false,
  "needPic": false,
  "circleTaskStatus": "上传期 未提交",
  "upPic": 0,
  "startDateStr": "2026-06-30T00:00:00+08:00",
  "endDateStr": "2026-07-30T00:00:00+08:00",
  "auditStartDateStr": "2026-07-31T00:00:00+08:00",
  "auditEndDateStr": "2026-09-30T00:00:00+08:00",
  "creatorName": "管理员",
  "roleName": "班主任",
  "creationTime": [2026, 6, 30, 11, 45, 22],
  "creationTimeStr": "2026-06-30T00:00:00+08:00",
  "termId": 18,
  "pushNum": 1,
  "scopeType": 2,
  "scopeTypeName": "年段任务"
}
```

社会实践 120h：

```json
{
  "id": 18161,
  "name": "2025-2026第二学期暑期社会实践",
  "typeName": "其他",
  "dimensionName": "社会实践",
  "hours": 120,
  "score": 1,
  "remark": "2025-2026第二学期暑期社会实践",
  "submitted": false,
  "needPic": true,
  "circleTaskStatus": "上传期 未提交",
  "upPic": 1,
  "startDateStr": "2026-06-30T00:00:00+08:00",
  "endDateStr": "2026-07-30T00:00:00+08:00",
  "auditStartDateStr": "2026-07-31T00:00:00+08:00",
  "auditEndDateStr": "2026-09-30T00:00:00+08:00",
  "creatorName": "管理员",
  "roleName": "班主任",
  "creationTime": [2026, 6, 30, 11, 42, 31],
  "creationTimeStr": "2026-06-30T00:00:00+08:00",
  "termId": 18,
  "pushNum": 1,
  "scopeType": 2,
  "scopeTypeName": "年段任务"
}
```

主题班会样例：

```json
{
  "id": 18296,
  "name": "20260706高一（8）班期末相关事宜及暑期安全教育",
  "typeName": "主题班会",
  "dimensionName": "思想品德",
  "hours": 0.5,
  "score": 1,
  "remark": "照片+心得",
  "submitted": false,
  "needPic": true,
  "circleTaskStatus": "上传期 未提交",
  "upPic": 1,
  "startDateStr": "2026-07-10T00:00:00+08:00",
  "endDateStr": "2026-07-18T00:00:00+08:00",
  "auditStartDateStr": "2026-07-19T00:00:00+08:00",
  "auditEndDateStr": "2026-07-22T00:00:00+08:00",
  "creatorName": "许风华",
  "roleName": "班主任",
  "creationTime": [2026, 7, 4, 9, 46, 56],
  "creationTimeStr": "2026-07-04T00:00:00+08:00",
  "termId": 18,
  "pushNum": 0,
  "scopeType": 1,
  "scopeTypeName": "班级任务"
}
```

## task submitted (GetSubmittedCircles)

第1条：

```json
{
  "id": 5425144,
  "name": "国旗下讲话",
  "content": "听完这次国旗下讲话，我明白了爱护校园环境是每位同学的责任...（完整内容过长，脱敏后取前50字示范）",
  "typeName": "",
  "approved": false,
  "circleDate": "0001-01-01T00:00:00Z",
  "hours": 0.5,
  "imgList": [
    {
      "id": 5025679,
      "circle_id": 5425144,
      "class_id": 162647,
      "task_id": 18142,
      "attachment_id": 5041653,
      "imgPath": ".jpg"
    }
  ],
  "imgPreViewList": [
    "http://www.nazhisoft.com/common/attachment/getImg?id=5041653"
  ],
  "remark": "爱护校园环境，共创美丽校园"
}
```

更多条目的结构和上面一致，只是 id/content/remark/attachment_id 不同。

## task submit (SubmitTask)

真实成功提交（4月生产劳动，已脱敏）：

请求输入（CLI / SDK 最小输入）：

```json
{
  "taskId": 18155,
  "content": "手握扫帚净校园，春意盎然拂面来。每一次躬身劳动，都是对责任与成长的最好诠释。",
  "imagePaths": ["/abs/path/to/photo.jpg"]
}
```

SDK 内部组装后的真实 addCircle 请求（按当前实现与平台成功响应记录整理）：

```json
{
  "id": null,
  "name": "",
  "hostName": "",
  "circleDate": "",
  "rank": "",
  "level": "5",
  "content": "手握扫帚净校园，春意盎然拂面来。每一次躬身劳动，都是对责任与成长的最好诠释。",
  "pictureList": [5043373],
  "circleTaskId": 18155,
  "circleTypeId": 9274,
  "dimensionId": 14,
  "hours": 2,
  "circleBeginDate": "",
  "circleEndDate": "",
  "checkResult": "",
  "patentType": "",
  "patentNum": "",
  "address": "示例中学",
  "termName": "",
  "activityName": "",
  "sportsName": "",
  "teamName": "",
  "orgName": "示例中学",
  "resultsName": "",
  "obtainTime": "",
  "specialtyTechnology": "",
  "playRole": "",
  "likeSpecialty1": "",
  "likeSpecialty2": "",
  "likeSpecialty3": ""
}
```

真实平台响应：

```json
{
  "code": 1,
  "msg": "成功"
}
```

失败样例（3月生产劳动，已存在记录）：

```json
{
  "code": -1,
  "msg": "发表写实失败，限制本写实活动只能发表1次"
}
```
## self-eval status (QuerySelfEvaluation)

```json
{
  "id": 372235,
  "studentComment": "这学期我尽量保持学习的专注，每天按时完成作业，课堂上也坚持记笔记...（完整内容过长，文档需脱敏缩写）",
  "teacherComment": "你开朗、乐观，在课上你认真听讲，积极发言...（完整内容过长，文档需脱敏缩写）"
}
```

## file upload (UploadFile)

真实平台响应（已脱敏）：

```json
{
  "attachmentID": 5041963
}
```

说明：真实上传服务原始 `returnData` 里还包含 `path / name / fileServer / creationTime`，SDK 公开返回值只保留 `attachmentID`。

## file download (DownloadFile)

CLI / SDK 执行结果：

- 输入：`attachmentID=5041963`
- 结果：下载成功，保存到本地文件
- SDK 返回：`nil`

## self-eval submit (SubmitSelfEvaluation)

真实平台响应：

```json
null
```

CLI 成功 envelope：

```json
{
  "status": "success",
  "code": 204,
  "message": "自我评价提交成功",
  "data": null
}
```

## honor add (AddHonor)

真实平台响应：

```json
null
```

CLI 成功 envelope：

```json
{
  "status": "success",
  "code": 204,
  "message": "荣誉申报成功",
  "data": null
}
```

## honor delete (DeleteHonor)

真实平台响应：

```json
null
```

CLI 成功 envelope：

```json
{
  "status": "success",
  "code": 204,
  "message": "荣誉记录已删除",
  "data": null
}
```

## honor types (GetHonorTypes)

按 id 排序前几条：

```json
[
  {"id": 1147, "name": "校学生优秀干部", "levelName": "", "level": 5, "dimensionName": ""},
  {"id": 1148, "name": "校三好学生", "levelName": "", "level": 5, "dimensionName": ""},
  {"id": 1150, "name": "二级运动员", "levelName": "", "level": 1, "dimensionName": ""},
  {"id": 1151, "name": "校优秀团干部", "levelName": "", "level": 5, "dimensionName": ""},
  {"id": 1251, "name": "福清市\"三好学生\"", "levelName": "", "level": 4, "dimensionName": ""}
]
```

> 注意：真实 API 返回的 `levelName` 和 `dimensionName` 为空字符串，与当前文档不同。

## honor list (GetHonorList)

```json
{
  "records": [
    {
      "id": 56241,
      "typeName": "",
      "levelName": "",
      "level": 5,
      "dimensionName": "",
      "approved": false,
      "approvedName": "",
      "getDate": "0001-01-01T00:00:00Z",
      "evaluationAgency": ""
    }
  ],
  "page": {
    "pageNo": 1,
    "pageSize": 20,
    "totalNum": 1,
    "totalPage": 1
  }
}
```

> 注意：该账号只有一条荣誉记录且各字段为空，不适合做文档示例。
> 建议文档示例改用构造的典型记录。
