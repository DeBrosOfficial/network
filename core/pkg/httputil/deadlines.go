package httputil

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// TransferBudget is how long a long-running request (a whole-database
// transfer, a storage upload or pin, a function deploy or invocation) may take
// to be read and written, on every gateway it passes through. The gateways'
// http.Server read and write timeouts (60s and 120s) would otherwise cut a
// database of a few hundred MiB, or an upload from a slow client, off
// mid-transfer whatever any proxy allowed it.
const TransferBudget = 5 * time.Minute

// ExtendIO moves this request's read and write deadlines to budget from now,
// replacing the server's own. It is for the routes that move a whole database;
// every other route keeps the server's timeouts.
//
// A ResponseWriter that cannot move its deadlines (a test recorder) is not an
// error: it has none to move. Any other failure is returned, because carrying
// on would cut the transfer off at the server's timeout.
func ExtendIO(w http.ResponseWriter, budget time.Duration) error {
	rc := http.NewResponseController(w)
	deadline := time.Now().Add(budget)
	if err := rc.SetReadDeadline(deadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return fmt.Errorf("extend the read deadline: %w", err)
	}
	if err := rc.SetWriteDeadline(deadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return fmt.Errorf("extend the write deadline: %w", err)
	}
	return nil
}

// StreamWithIdleWrites copies src to w, moving the write deadline to idle
// from the start of each write. It replaces the server's whole-response write
// timeout (and any budget ExtendIO set) with one that tracks progress, for a
// response whose size and duration are the client's and the content's, such as
// a stored object: a body that keeps moving is never cut off, and a client that
// stops reading still is, idle after the write it stalled. How long src may
// take to produce the next bytes is src's to bound.
//
// A ResponseWriter that cannot move its deadline (a test recorder) is copied
// to without one.
func StreamWithIdleWrites(w http.ResponseWriter, src io.Reader, idle time.Duration) (int64, error) {
	rc := http.NewResponseController(w)
	buf := make([]byte, streamChunkBytes)
	var written int64
	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			if err := rc.SetWriteDeadline(time.Now().Add(idle)); err != nil && !errors.Is(err, http.ErrNotSupported) {
				return written, fmt.Errorf("move the write deadline: %w", err)
			}
			m, err := w.Write(buf[:n])
			written += int64(m)
			if err != nil {
				return written, err
			}
		}
		if readErr == io.EOF {
			return written, nil
		}
		if readErr != nil {
			return written, readErr
		}
	}
}

// streamChunkBytes is StreamWithIdleWrites's copy buffer, io.Copy's size.
const streamChunkBytes = 32 << 10
