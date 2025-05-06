// Copyright 2020 Google Inc. All Rights Reserved.
// This file is available under the Apache license.

//go:build unix

package logstream_test

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/mtail/internal/logline"
	"github.com/google/mtail/internal/tailer/logstream"
	"github.com/google/mtail/internal/testutil"
	"github.com/google/mtail/internal/waker"
)

const dgramTimeout = 1 * time.Second

func TestDgramStreamReadCompletedBecauseSocketClosed(t *testing.T) {
	for _, scheme := range []string{"unixgram", "udp"} {
		scheme := scheme
		t.Run(scheme, testutil.TimeoutTest(dgramTimeout, func(t *testing.T) { //nolint:thelper
			var wg sync.WaitGroup

			var addr string
			switch scheme {
			case "unixgram":
				tmpDir := testutil.TestTempDir(t)
				addr = filepath.Join(tmpDir, "sock")
			case "udp":
				addr = fmt.Sprintf("[::]:%d", testutil.FreePort(t))
			default:
				t.Fatalf("bad scheme %s", scheme)
			}

			ctx, cancel := context.WithCancel(context.Background())
			// Stream is not shut down with cancel in this test
			defer cancel()
			waker := waker.NewTestAlways()

			sockName := scheme + "://" + addr
			ds, err := logstream.New(ctx, &wg, waker, sockName, logstream.OneShotEnabled)
			testutil.FatalIfErr(t, err)

			expected := []*logline.LogLine{
				{Context: context.TODO(), Filename: sockName, Line: "1"},
			}
			checkLineDiff := testutil.ExpectLinesReceivedNoDiff(t, expected, ds.Lines())

			s, err := net.Dial(scheme, addr)
			testutil.FatalIfErr(t, err)

			_, err = s.Write([]byte("1\n"))
			testutil.FatalIfErr(t, err)

			// "Close" the socket by sending zero bytes, which in oneshot mode tells the stream to act as if we're done.
			_, err = s.Write([]byte{})
			testutil.FatalIfErr(t, err)

			wg.Wait()

			checkLineDiff()

			if v := <-ds.Lines(); v != nil {
				t.Errorf("expecting dgramstream to be complete because socket closed")
			}
		}))
	}
}

func TestDgramStreamReadCompletedBecauseCancel(t *testing.T) {
	for _, scheme := range []string{"unixgram", "udp"} {
		scheme := scheme
		t.Run(scheme, testutil.TimeoutTest(dgramTimeout, func(t *testing.T) { //nolint:thelper
			var wg sync.WaitGroup

			var addr string
			switch scheme {
			case "unixgram":
				tmpDir := testutil.TestTempDir(t)
				addr = filepath.Join(tmpDir, "sock")
			case "udp":
				addr = fmt.Sprintf("[::]:%d", testutil.FreePort(t))
			default:
				t.Fatalf("bad scheme %s", scheme)
			}

			ctx, cancel := context.WithCancel(context.Background())
			waker := waker.NewTestAlways()

			sockName := scheme + "://" + addr
			ds, err := logstream.New(ctx, &wg, waker, sockName, logstream.OneShotDisabled)
			testutil.FatalIfErr(t, err)

			expected := []*logline.LogLine{
				{Context: context.TODO(), Filename: sockName, Line: "1"},
			}
			checkLineDiff := testutil.ExpectLinesReceivedNoDiff(t, expected, ds.Lines())

			s, err := net.Dial(scheme, addr)
			testutil.FatalIfErr(t, err)

			_, err = s.Write([]byte("1\n"))
			testutil.FatalIfErr(t, err)

			// Yield to give the stream a chance to read.
			time.Sleep(10 * time.Millisecond)

			cancel() // This cancellation should cause the stream to shut down.
			wg.Wait()

			checkLineDiff()

			if v := <-ds.Lines(); v != nil {
				t.Errorf("expecting dgramstream to be complete because cancel")
			}
		}))
	}
}
