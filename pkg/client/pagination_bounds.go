// pagination_bounds.go：写实分页的页数推导纯函数。
//
// 提取动因（2026-09-26 架构核实）：页数下界推导在 submitted.go 与
// raw_json.go 三处逐字重复（各 4 行），limit 路径的 endPage 收敛又各写一遍。
// 重复导致同类修复要审多份实现与多套测试。
//
// 抽出为纯函数的原因：这些规则是分页安全的不变量（v1.6.4 / 84982af
// 确立），必须只有一处定义，且可被行为矩阵独立验证——它们不依赖
// Client 状态、不发请求，纯 CPU 计算。
//
// 注意：这些函数只决定「要抓几页」，不得改写返回给调用方的 PageBean。
// 服务端声明的 totalPage/totalNum 必须原样透传（raw_json_limit_pages_test.go
// 锁定该契约），推导值仅用于内部翻页决策。
package client

// derivePageBounds 返回翻页时应抓取的页数，已做上界钳制。
//
// 规则：取 max(totalPage, ceil(totalNum/pageSize)) 作为下界，
// 再钳制到 [1, maxTotalPage]。
//
// 为什么用 max：服务端 totalPage 来自单一字段声明，可能虚低或为 0
// （v1.6.4 事故）。只信 totalPage 会静默漏数据，因此 totalNum 推导值
// 作为安全下界。
//
// 为什么钳制上界：totalPage 来自服务端声明，恶意/异常值（如 1e9）直接
// 驱动 make 分配会导致单请求 OOM。上界 maxTotalPage 防的是「放大」，
// 不是「翻页完整性」——超界时截断而非继续翻。
//
// 为什么下界为 1：results 索引槽以页号下标存放，首页固定在 results[1]。
func derivePageBounds(totalNum, totalPage, pageSize int) int {
	return clampPage(derivePageBoundsUnclamped(totalNum, totalPage, pageSize))
}

// derivePageBoundsUnclamped 返回 max(totalPage, ceil(totalNum/pageSize))，
// 不做上界钳制，只保留下界 1。
//
// 单独提供是因为 limit 路径的收敛顺序不同：那里需要「声明页数」的原始值
// 先与 limit 派生值做 min，收敛后才判断是否超上界。若先钳到 maxTotalPage
// 再收敛，虚高的 totalNum（如 1e9）会被当成合法 10000 页而真的去翻页，
// 违反 TestGetCirclesLimitJSON_HugeLimitClamped 锁定的
// 「超限退回首页、不再请求 page2」。
func derivePageBoundsUnclamped(totalNum, totalPage, pageSize int) int {
	if pageSize <= 0 {
		// pageSize 非法时只信任服务端声明。
		if totalPage < 1 {
			return 1
		}
		return totalPage
	}
	derivedPages := (totalNum + pageSize - 1) / pageSize
	pages := totalPage
	if pages < derivedPages {
		pages = derivedPages
	}
	if pages < 1 {
		return 1
	}
	return pages
}

// clampPage 把页数钳制到 [1, maxTotalPage]。
func clampPage(pages int) int {
	if pages > maxTotalPage {
		return maxTotalPage
	}
	if pages < 1 {
		return 1
	}
	return pages
}

// limitEndPage 返回 limit 路径实际需要抓取的最后一页页号。
//
// 规则：endPage = min(ceil((offset+limit)/pageSize), declaredPages)；
// declaredPages 必须是**未钳制**的声明值（见 derivePageBoundsUnclamped）。
//
// 收敛后若 endPage 超 maxTotalPage，直接退回 1（首页快照）而不是钳到上界——
// ：endPage 来自调用方 limit 与服务端 totalNum 声明的组合，虚高时
// 可达百万，make([]rawResult, endPage+1) 一次预分配几十 MB。防放大优先于
// 分页完整性。
//
// 为什么不按 totalPage 翻页再截断：那样会为用不到的数据发出请求
// （raw_json_limit_pages_test.go 锁定「只请求覆盖到的页」）。
func limitEndPage(offset, limit, pageSize, declaredPages int) int {
	if pageSize <= 0 {
		return clampPage(declaredPages)
	}
	need := offset + limit
	endPage := (need + pageSize - 1) / pageSize
	if endPage > declaredPages {
		endPage = declaredPages
	}
	if endPage > maxTotalPage {
		return 1
	}
	if endPage < 1 {
		return 1
	}
	return endPage
}
