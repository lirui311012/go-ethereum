// Command peer is either the TCP server or the TCP client.
//
//	go run ./cmd/peer -mode server -pprof 127.0.0.1:6060
//	go run ./cmd/peer -mode client
//	go run ./cmd/peer -mode client -drop
//
// 客户端这次拨号、读取或核对失败时，把错误打到 stderr 并以状态码 1 退出。
// 服务端单条连接失败只结束这一条，进程继续 Accept。
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"
	"time"

	"stage1/envelope"
	"stage1/exchange"
)

func main() {
	// mode：server 接受连接并回复，client 发出 JSON 并核对回复。
	mode := flag.String("mode", "", "server or client")
	// addr：两边共用的 TCP 地址。
	addr := flag.String("addr", "127.0.0.1:19091", "tcp address")
	// pprofAddr：服务端的 pprof 监听地址。空字符串表示不开启。
	pprofAddr := flag.String("pprof", "", "server: pprof listen address, for example 127.0.0.1:6060")
	// from：客户端消息里的发送方，服务端用它决定是否接受。
	from := flag.String("from", "alice", "client: message from")
	// body：客户端消息正文。服务端对它计算 Sum。
	body := flag.String("body", "hello", "client: message body")
	// peers：服务端允许的发送方，用逗号分隔。
	peers := flag.String("peers", "alice", "server: comma-separated allowed From values")
	// n：客户端连续交换的次数。压测时加大。
	n := flag.Int("n", 1, "client: number of exchanges")
	// drop：客户端连上后立刻关闭，用来看服务端是否打印错误并继续接受连接。
	drop := flag.Bool("drop", false, "client: connect and close without sending")
	flag.Parse()

	if err := run(*mode, *addr, *pprofAddr, *from, *body, *peers, *n, *drop); err != nil {
		name := *mode
		if name == "" {
			name = "peer"
		}
		fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
		os.Exit(1)
	}
}

func run(mode, addr, pprofAddr, from, body, peers string, n int, drop bool) error {
	switch mode {
	case "server":
		return runServer(addr, pprofAddr, peers)
	case "client":
		return runClient(addr, from, body, n, drop)
	default:
		return fmt.Errorf("usage: peer -mode server|client")
	}
}

func runServer(addr, pprofAddr, peers string) error {
	if pprofAddr != "" {
		if err := listenPprof(pprofAddr); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "pprof http://%s/debug/pprof/\n", pprofAddr)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	fmt.Fprintf(os.Stderr, "listening on %s\n", ln.Addr())
	return exchange.Serve(context.Background(), ln, envelope.NewAllowList(splitComma(peers)...))
}

func runClient(addr, from, body string, n int, drop bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(n)*ioBudget())
	defer cancel()
	if drop {
		return dropConn(ctx, addr)
	}
	if n < 1 {
		return fmt.Errorf("n must be >= 1")
	}
	req := envelope.Message{From: from, Body: body}
	var last envelope.Message
	for i := 0; i < n; i++ {
		resp, err := exchange.Exchange(ctx, addr, req)
		if err != nil {
			return err
		}
		if err := exchange.CheckReply(req, resp); err != nil {
			return err
		}
		last = resp
	}
	if n == 1 {
		fmt.Printf("%s %s %s\n", last.From, last.Body, last.Sum)
	} else {
		fmt.Printf("exchanged %d\n", n)
	}
	return nil
}

func dropConn(ctx context.Context, addr string) error {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}
	if err := conn.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}
	return nil
}

func listenPprof(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("pprof listen %s: %w", addr, err)
	}
	go func() {
		err := http.Serve(ln, nil)
		fmt.Fprintf(os.Stderr, "pprof: %v\n", err)
		os.Exit(1)
	}()
	return nil
}

func splitComma(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// ioBudget leaves room for Mix plus one round trip per message.
func ioBudget() time.Duration {
	return 15 * time.Second
}
