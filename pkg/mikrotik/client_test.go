package mikrotik

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"
)

func TestConnectionLimiterBlocksUntilRelease(t *testing.T) {
	SetMaxConcurrent(1)
	defer SetMaxConcurrent(DefaultMaxConcurrent)

	ctx := context.Background()
	if err := acquireConn(ctx); err != nil {
		t.Fatal(err)
	}

	blocked := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		cctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		err := acquireConn(cctx)
		if err == nil {
			releaseConn()
			close(blocked)
		}
	}()
	wg.Wait()

	select {
	case <-blocked:
		t.Fatal("second acquire should have timed out while slot held")
	default:
	}

	releaseConn()
	if err := acquireConn(ctx); err != nil {
		t.Fatal(err)
	}
	releaseConn()
}

func TestBeginScrapeDeadline(t *testing.T) {
	c := NewClient("127.0.0.1:1", "u", "p", 20*time.Millisecond)
	ctx, cancel := c.BeginScrape(context.Background())
	defer cancel()
	defer c.Close()

	select {
	case <-ctx.Done():
		// ok
	case <-time.After(200 * time.Millisecond):
		t.Fatal("scrape context did not expire")
	}
}

func TestBeginScrapeCancelledParent(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	cancel()
	c := NewClient("192.0.2.1:8728", "u", "p", 30*time.Second)
	ctx, stop := c.BeginScrape(parent)
	defer stop()
	defer c.Close()
	select {
	case <-ctx.Done():
	case <-time.After(200 * time.Millisecond):
		t.Fatal("child context still active")
	}
}

func TestAcquireConnAlreadyCancelled(t *testing.T) {
	// Shares the process-global limiter with other tests in this package.
	// Do not call t.Parallel. Other test packages are separate processes.
	SetMaxConcurrent(1)
	defer SetMaxConcurrent(DefaultMaxConcurrent)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for i := 0; i < 30; i++ {
		if err := acquireConn(ctx); err == nil {
			releaseConn()
			t.Fatalf("cancelled context acquired a slot on try %d", i)
		}
	}
}

func TestConnectContextStalledPeerRespectsDeadline(t *testing.T) {
	SetMaxConcurrent(1)
	defer SetMaxConcurrent(DefaultMaxConcurrent)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		accepted <- conn
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	c := NewClient(ln.Addr().String(), "u", "p", 30*time.Second)
	done := make(chan error, 1)
	go func() {
		done <- c.ConnectContext(ctx)
	}()
	var connectErr error
	select {
	case connectErr = <-done:
	case <-time.After(time.Second):
		cancel()
		_ = ln.Close()
		select {
		case conn := <-accepted:
			_ = conn.Close()
		default:
		}
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
		c.Close()
		t.Fatal("stalled peer held the scrape for more than 1s")
	}
	c.Close()
	if connectErr == nil {
		t.Fatal("expected connect to fail")
	}

	select {
	case conn := <-accepted:
		_ = conn.Close()
	default:
	}

	slotCtx, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if err := acquireConn(slotCtx); err != nil {
		t.Fatalf("slot still held: %v", err)
	}
	releaseConn()
}

func TestConnectContextCancelledReturns(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	cancel()
	c := NewClient("192.0.2.1:8728", "u", "p", 30*time.Second)
	err := c.ConnectContext(parent)
	c.Close()
	if err == nil {
		t.Fatal("expected connect to fail")
	}
}

func TestCloseIdempotent(t *testing.T) {
	c := NewClient("127.0.0.1:1", "u", "p", time.Second)
	c.Close()
	c.Close()
}

func TestPreferContextErr(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := preferContextErr(ctx, errors.New("connection reset"))
	if !errors.Is(got, context.Canceled) {
		t.Fatalf("got=%v", got)
	}
	live := preferContextErr(context.Background(), errors.New("boom"))
	if live.Error() != "boom" {
		t.Fatalf("got=%v", live)
	}
	if preferContextErr(context.Background(), nil) != nil {
		t.Fatal("nil should stay nil")
	}
}
