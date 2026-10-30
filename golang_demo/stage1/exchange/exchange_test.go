package exchange

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"stage1/envelope"
)

func TestExchangeRoundTrip(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ln := listen(t)
	errc := make(chan error, 1)
	go func() {
		errc <- Serve(ctx, ln, envelope.NewAllowList("alice"))
	}()

	req := envelope.Message{From: "alice", Body: "ping"}
	callCtx, callCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer callCancel()
	resp, err := Exchange(callCtx, ln.Addr().String(), req)
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if err := CheckReply(req, resp); err != nil {
		t.Fatalf("CheckReply: %v", err)
	}

	cancel()
	if err := <-errc; err != nil {
		t.Fatalf("Serve after cancel: %v", err)
	}
}

func TestExchangeRejectsEmptyMessage(t *testing.T) {
	_, err := Exchange(context.Background(), "127.0.0.1:1", envelope.Message{})
	if !errors.Is(err, envelope.ErrEmptyFrom) {
		t.Fatalf("Exchange error = %v, want %v", err, envelope.ErrEmptyFrom)
	}
}

func TestServerContinuesAfterDisconnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ln := listen(t)
	errc := make(chan error, 1)
	connErrc := make(chan error, 1)
	prev := reportConnErr
	reportConnErr = func(err error) { connErrc <- err }
	t.Cleanup(func() { reportConnErr = prev })
	go func() {
		errc <- Serve(ctx, ln, envelope.NewAllowList("alice"))
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-connErrc:
		if !errors.Is(err, io.EOF) {
			t.Fatalf("connection error = %v, want io.EOF", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("disconnect was not reported")
	}

	req := envelope.Message{From: "alice", Body: "ping"}
	callCtx, callCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer callCancel()
	resp, err := Exchange(callCtx, ln.Addr().String(), req)
	if err != nil {
		t.Fatalf("Exchange after disconnect: %v", err)
	}
	if err := CheckReply(req, resp); err != nil {
		t.Fatal(err)
	}

	cancel()
	if err := <-errc; err != nil {
		t.Fatalf("Serve after cancel: %v", err)
	}
}

func TestServerRejectsUnknownPeer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ln := listen(t)
	errc := make(chan error, 1)
	connErrc := make(chan error, 1)
	prev := reportConnErr
	reportConnErr = func(err error) { connErrc <- err }
	t.Cleanup(func() { reportConnErr = prev })
	go func() {
		errc <- Serve(ctx, ln, envelope.NewAllowList("alice"))
	}()

	callCtx, callCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer callCancel()
	_, err := Exchange(callCtx, ln.Addr().String(), envelope.Message{From: "eve", Body: "ping"})
	if err == nil {
		t.Fatal("Exchange succeeded for a rejected peer")
	}
	select {
	case err := <-connErrc:
		if !errors.Is(err, envelope.ErrBadPeer) {
			t.Fatalf("connection error = %v, want %v", err, envelope.ErrBadPeer)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("rejected peer was not reported")
	}

	req := envelope.Message{From: "alice", Body: "ping"}
	resp, err := Exchange(callCtx, ln.Addr().String(), req)
	if err != nil {
		t.Fatalf("Exchange after reject: %v", err)
	}
	if err := CheckReply(req, resp); err != nil {
		t.Fatal(err)
	}

	cancel()
	if err := <-errc; err != nil {
		t.Fatalf("Serve after cancel: %v", err)
	}
}

func listen(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return ln
}
