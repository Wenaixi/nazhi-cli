package client

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/Wenaixi/nazhi-cli/internal/recoverx"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// fetchTasksConcurrentLimit 是 FetchTasks 并发拉取维度的上限。
//
// 设计权衡：业务系统实际维度数通常 ≤ 20，单次 FetchTasks 并发度受维度数封顶，
// 远低于 DoS 阈值。限制 = min(len(dimensions), 8) 平衡 wall time 与服务端压力：
//   - 8 路并发足够让 20 维度在 ~3 RTT 内完成（vs 串行 20 RTT）
//   - 不会因下游抖动放大熔断风险
//
// 如未来业务接口维度数 > 50，可考虑调到此常量或暴露为 Client 字段。
const fetchTasksConcurrentLimit = 8

// appendLocked 在 mu 锁内安全地追加 items 到 slice。
//
// 消除 FetchTasks goroutine 闭包内重复的 mu.Lock + append + mu.Unlock 模式。
// 使用 *[]T 而非 []T 返回值，避免调用方忽略 slice header realloc 的 bug：
// append 在容量不足时会分配新底层数组，调用方必须用返回值回写原变量。
// 传入指针让 helper 直接修改调用方的 slice header，调用方无需重新赋值。
//
// 泛型支持单元素追加（dimErrs = appendLocked(&mu, &dimErrs, err)）
// 和变长追加（allTasks = appendLocked(&mu, &allTasks, tasks...)）。
func appendLocked[T any](mu *sync.Mutex, slice *[]T, items ...T) {
	mu.Lock()
	*slice = append(*slice, items...)
	mu.Unlock()
}

// fetchDimensions 拉取任务维度列表（FetchTasks / GetDimensions 共用）。
// 内部包含 session 预热 + 响应解码，错误信息前缀由 caller 决定。
func (c *Client) fetchDimensions(ctx context.Context, token string, errPrefix string) ([]types.Dimension, error) {
	dims, err := doBizGetDecode[[]types.Dimension](c, ctx, token, errPrefix, "/api/studentCircleNew/getDimensions",
		func(resp types.UnifiedResponse) (*[]types.Dimension, error) {
			v, err := types.DecodeDataList[types.Dimension](resp)
			if err != nil {
				return nil, err
			}
			return &v, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return *dims, nil
}

// FetchTasks 拉取目标平台全部维度的任务列表。
// 内部流程：ActivateSession → getDimensions → 遍历维度 getCircleStatistics → 聚合。
//
// 并发拉取：多个维度的 getCircleStatistics 通过 errgroup 并发执行，
// 并发上限 = min(len(dimensions), fetchTasksConcurrentLimit)。
// 既享受并发提速（20 维度 ≈ 3 RTT vs 串行 20 RTT），
// 又防止 > 50 维度的业务接口把服务端打爆（无限制 goroutine fan-out 风险）。
//
// 单个维度失败时通过 c.logDebug() 记录（不会中断整体拉取），
// 调用方可通过 client.WithLogger() 注入自定义 logger 捕获详细错误。
func (c *Client) FetchTasks(ctx context.Context, token string) ([]types.Task, error) {
	dimensions, err := c.fetchDimensions(ctx, token, "FetchTasks getDimensions")
	if err != nil {
		return nil, err
	}

	headers := c.bizHeaders(token)

	limit := len(dimensions)
	if limit > fetchTasksConcurrentLimit {
		limit = fetchTasksConcurrentLimit
	}
	// 复用 ParallelDims：让 hypothetical seam 变 real seam。
	// id==0 跳过、gctx 取消检测、appendLocked 收集与错误分类均在 ParallelDims 内完成。
	result, egErr := ParallelDims[types.Task](ctx, dimensions, limit, func(gctx context.Context, dim types.Dimension) ([]types.Task, error) {
		return c.fetchTasksForDimensionSafe(gctx, dim, headers)
	})

	if egErr != nil {
		if isContextError(egErr) {
			if len(result.Items) > 0 {
				return result.Items, fmt.Errorf("%w: FetchTasks context 取消后部分维度成功: %w",
					ErrBusinessRejected,
					fmt.Errorf("%w: %w", ErrRetryable, egErr))
			}
			return nil, fmt.Errorf("%w: FetchTasks 全部维度因 context 取消失败: %w", ErrRetryable, egErr)
		}
		return nil, fmt.Errorf("FetchTasks 并发拉取失败: %w", egErr)
	}

	allTasks := result.Items
	if result.FailedCount == 0 && result.CancelledCount == 0 {
		return allTasks, nil
	}

	bizErrs := result.BizErrors
	ctxErrs := result.ContextErrors
	cancelledCount := result.CancelledCount

	var cancelPlaceholder error
	if cancelledCount > 0 {
		cancelPlaceholder = fmt.Errorf("%w: %d 个维度因 context 取消而失败", ErrRetryable, cancelledCount)
	}

	if len(bizErrs) == 0 && cancelledCount > 0 {
		joined := errors.Join(append(ctxErrs, cancelPlaceholder)...)
		if len(allTasks) == 0 {
			return nil, joined
		}
		return allTasks, fmt.Errorf("%w: FetchTasks context 取消后部分维度成功: %w",
			ErrBusinessRejected, joined)
	}

	joined := errors.Join(append(append(bizErrs, ctxErrs...), cancelPlaceholder)...)
	failedCount := len(bizErrs)

	if len(allTasks) == 0 {
		return nil, fmt.Errorf("%w: FetchTasks 全部 %d 个维度均失败: %w",
			ErrBusinessRejected, failedCount, joined)
	}

	return allTasks, fmt.Errorf("%w: FetchTasks %d 个维度部分失败: %w",
		ErrBusinessRejected, failedCount, joined)
}
func (c *Client) fetchTasksForDimension(ctx context.Context, dim types.Dimension, headers map[string]string) (tasks []types.Task, err error) {
	// 上下文取消（Canceled/DeadlineExceeded）直接 propagate，
	// 不吞掉走 best-effort——调用方需要知道 context 信号已触发，才能正确区分
	// 「真空数据」与「被取消」。
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// 说明：int64 参数纯数字，直接 strconv.FormatInt 拼接 URL 安全，
	// 无需 URL 编码（数字不包含特殊字符）。如需未来扩展为字符串参数，
	// 应改用 url.Values.Encode()。
	statURL := c.bizURL("/api/studentCircleNew/getCircleStatistics") + "?dimensionId=" + strconv.FormatInt(dim.ID, 10)
	statBody, err := c.httpDo(ctx, http.MethodGet, statURL, nil, headers, "")
	if err != nil {
		if isContextError(err) {
			return nil, err // 上下文取消应 propagate，不做 best-effort 吞没
		}
		c.logDebug("FetchTasks 维度 %d(%s) 请求失败: %v", dim.ID, dim.Name, err)
		return nil, err // propagate 网络错误到 dimErrs，不再静默吞咽
	}

	statResp, err := types.DecodeResponse(statBody)
	if err != nil {
		c.logDebug("FetchTasks 维度 %d(%s) 响应解析失败: %v", dim.ID, dim.Name, err)
		return nil, err // propagate 解析错误到 dimErrs，不再静默吞咽
	}
	if err := types.CheckCode(statResp); err != nil {
		return nil, fmt.Errorf("%w: 维度 %d(%s) 业务错误: %w", ErrBusinessRejected, dim.ID, dim.Name, err)
	}

	tasks, err = types.DecodeDataList[types.Task](statResp)
	if err != nil {
		c.logDebug("FetchTasks 维度 %d(%s) 任务解析失败: %v", dim.ID, dim.Name, err)
		return nil, err // propagate 任务列表解析错误到 dimErrs，不再静默吞咽
	}

	for i := range tasks {
		tasks[i].DimensionName = dim.Name
		tasks[i].SetSubmittedByStatus()
		tasks[i].SetNeedPicFromUpPic()
	}
	return tasks, nil
}

// fetchTasksForDimensionSafe 是 fetchTasksForDimension 的 panic-safe 包装。
//
// errgroup.Go 闭包内无 panic recover 时，nil deref 或
// 第三方库 panic 会逃逸到 runtime → 进程崩溃 → g.Wait() 永不返回。
// 此 helper 在维度粒度捕获 panic，把它当业务错误记录到 dimErrs，
// 防止单个维度的 panic 影响其他维度的并发拉取。
//
// panic 信息：包含 dim.ID + dim.Name 便于排查（panic 路径无法
// 依赖 errgroup 自带的 nil-safe 包装，必须自己构建可读错误）。
//
// 错误链保留（F10.1）：recover() 返回的是 any，r 是 error 时走 %w
// 保留 chain，让 SDK 用户能用 errors.Is 识别 panic 根因（典型场景：
// mock 误实现 panic(errors.New("xxx")) → 调试时能直接定位根 error）。
func (c *Client) fetchTasksForDimensionSafe(ctx context.Context, dim types.Dimension, headers map[string]string) (tasks []types.Task, err error) {
	defer func() {
		if err2 := recoverx.RecoverPanic(recover(), nil, fmt.Sprintf("维度 %d(%s)", dim.ID, dim.Name)); err2 != nil {
			tasks = nil
			err = err2
		}
	}()
	return c.fetchTasksForDimension(ctx, dim, headers)
}

func (c *Client) buildTaskSubmitPayload(ctx context.Context, token string, input types.TaskSubmitInput) (*types.TaskAddCirclePayload, error) {
	return c.buildTaskPayload(ctx, token, input, "SubmitTask")
}

// parseHours 解析写实提交的时长，对齐前端 hoursStatus 逻辑：
//   - 用户非空：解析为 float；非法则 ErrInvalidPayload
//   - 用户空且 metaHours > 0：用任务预设（前端只读自动填）
//   - 用户空且 metaHours <= 0：ErrInvalidPayload（前端可编辑且 checkData 常要求非空）
var hoursRequiredTarget = map[int]bool{1: true, 6: true, 10: true}

func parseHours(userInput string, metaHours float64, targetType int) (float64, error) {
	h := strings.TrimSpace(userInput)
	if h == "" {
		if metaHours > 0 {
			return metaHours, nil
		}
		if hoursRequiredTarget[targetType] {
			return 0, fmt.Errorf("%w: hours 必填（任务未预设学时，须由调用方填写）", ErrInvalidPayload)
		}
		return 0, nil
	}
	parsed, err := strconv.ParseFloat(h, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, fmt.Errorf("%w: hours 非法: %q", ErrInvalidPayload, h)
	}
	return parsed, nil
}

// buildTaskPayload 是 payload 构建的公共逻辑，供 SubmitTask 和 EditCircle 共用。
//
// 参数说明：
//   - input: 实现 TaskInput 接口的输入（TaskSubmitInput 或 TaskEditInput）
//   - callerName: 调用方名称，用于错误信息前缀
//
// 对齐前端（输入暴露原则）：
//   - circleTaskId/circleTypeId/dimensionId/hours(预设>0)：SDK 从 getCircleTypeByTaskId 自动填
//   - Address/OrgName/Level/PlayRole 等：用户填什么发什么；空串原样，不发明学校名或等级「5」
func (c *Client) buildTaskPayload(ctx context.Context, token string, input types.TaskInput, callerName string) (*types.TaskAddCirclePayload, error) {
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidPayload, err)
	}

	meta, err := c.GetCircleTypeByTaskID(ctx, token, input.GetTaskID())
	if err != nil {
		return nil, fmt.Errorf("%s 获取任务元数据失败: %w", callerName, err)
	}

	hours, err := parseHours(input.GetHours(), meta.Hours, meta.Type)
	if err != nil {
		return nil, err
	}

	// 处理图片：合并 ImageIDs + ImagePaths
	pictureList := make([]int64, 0, len(input.GetImageIDs())+len(input.GetImagePaths()))
	for _, id := range input.GetImageIDs() {
		if id <= 0 {
			continue
		}
		pictureList = append(pictureList, id)
	}
	for _, path := range input.GetImagePaths() {
		if strings.TrimSpace(path) == "" {
			continue
		}
		result, upErr := c.UploadFile(ctx, path)
		if upErr != nil {
			if len(pictureList) > 0 {
				return nil, fmt.Errorf("%s 上传图片失败（已上传的 attachmentID: %v）: %w", callerName, pictureList, upErr)
			}
			return nil, fmt.Errorf("%s 上传图片失败: %w", callerName, upErr)
		}
		pictureList = append(pictureList, result.AttachmentID)
	}

	// 验证任务元数据中的图片要求
	if meta.Remark != "" && len(pictureList) == 0 {
		lowerRemark := strings.ToLower(meta.Remark)
		if strings.Contains(meta.Remark, "照片") || strings.Contains(meta.Remark, "图片") || strings.Contains(lowerRemark, "pdf") {
			return nil, fmt.Errorf("%w: 该任务要求上传图片或附件", ErrInvalidPayload)
		}
	}

	// 用户字段：Trim 后原样写入；前端不会自动填学校名 / 默认等级 5
	address := strings.TrimSpace(input.GetAddress())
	playRole := strings.TrimSpace(input.GetPlayRole())
	level := strings.TrimSpace(input.GetLevel())
	name := strings.TrimSpace(input.GetName())
	hostName := strings.TrimSpace(input.GetHostName())
	circleDate := strings.TrimSpace(input.GetCircleDate())
	rank := strings.TrimSpace(input.GetRank())
	activityName := strings.TrimSpace(input.GetActivityName())
	sportsName := strings.TrimSpace(input.GetSportsName())
	teamName := strings.TrimSpace(input.GetTeamName())
	orgName := strings.TrimSpace(input.GetOrgName())
	resultsName := strings.TrimSpace(input.GetResultsName())
	obtainTime := strings.TrimSpace(input.GetObtainTime())
	specialtyTechnology := strings.TrimSpace(input.GetSpecialtyTechnology())
	likeSpecialty1 := strings.TrimSpace(input.GetLikeSpecialty1())
	likeSpecialty2 := strings.TrimSpace(input.GetLikeSpecialty2())
	likeSpecialty3 := strings.TrimSpace(input.GetLikeSpecialty3())

	payload := &types.TaskAddCirclePayload{
		ID:                  input.GetID(),
		Name:                name,
		HostName:            hostName,
		CircleDate:          circleDate,
		Rank:                rank,
		Level:               level,
		Content:             strings.TrimSpace(input.GetContent()),
		PictureList:         pictureList,
		CircleTaskID:        meta.TaskID,
		CircleTypeID:        meta.CircleTypeID,
		DimensionID:         meta.DimensionID,
		Hours:               hours,
		CircleBeginDate:     strings.TrimSpace(input.GetCircleBeginDate()),
		CircleEndDate:       strings.TrimSpace(input.GetCircleEndDate()),
		CheckResult:         strings.TrimSpace(input.GetCheckResult()),
		PatentType:          strings.TrimSpace(input.GetPatentType()),
		PatentNum:           strings.TrimSpace(input.GetPatentNum()),
		Address:             address,
		TermName:            strings.TrimSpace(input.GetTermName()),
		ActivityName:        activityName,
		SportsName:          sportsName,
		TeamName:            teamName,
		OrgName:             orgName,
		ResultsName:         resultsName,
		ObtainTime:          obtainTime,
		SpecialtyTechnology: specialtyTechnology,
		PlayRole:            playRole,
		LikeSpecialty1:      likeSpecialty1,
		LikeSpecialty2:      likeSpecialty2,
		LikeSpecialty3:      likeSpecialty3,
	}

	return payload, nil
}

// decodeSubmitResult 将 doBizAndDecode 的响应/错误映射为 TaskResult。
//
// SubmitTask 和 EditCircle 共用的后处理逻辑：业务错误提取 code/msg 到
// TaskResult（同时保留 error 供 envelope 识别 partial），成功时从 resp 提取。
func decodeSubmitResult(resp *types.UnifiedResponse, err error) (*types.TaskResult, error) {
	if err != nil {
		var bizErr *types.BusinessError
		if errors.As(err, &bizErr) {
			return &types.TaskResult{Code: bizErr.Code, Msg: bizErr.Msg}, err
		}
		return nil, err
	}
	return &types.TaskResult{
		Code: resp.Code,
		Msg:  types.DerefOr(resp.Msg, ""),
	}, nil
}

// EditCircle 修改一条已提交的写实记录。
//
// 与 SubmitTask 对称：内部自动完成 getCircleTypeByTaskId → 图片上传 → 组装 editCircle payload。
// 区别：
//   - SubmitTask 调用 addCircle（无 id 字段，新增记录）
//   - EditCircle 调用 editCircle（必须传 id 字段，修改已有记录）
//
// 用户字段（address/level/playRole 等）空串原样发送，不发明学校名或等级 5；
// 任务元数据与图片由 SDK 自动补齐。
func (c *Client) EditCircle(ctx context.Context, token string, input types.TaskEditInput) (*types.TaskResult, error) {
	payload, err := c.buildTaskEditPayload(ctx, token, input)
	if err != nil {
		return nil, err
	}

	resp, err := c.doBizAndDecode(ctx, token, "EditCircle", "/api/studentCircleNew/editCircle", http.MethodPost, payload)
	return decodeSubmitResult(resp, err)
}

// buildTaskEditPayload 构建 editCircle 的完整请求体。
//
// 与 buildTaskSubmitPayload 共享核心逻辑，区别仅在于 ID 字段的处理：
//   - buildTaskSubmitPayload: ID = nil（新增记录）
//   - buildTaskEditPayload: ID = input.ID（修改已有记录）
func (c *Client) buildTaskEditPayload(ctx context.Context, token string, input types.TaskEditInput) (*types.TaskAddCirclePayload, error) {
	return c.buildTaskPayload(ctx, token, input, "EditCircle")
}

// 公开接口仅接收最少必要输入，SDK 内部自动补齐真实网页提交流程所需字段：
// getCircleTypeByTaskId → UploadFile → addCircle（不再发明学校名/默认等级）。
func (c *Client) SubmitTask(ctx context.Context, token string, input types.TaskSubmitInput) (*types.TaskResult, error) {
	payload, err := c.buildTaskSubmitPayload(ctx, token, input)
	if err != nil {
		return nil, err
	}

	resp, err := c.doBizAndDecode(ctx, token, "SubmitTask", "/api/studentCircleNew/addCircle", http.MethodPost, payload)
	return decodeSubmitResult(resp, err)
}

// GetCircleTypeByTaskID 获取任务提交前所需的 circleTypeId / dimensionId / hours 等元数据。
//
// 真实网页在打开任务提交通道前会先请求 getCircleTypeByTaskId，再用返回的 dataMap
// 填充 addCircle 请求体。SDK 暴露该方法，避免调用方手工猜测 circleTypeId。
// PreviewSubmitPayload 预览提交时的最终 payload（不发请求，仅用于“暴露预设值”）。
//
// 语义与 SubmitTask 完全一致：内部走同样的 GetCircleTypeByTaskID → parseHours → 图片合并 → 30 字段组装，
// 但不会 POST /api/studentCircleNew/addCircle。调用方可据此检查：
//   - 预设的 circleTaskId/circleTypeId/dimensionId/hours 是否如预期
//   - 空的 address/level 是否保持为空（不会被偷填成学校名/"5"）
//   - 14 类任务各自必填的字段是否已就位
//
// 对齐前端：前端是 JSON.stringify(form) 原样发送，SDK 预览也是 Trim 后原样 + 预设回填，不发明默认值。
func (c *Client) buildPreviewPayload(ctx context.Context, token string, input types.TaskInput, callerName string) (*types.TaskAddCirclePayload, error) {
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidPayload, err)
	}
	meta, err := c.GetCircleTypeByTaskID(ctx, token, input.GetTaskID())
	if err != nil {
		return nil, fmt.Errorf("%s 获取任务元数据失败: %w", callerName, err)
	}
	hours, err := parseHours(input.GetHours(), meta.Hours, meta.Type)
	if err != nil {
		return nil, err
	}
	pictureList := make([]int64, 0, len(input.GetImageIDs()))
	for _, id := range input.GetImageIDs() {
		if id <= 0 {
			continue
		}
		pictureList = append(pictureList, id)
	}
	address := input.GetAddress()
	playRole := input.GetPlayRole()
	level := input.GetLevel()
	name := input.GetName()
	hostName := input.GetHostName()
	circleDate := input.GetCircleDate()
	rank := input.GetRank()
	activityName := input.GetActivityName()
	sportsName := input.GetSportsName()
	teamName := input.GetTeamName()
	orgName := input.GetOrgName()
	resultsName := input.GetResultsName()
	obtainTime := input.GetObtainTime()
	specialtyTechnology := input.GetSpecialtyTechnology()
	likeSpecialty1 := input.GetLikeSpecialty1()
	likeSpecialty2 := input.GetLikeSpecialty2()
	likeSpecialty3 := input.GetLikeSpecialty3()
	// 对齐 buildTaskPayload 的 Trim 语义：空串保持空，不发明
	address = strings.TrimSpace(address)
	playRole = strings.TrimSpace(playRole)
	level = strings.TrimSpace(level)
	name = strings.TrimSpace(name)
	hostName = strings.TrimSpace(hostName)
	circleDate = strings.TrimSpace(circleDate)
	rank = strings.TrimSpace(rank)
	activityName = strings.TrimSpace(activityName)
	sportsName = strings.TrimSpace(sportsName)
	teamName = strings.TrimSpace(teamName)
	orgName = strings.TrimSpace(orgName)
	resultsName = strings.TrimSpace(resultsName)
	obtainTime = strings.TrimSpace(obtainTime)
	specialtyTechnology = strings.TrimSpace(specialtyTechnology)
	likeSpecialty1 = strings.TrimSpace(likeSpecialty1)
	likeSpecialty2 = strings.TrimSpace(likeSpecialty2)
	likeSpecialty3 = strings.TrimSpace(likeSpecialty3)
	payload := &types.TaskAddCirclePayload{
		ID:                  input.GetID(),
		Name:                name,
		HostName:            hostName,
		CircleDate:          circleDate,
		Rank:                rank,
		Level:               level,
		Content:             strings.TrimSpace(input.GetContent()),
		PictureList:         pictureList,
		CircleTaskID:        meta.TaskID,
		CircleTypeID:        meta.CircleTypeID,
		DimensionID:         meta.DimensionID,
		Hours:               hours,
		CircleBeginDate:     strings.TrimSpace(input.GetCircleBeginDate()),
		CircleEndDate:       strings.TrimSpace(input.GetCircleEndDate()),
		CheckResult:         strings.TrimSpace(input.GetCheckResult()),
		PatentType:          strings.TrimSpace(input.GetPatentType()),
		PatentNum:           strings.TrimSpace(input.GetPatentNum()),
		Address:             address,
		TermName:            strings.TrimSpace(input.GetTermName()),
		ActivityName:        activityName,
		SportsName:          sportsName,
		TeamName:            teamName,
		OrgName:             orgName,
		ResultsName:         resultsName,
		ObtainTime:          obtainTime,
		SpecialtyTechnology: specialtyTechnology,
		PlayRole:            playRole,
		LikeSpecialty1:      likeSpecialty1,
		LikeSpecialty2:      likeSpecialty2,
		LikeSpecialty3:      likeSpecialty3,
	}
	return payload, nil
}

func (c *Client) PreviewSubmitPayload(ctx context.Context, token string, input types.TaskSubmitInput) (*types.TaskAddCirclePayload, error) {
	return c.buildPreviewPayload(ctx, token, input, "PreviewSubmitPayload")
}

// PreviewEditPayload 预览编辑时的最终 payload（不发请求，纯预览不上传）。
// 与 EditCircle 共用同一字段映射，但不执行 UploadFile，避免本地文件副作用。
func (c *Client) PreviewEditPayload(ctx context.Context, token string, input types.TaskEditInput) (*types.TaskAddCirclePayload, error) {
	return c.buildPreviewPayload(ctx, token, input, "PreviewEditPayload")
}

func (c *Client) GetCircleTypeByTaskID(ctx context.Context, token string, taskID int64) (*types.TaskCircleTypeInfo, error) {
	path := "/api/studentCircleNew/getCircleTypeByTaskId?taskId=" + strconv.FormatInt(taskID, 10)
	return doBizGetDecode[types.TaskCircleTypeInfo](c, ctx, token, "GetCircleTypeByTaskID", path,
		types.DecodeDataMap[types.TaskCircleTypeInfo],
	)
}

// GetDimensions 获取任务维度列表。
//
// SDK 高级用户使用，CLI 暂未暴露此命令。
func (c *Client) GetDimensions(ctx context.Context, token string) ([]types.Dimension, error) {
	return c.fetchDimensions(ctx, token, "GetDimensions")
}
