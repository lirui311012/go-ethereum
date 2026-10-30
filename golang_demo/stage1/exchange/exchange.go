// Package exchange 在一条 TCP 连接上发送一条 JSON，再读回一条回复。
package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"stage1/cpuwork"
	"stage1/envelope"
)

// 单次读写的最长时间。到点后读或写返回错误。
const ioTimeout = 10 * time.Second

// reportConnErr 报告一条连接的失败。测试可以替换它。
var reportConnErr = func(err error) {
	fmt.Fprintf(os.Stderr, "connection: %v\n", err)
}

// Serve 一直接受连接，直到 ctx 被取消或监听失败。
// 单条连接的读失败、校验失败、写失败只结束这一条：错误打印到 stderr，然后继续 Accept。
// 取消 ctx 会关掉监听器，Serve 返回 nil。
func Serve(ctx context.Context, ln net.Listener, check envelope.Checker) error {
	defer ln.Close()
	// context.Background 的 Done 是 nil，不能接收。只有可取消的 ctx 才走这条退出路径。
	if ctx.Done() != nil {
		go func() {
			<-ctx.Done()
			ln.Close() // 让正堵在 Accept 上的调用返回
		}()
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			// 监听器是被取消关掉的，这是正常结束。
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("accept: %w", err)
		}
		// 一条连接的失败不代表监听器坏了。打印后继续接受下一条。
		if err := Handle(conn, check); err != nil {
			reportConnErr(err)
			continue
		}
	}
}

// Handle 读一条消息，用 check 核对，通过后再写一条回复。
// 返回前关闭连接。
func Handle(conn net.Conn, check envelope.Checker) error {
	defer conn.Close()
	// 截止时间一到，后面的读和写都会带着超时错误返回。
	if err := conn.SetDeadline(time.Now().Add(ioTimeout)); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}
	req, err := readMessage(conn)
	if err != nil {
		return fmt.Errorf("read request: %w", err)
	}
	// 先核对再回复。校验失败不写回复，错误向上返回。
	if err := check.Check(req); err != nil {
		return fmt.Errorf("check request: %w", err)
	}
	// Sum 是正文的指纹。客户端会重算并比对。
	resp := envelope.Message{
		From: "server",
		Body: req.Body,
		Sum:  fmt.Sprintf("%x", cpuwork.Mix(req.Body)),
	}
	if err := writeMessage(conn, resp); err != nil {
		return fmt.Errorf("write reply: %w", err)
	}
	return nil
}

// Exchange 拨号到 addr，写下 msg，返回对端的回复。
func Exchange(ctx context.Context, addr string, msg envelope.Message) (envelope.Message, error) {
	if err := msg.Validate(); err != nil {
		return envelope.Message{}, err
	}
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return envelope.Message{}, fmt.Errorf("dial %s: %w", addr, err)
	}
	defer conn.Close()

	deadline := time.Now().Add(ioTimeout)
	// ctx 自己带了更早的截止时间时，用那个更早的。
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return envelope.Message{}, fmt.Errorf("set deadline: %w", err)
	}
	if err := writeMessage(conn, msg); err != nil {
		return envelope.Message{}, fmt.Errorf("write request: %w", err)
	}
	resp, err := readMessage(conn)
	if err != nil {
		return envelope.Message{}, fmt.Errorf("read reply: %w", err)
	}
	return resp, nil
}

// CheckReply 在回复不是由 req 产生时返回错误。
func CheckReply(req, resp envelope.Message) error {
	if err := resp.Validate(); err != nil {
		return fmt.Errorf("check reply: %w", err)
	}
	if resp.From != "server" {
		return fmt.Errorf("reply from %q: %w", resp.From, envelope.ErrBadPeer)
	}
	if resp.Body != req.Body {
		return fmt.Errorf("reply body %q, want %q", resp.Body, req.Body)
	}
	// 客户端自己再算一遍，和回复里的 Sum 比对。
	want := fmt.Sprintf("%x", cpuwork.Mix(req.Body))
	if resp.Sum != want {
		return fmt.Errorf("reply sum %q, want %q", resp.Sum, want)
	}
	return nil
}

// readMessage 从 r 解码一条 JSON。对端中途断开时，错误里会带上 io.EOF。
func readMessage(r io.Reader) (envelope.Message, error) {
	var msg envelope.Message
	if err := json.NewDecoder(r).Decode(&msg); err != nil {
		return envelope.Message{}, fmt.Errorf("decode message: %w", err)
	}
	return msg, nil
}

// writeMessage 把 msg 编码成 JSON 写到 w。Encode 会在末尾加换行。
func writeMessage(w io.Writer, msg envelope.Message) error {
	if err := json.NewEncoder(w).Encode(msg); err != nil {
		return fmt.Errorf("encode message: %w", err)
	}
	return nil
}
