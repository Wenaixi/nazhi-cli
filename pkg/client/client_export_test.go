package client

import "net/http"

// TransportForTest 返回 Client 使用的 *http.Transport，仅测试用。
func TransportForTest(c *Client) *http.Transport {
	if c == nil || c.http == nil {
		return nil
	}
	tr, ok := c.http.Transport.(*http.Transport)
	if !ok {
		return nil
	}
	return tr
}

// HTTPClientForTest 返回 Client 使用的 *http.Client，仅测试用。
func HTTPClientForTest(c *Client) *http.Client {
	if c == nil {
		return nil
	}
	return c.http
}
