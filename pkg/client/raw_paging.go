package client

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
)

// fetchRawCirclePages 并发抓取第 2..pages 页的原始字节，按页号下标落位。
//
// results[1] 由调用方预先填好首页快照；本函数只写 2..pages。槽位 [0]
// 恒为零值（页号从 1 起），失败页的槽位保持零值——调用方据此判断
// 哪些页真实到手。返回首个失败，已包装「第 N 页失败」页号上下文。
//
// 只调度、不加工：单页结果被原样写入槽位，本函数不解析、不重排、
// 不重新序列化页内容。byte-for-byte 透传契约由这一语义保证。
//
// 翻页前的字节预算闸必须由调用方在进入本函数之前完成（见
// getCirclesJSON 与 getCirclesLimitJSON）。若把预算判断挪进来，
// 会复现「请求已全部发出才截断」的历史缺陷：闸的意义正是在
// 发出请求之前拦住，而不是事后回收。
func (c *Client) fetchRawCirclePages(ctx context.Context, token string, pages, pageSize, circleType int, key string, results []rawResult) error {
	if pages < 2 {
		return nil
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrentPageLimit)

	for pageNo := 2; pageNo <= pages; pageNo++ {
		pn := pageNo
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				return err
			}
			_, raw, err := c.fetchCirclePageJSON(gctx, token, pn, pageSize, circleType, key)
			if err != nil {
				return fmt.Errorf("第 %d 页失败: %w", pn, err)
			}
			results[pn] = rawResult{raw: raw}
			return nil
		})
	}

	return g.Wait()
}
