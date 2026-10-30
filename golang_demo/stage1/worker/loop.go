// Package worker runs a loop that can be stopped with context cancellation.
package worker

import "context"

// Loop 从 in 读取并转发到 out，直到 ctx 被取消或 in 被关闭。
// 返回前会关闭 out。
func Loop(ctx context.Context, in <-chan int, out chan<- int) {
	defer close(out)
	// select 每轮只执行一个已经就绪的 case，不会把每个 case 都跑一遍。
	// 多个 case 同时就绪时随机挑一个。一个都没就绪就停在这里等。
	for {
		select {
		case <-ctx.Done():
			return
		case v, ok := <-in:
			if !ok {
				return
			}
			// 发送可能堵住。再 select 一次：取消和发送里只走先就绪的那个。
			select {
			case <-ctx.Done():
				return
			case out <- v:
			}
		}
	}
}
