package mikrotik

import (
	"context"
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

func TestCloseIdempotent(t *testing.T) {
	c := NewClient("127.0.0.1:1", "u", "p", time.Second)
	c.Close()
	c.Close()
}
