package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func resetTraceCacheForTest() {
	traceCacheMu.Lock()
	traceCache = traceCacheState{}
	traceCacheMu.Unlock()
}

func TestTraceSnapshotRefreshesAsynchronouslyAndCaches(t *testing.T) {
	oldFetcher := traceFetcher
	resetTraceCacheForTest()
	defer func() {
		traceFetcher = oldFetcher
		resetTraceCacheForTest()
	}()

	var calls int32
	traceFetcher = func(context.Context, string) (string, error) {
		atomic.AddInt32(&calls, 1)
		return "warp=on\ncolo=SEA\nip=104.28.1.2\n", nil
	}

	first := traceSnapshot("warp0", 123)
	if !first.Pending || len(first.Fields) != 0 {
		t.Fatalf("first snapshot should return pending without blocking: %+v", first)
	}

	deadline := time.Now().Add(time.Second)
	var got TraceSnapshot
	for time.Now().Before(deadline) {
		got = traceSnapshot("warp0", 123)
		if !got.Pending && got.Fields["warp"] == "on" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got.Fields["warp"] != "on" || got.Fields["colo"] != "SEA" || got.VerifiedAt == 0 {
		t.Fatalf("trace result was not cached: %+v", got)
	}
	if count := atomic.LoadInt32(&calls); count != 1 {
		t.Fatalf("expected one probe inside the cache interval, got %d", count)
	}
}
