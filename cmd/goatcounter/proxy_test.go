package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"zgo.at/goatcounter/v2/handlers"
	"zgo.at/json"
	"zgo.at/zli"
)

// upstream is a minimal GoatCounter server that only implements the two
// endpoints the proxy talks to: /api/v0/me (for checkSite) and /api/v0/count.
// The reply to /api/v0/count can be changed while the test runs, to act like a
// server that is down for a while.
type upstream struct {
	*httptest.Server

	mu       sync.Mutex
	got      []handlers.APICountRequestHit
	posts    int
	status   int    // Reply with this instead of accepting
	resetHdr string // X-Rate-Limit-Reset to send along with the status
}

func newUpstream(t *testing.T) *upstream {
	t.Helper()

	u := new(upstream)
	u.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v0/me":
			// permissions=2 is APIPermCount.
			fmt.Fprint(w, `{"token":{"name":"test","permissions":2}}`)
		case "/api/v0/count":
			var req handlers.APICountRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("decode count request: %s", err)
				w.WriteHeader(400)
				return
			}

			u.mu.Lock()
			defer u.mu.Unlock()
			u.posts++
			if u.status != 0 {
				if u.resetHdr != "" {
					w.Header().Set("X-Rate-Limit-Reset", u.resetHdr)
				}
				w.WriteHeader(u.status)
				fmt.Fprint(w, `{"error":"nope"}`)
				return
			}
			u.got = append(u.got, req.Hits...)
			w.WriteHeader(http.StatusAccepted)
		default:
			t.Errorf("unexpected request to %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(u.Close)
	return u
}

// reply makes /api/v0/count answer with this status. 0 accepts pageviews again.
func (u *upstream) reply(status int, reset string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.status, u.resetHdr = status, reset
}

func (u *upstream) hits() []handlers.APICountRequestHit {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]handlers.APICountRequestHit(nil), u.got...)
}

func (u *upstream) numPosts() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.posts
}

// waitFor polls until f returns true, or fails the test after a few seconds.
func waitFor(t *testing.T, what string, f func() bool) {
	t.Helper()
	for range 200 {
		if f() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}

// startProxy runs the proxy command against an upstream and returns a function
// to shut it down.
func startProxy(t *testing.T, listen string, args ...string) func() {
	t.Helper()

	exit, _, _ := zli.Test(t)
	ready := make(chan struct{}, 1)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		runCmdStop(t, exit, ready, stop, "proxy", append([]string{"-listen=" + listen}, args...)...)
		close(done)
	}()
	<-ready

	var once sync.Once
	shutdown := func() {
		once.Do(func() {
			close(stop)
			<-done
		})
	}
	t.Cleanup(shutdown)
	return shutdown
}

func sendCount(t *testing.T, listen, query string) *http.Response {
	t.Helper()
	r, err := http.NewRequest("GET", "http://"+listen+"/count?"+query, nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) Firefox/130.0")
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestProxy(t *testing.T) {
	up := newUpstream(t)
	t.Setenv("GOATCOUNTER_API_KEY", "test-key")

	const listen = "localhost:9923"
	shutdown := startProxy(t, listen,
		"-site="+up.URL,
		"-ratelimit=off",
		"-batch-min-time=50ms",
		"-batch-max-time=500ms")

	for i := range 3 {
		resp := sendCount(t, listen, "p=/page"+strconv.Itoa(i)+"&t=Title"+strconv.Itoa(i))
		if resp.StatusCode != 200 {
			t.Fatalf("/count returned %d", resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "image/gif" {
			t.Errorf("Content-Type = %q, want image/gif", ct)
		}
		resp.Body.Close()
	}

	waitFor(t, "3 forwarded pageviews", func() bool { return len(up.hits()) >= 3 })
	shutdown()

	got := up.hits()
	if len(got) != 3 {
		t.Fatalf("forwarded %d pageviews, want 3", len(got))
	}
	for i, h := range got {
		if want := "/page" + strconv.Itoa(i); h.Path != want {
			t.Errorf("pageview %d: Path = %q, want %q", i, h.Path, want)
		}
		if want := "Title" + strconv.Itoa(i); h.Title != want {
			t.Errorf("pageview %d: Title = %q, want %q", i, h.Title, want)
		}
		if h.UserAgent == "" {
			t.Errorf("pageview %d: UserAgent is empty", i)
		}
		if h.IP == "" {
			t.Errorf("pageview %d: IP is empty", i)
		}
		if h.CreatedAt.IsZero() {
			t.Errorf("pageview %d: CreatedAt is zero", i)
		}
	}
}

func TestProxyRatelimit(t *testing.T) {
	up := newUpstream(t)
	t.Setenv("GOATCOUNTER_API_KEY", "test-key")

	const listen = "localhost:9924"
	shutdown := startProxy(t, listen,
		"-site="+up.URL,
		"-ratelimit=2/1",
		"-batch-min-time=50ms",
		"-batch-max-time=500ms")

	var ok, limited int
	for range 10 {
		resp := sendCount(t, listen, "p=/x")
		switch resp.StatusCode {
		case 200:
			ok++
		case http.StatusTooManyRequests:
			limited++
		default:
			t.Errorf("unexpected status %d", resp.StatusCode)
		}
		resp.Body.Close()
	}

	if limited == 0 {
		t.Errorf("expected some requests to be rate limited, got %d ok / %d limited", ok, limited)
	}
	// Only the non-limited requests should have been forwarded.
	waitFor(t, "the non-limited pageviews", func() bool { return len(up.hits()) >= ok })
	shutdown()

	if n := len(up.hits()); n != ok {
		t.Errorf("forwarded %d pageviews, want %d (the non-limited ones)", n, ok)
	}
}

// A batch is kept and retried while the upstream server is unavailable, and
// forwarded once it comes back.
func TestProxyUpstreamOutage(t *testing.T) {
	up := newUpstream(t)
	t.Setenv("GOATCOUNTER_API_KEY", "test-key")

	// What a proxy in front of a dead GoatCounter server would say.
	up.reply(http.StatusServiceUnavailable, "")

	const listen = "localhost:9925"
	shutdown := startProxy(t, listen,
		"-site="+up.URL,
		"-ratelimit=off",
		"-batch-min-time=50ms",
		"-batch-max-time=200ms")

	resp := sendCount(t, listen, "p=/down")
	resp.Body.Close()

	// The batch is tried, fails, and is kept for later.
	waitFor(t, "the first attempt", func() bool { return up.numPosts() >= 1 })
	if n := len(up.hits()); n != 0 {
		t.Fatalf("forwarded %d pageviews while the server was down, want 0", n)
	}

	up.reply(0, "") // Server is back.
	waitFor(t, "the pageview after recovery", func() bool { return len(up.hits()) == 1 })

	shutdown()
	if got := up.hits(); got[0].Path != "/down" {
		t.Errorf("Path = %q, want /down", got[0].Path)
	}
}

// While the forwarding goroutine is stuck retrying, /count keeps answering and
// the queue keeps the most recent pageviews rather than the first ones.
func TestProxyKeepsServingDuringOutage(t *testing.T) {
	up := newUpstream(t)
	t.Setenv("GOATCOUNTER_API_KEY", "test-key")

	up.reply(http.StatusBadGateway, "")

	const (
		listen = "localhost:9928"
		queue  = 5
		total  = 40
	)
	shutdown := startProxy(t, listen,
		"-site="+up.URL,
		"-ratelimit=off",
		"-queue="+strconv.Itoa(queue),
		"-batch=2",
		"-batch-min-time=10ms",
		"-batch-max-time=50ms")

	// The upstream server is unreachable for all of this, so none of these may
	// wait on it.
	start := time.Now()
	for i := range total {
		resp := sendCount(t, listen, "p=/p"+strconv.Itoa(i))
		if resp.StatusCode != 200 {
			t.Fatalf("/count returned %d while the upstream server was down", resp.StatusCode)
		}
		resp.Body.Close()
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("%d requests took %s: /count waited for the upstream server", total, d)
	}
	waitFor(t, "the first attempt", func() bool { return up.numPosts() >= 1 })

	up.reply(0, "") // Server is back.
	// Wait until nothing new arrives for a while: the backlog is through.
	var n int
	for {
		time.Sleep(250 * time.Millisecond)
		if got := len(up.hits()); got == n {
			break
		} else {
			n = got
		}
	}
	shutdown()

	got := up.hits()
	if len(got) == 0 {
		t.Fatal("nothing was forwarded after the upstream server came back")
	}
	if len(got) >= total {
		t.Errorf("forwarded %d of %d pageviews; a queue of %d should have dropped some",
			len(got), total, queue)
	}

	// Whatever came through must be pageviews we sent, in order, and has to
	// include the last one: the oldest are dropped, never the newest.
	seen := make(map[string]bool, len(got))
	last := -1
	for _, h := range got {
		i, err := strconv.Atoi(strings.TrimPrefix(h.Path, "/p"))
		if err != nil || i >= total {
			t.Fatalf("forwarded a pageview we never sent: %q", h.Path)
		}
		if seen[h.Path] {
			t.Errorf("forwarded %q twice", h.Path)
		}
		seen[h.Path] = true
		if i < last {
			t.Errorf("pageview %q came after /p%d: batches are out of order", h.Path, last)
		}
		last = i
	}
	if want := "/p" + strconv.Itoa(total-1); !seen[want] {
		t.Errorf("%s was not forwarded; the newest pageviews should be the ones kept", want)
	}
}

// A 4xx means the batch itself is wrong, so it's dropped instead of retried.
func TestProxyFatalStatus(t *testing.T) {
	up := newUpstream(t)
	t.Setenv("GOATCOUNTER_API_KEY", "test-key")

	up.reply(http.StatusUnauthorized, "") // e.g. the API key was revoked.

	const listen = "localhost:9926"
	shutdown := startProxy(t, listen,
		"-site="+up.URL,
		"-ratelimit=off",
		"-batch-min-time=50ms",
		"-batch-max-time=200ms")

	resp := sendCount(t, listen, "p=/gone")
	resp.Body.Close()

	waitFor(t, "the only attempt", func() bool { return up.numPosts() >= 1 })

	// Well past the first retry wait of one second: it must not come back.
	time.Sleep(1500 * time.Millisecond)
	if n := up.numPosts(); n != 1 {
		t.Errorf("posted %d times, want 1: a 401 should not be retried", n)
	}
	shutdown()
}

// A 429 is retried after the delay the server asks for.
func TestProxyUpstreamRatelimited(t *testing.T) {
	up := newUpstream(t)
	t.Setenv("GOATCOUNTER_API_KEY", "test-key")

	up.reply(http.StatusTooManyRequests, "1")

	const listen = "localhost:9927"
	shutdown := startProxy(t, listen,
		"-site="+up.URL,
		"-ratelimit=off",
		"-batch-min-time=50ms",
		"-batch-max-time=200ms")

	resp := sendCount(t, listen, "p=/slow")
	resp.Body.Close()

	waitFor(t, "the rate limited attempt", func() bool { return up.numPosts() >= 1 })
	up.reply(0, "")
	waitFor(t, "the pageview after the rate limit", func() bool { return len(up.hits()) == 1 })
	shutdown()
}

func TestHitQueue(t *testing.T) {
	ctx := context.Background()
	push := func(q *hitQueue, paths ...string) {
		for _, p := range paths {
			q.push(ctx, handlers.APICountRequestHit{Path: p, CreatedAt: time.Now().UTC()})
		}
	}
	paths := func(hits []handlers.APICountRequestHit) []string {
		p := make([]string, len(hits))
		for i, h := range hits {
			p[i] = h.Path
		}
		return p
	}
	wantPop := func(t *testing.T, q *hitQueue, max int, want ...string) {
		t.Helper()
		got := paths(q.pop(max))
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("pop(%d) = %v, want %v", max, got, want)
		}
	}

	t.Run("fifo", func(t *testing.T) {
		q := newHitQueue(5)
		push(q, "/a", "/b", "/c")
		if q.n != 3 {
			t.Errorf("size = %d, want 3", q.n)
		}
		wantPop(t, q, 2, "/a", "/b")
		wantPop(t, q, 2, "/c")
		if q.n != 0 {
			t.Errorf("size = %d, want 0", q.n)
		}
		if got := q.pop(1); got != nil {
			t.Errorf("pop on empty queue = %v, want nil", got)
		}
	})

	t.Run("drops oldest when full", func(t *testing.T) {
		q := newHitQueue(3)
		push(q, "/a", "/b", "/c", "/d", "/e")
		if q.n != 3 {
			t.Errorf("size = %d, want 3", q.n)
		}
		if d := q.dropped; d != 2 {
			t.Errorf("numDropped = %d, want 2", d)
		}
		wantPop(t, q, 10, "/c", "/d", "/e")
	})

	t.Run("grows up to max", func(t *testing.T) {
		q := newHitQueue(5)
		if len(q.buf) != 5 {
			t.Errorf("small queue was allocated as %d, want 5", len(q.buf))
		}

		q = newHitQueue(3 * hitQueueMin)
		if len(q.buf) != hitQueueMin {
			t.Errorf("initial ring is %d, want %d", len(q.buf), hitQueueMin)
		}
		for i := range 3 * hitQueueMin {
			push(q, "/"+strconv.Itoa(i))
		}
		if len(q.buf) != 3*hitQueueMin {
			t.Errorf("ring grew to %d, want %d", len(q.buf), 3*hitQueueMin)
		}
		if q.n != 3*hitQueueMin {
			t.Errorf("size = %d, want %d", q.n, 3*hitQueueMin)
		}
		if d := q.dropped; d != 0 {
			t.Errorf("numDropped = %d, want 0: the queue should have grown instead", d)
		}
		// Everything is still there, in order, after two rounds of growing.
		got := q.pop(3 * hitQueueMin)
		for i, h := range got {
			if want := "/" + strconv.Itoa(i); h.Path != want {
				t.Fatalf("pageview %d: Path = %q, want %q", i, h.Path, want)
			}
		}
	})

	t.Run("shrinks back down as the backlog clears", func(t *testing.T) {
		q := newHitQueue(8 * hitQueueMin)
		for i := range 8 * hitQueueMin {
			push(q, "/"+strconv.Itoa(i))
		}
		if len(q.buf) != 8*hitQueueMin {
			t.Fatalf("ring grew to %d, want %d", len(q.buf), 8*hitQueueMin)
		}

		// Just over a quarter in use is not enough to halve it: that would
		// leave the ring completely full.
		q.pop(6*hitQueueMin - 1)
		if want := 8 * hitQueueMin; len(q.buf) != want {
			t.Errorf("ring is %d with %d of %d in use, want %d", len(q.buf), q.n, want, want)
		}

		// At the quarter mark it halves once, and the result is half empty.
		q.pop(1)
		if want := 4 * hitQueueMin; len(q.buf) != want {
			t.Errorf("ring is %d, want %d", len(q.buf), want)
		}

		// Emptying it goes all the way back down, but no further.
		q.pop(8 * hitQueueMin)
		if q.n != 0 {
			t.Fatalf("size = %d, want 0", q.n)
		}
		if len(q.buf) != hitQueueMin {
			t.Errorf("empty ring is %d, want %d", len(q.buf), hitQueueMin)
		}

		// The pageviews left over survive the shrinking, in order.
		q = newHitQueue(4 * hitQueueMin)
		for i := range 4 * hitQueueMin {
			push(q, "/"+strconv.Itoa(i))
		}
		q.pop(4*hitQueueMin - 3)
		if len(q.buf) != hitQueueMin {
			t.Errorf("ring is %d, want %d", len(q.buf), hitQueueMin)
		}
		wantPop(t, q, 10,
			"/"+strconv.Itoa(4*hitQueueMin-3),
			"/"+strconv.Itoa(4*hitQueueMin-2),
			"/"+strconv.Itoa(4*hitQueueMin-1))
	})

	t.Run("never shrinks below its own max", func(t *testing.T) {
		// A queue smaller than hitQueueMin starts at its max and stays there.
		q := newHitQueue(4)
		push(q, "/a", "/b", "/c", "/d")
		q.pop(4)
		if len(q.buf) != 4 {
			t.Errorf("ring is %d, want 4", len(q.buf))
		}
	})

	t.Run("grows from a wrapped ring", func(t *testing.T) {
		q := newHitQueue(8)
		q.buf = make([]handlers.APICountRequestHit, 2) // Pretend it hasn't grown yet.
		push(q, "/a", "/b")
		wantPop(t, q, 1, "/a")
		push(q, "/c", "/d") // Wraps, then grows while wrapped.
		wantPop(t, q, 10, "/b", "/c", "/d")
	})

	t.Run("wraps around", func(t *testing.T) {
		q := newHitQueue(3)
		push(q, "/a", "/b")
		wantPop(t, q, 2, "/a", "/b")
		push(q, "/c", "/d", "/e") // Writes across the end of the ring.
		wantPop(t, q, 10, "/c", "/d", "/e")
		if d := q.dropped; d != 0 {
			t.Errorf("numDropped = %d, want 0", d)
		}
	})

	t.Run("oldest", func(t *testing.T) {
		q := newHitQueue(3)
		if _, ok := q.oldest(); ok {
			t.Error("oldest on an empty queue returned ok")
		}
		first := time.Now().UTC()
		q.push(ctx, handlers.APICountRequestHit{Path: "/a", CreatedAt: first})
		q.push(ctx, handlers.APICountRequestHit{Path: "/b", CreatedAt: first.Add(time.Second)})
		got, ok := q.oldest()
		if !ok || !got.Equal(first) {
			t.Errorf("oldest = %s, %t; want %s, true", got, ok, first)
		}
	})

	t.Run("pop frees the strings", func(t *testing.T) {
		q := newHitQueue(2)
		push(q, "/a", "/b")
		q.pop(2)
		for i, h := range q.buf {
			if h.Path != "" {
				t.Errorf("slot %d still holds %q after pop", i, h.Path)
			}
		}
	})
}

func TestRetryableStatus(t *testing.T) {
	tests := []struct {
		code int
		want bool
	}{
		{http.StatusBadRequest, false},
		{http.StatusUnauthorized, false},
		{http.StatusForbidden, false},
		{http.StatusNotFound, false},
		{http.StatusRequestEntityTooLarge, false},
		{http.StatusUnsupportedMediaType, false},
		{http.StatusRequestTimeout, true},
		{http.StatusTooEarly, true},
		{http.StatusInternalServerError, true},
		{http.StatusBadGateway, true},
		{http.StatusServiceUnavailable, true},
		{http.StatusGatewayTimeout, true},
	}
	for _, tt := range tests {
		if got := retryableStatus(tt.code); got != tt.want {
			t.Errorf("retryableStatus(%d) = %t, want %t", tt.code, got, tt.want)
		}
	}
}

func TestRetryWait(t *testing.T) {
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second}
	for i, w := range want {
		if got := retryWait(i); got != w {
			t.Errorf("retryWait(%d) = %s, want %s", i, got, w)
		}
	}
	for _, n := range []int{6, 10, 100, 1000} {
		if got := retryWait(n); got != maxRetryWait {
			t.Errorf("retryWait(%d) = %s, want %s", n, got, maxRetryWait)
		}
	}
}

func TestRetryAfter(t *testing.T) {
	tests := []struct {
		hdr  http.Header
		want time.Duration
	}{
		{http.Header{}, 0},
		{http.Header{"X-Rate-Limit-Reset": {"5"}}, 5 * time.Second},
		{http.Header{"Retry-After": {"7"}}, 7 * time.Second},
		{http.Header{"X-Rate-Limit-Reset": {"0"}, "Retry-After": {"7"}}, 7 * time.Second},
		{http.Header{"X-Rate-Limit-Reset": {"-3"}}, 0},
		{http.Header{"X-Rate-Limit-Reset": {"nope"}}, 0},
		{http.Header{"X-Rate-Limit-Reset": {"99999"}}, maxRateLimitWait},
	}
	for _, tt := range tests {
		if got := retryAfter(tt.hdr); got != tt.want {
			t.Errorf("retryAfter(%v) = %s, want %s", tt.hdr, got, tt.want)
		}
	}
}
