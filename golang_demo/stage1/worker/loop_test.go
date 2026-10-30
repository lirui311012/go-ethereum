package worker

import (
	"context"
	"testing"
	"time"
)

// TestLoopExitsOnCancel 先确认循环会把值从 in 转到 out，
// 再取消 ctx，确认 Loop 返回，并且返回后不再从 in 接收。
func TestLoopExitsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	in := make(chan int)
	out := make(chan int)
	exited := make(chan struct{})
	go func() {
		Loop(ctx, in, out)
		close(exited)
	}()

	in <- 1
	if got := <-out; got != 1 {
		t.Fatalf("forwarded %d, want 1", got)
	}
	cancel()

	select {
	case <-exited:
	case <-time.After(time.Second):
		t.Fatal("Loop did not return after cancel")
	}

	// 此时没有接收方。发送能完成，说明 Loop 还在运行。
	select {
	case in <- 2:
		t.Fatal("Loop accepted a value after it returned")
	case <-time.After(50 * time.Millisecond):
	}
}

// TestLoopExitsWhenInputClosed 关闭 in 后，Loop 应返回，并在返回前关闭 out。
func TestLoopExitsWhenInputClosed(t *testing.T) {
	in := make(chan int)
	out := make(chan int)
	exited := make(chan struct{})
	go func() {
		Loop(context.Background(), in, out)
		close(exited)
	}()
	close(in)

	select {
	case <-exited:
	case <-time.After(time.Second):
		t.Fatal("Loop did not return after input was closed")
	}
	_, ok := <-out
	if ok {
		t.Fatal("out was not closed")
	}
}
