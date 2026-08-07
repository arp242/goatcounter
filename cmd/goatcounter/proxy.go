package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/monoculum/formam/v3"
	"github.com/sethvargo/go-limiter"
	"github.com/sethvargo/go-limiter/memorystore"
	"golang.org/x/text/language"
	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/handlers"
	"zgo.at/goatcounter/v2/pkg/log"
	"zgo.at/isbot"
	"zgo.at/json"
	"zgo.at/z18n"
	"zgo.at/zhttp"
	"zgo.at/zhttp/mware"
	"zgo.at/zli"
	"zgo.at/zstd/zstring"
)

const usageProxy = `
Run a proxy for the /count endpoint.

Overview:

  The proxy exposes only the /count endpoint that the GoatCounter tracking
  script (and the navigator.sendBeacon API) talk to. It collects pageviews in
  memory, batches them, and forwards them to a "real" GoatCounter server using
  the /api/v0/count API.

  It needs a running GoatCounter server to forward to, and an API key with
  "Record pageviews" permissions set in GOATCOUNTER_API_KEY:

      $ export GOATCOUNTER_API_KEY=[..]
      $ goatcounter proxy -site=https://stats.example.com

  All pageviews are forwarded to the single site the API key belongs to.

  Pageviews are not forwarded immediately, but collected and sent in batches.
  A batch is flushed when any of these happens, whichever comes first:

    - it reaches -batch pageviews,
    - -batch-max-time elapses since the oldest waiting pageview came in, or
    - -batch-min-time elapses without any new pageview coming in.

  If the upstream server can't be reached, or answers with a temporary error
  such as "503 no backend available", the batch is kept and retried. The wait
  between retries doubles up to a minute, and the proxy keeps trying for as long
  as the server stays down. Pageviews that come in meanwhile are queued, up to
  -queue of them. Past that the oldest ones are dropped.

  A batch is only given up on when the upstream server says the request itself
  is the problem (any 4xx other than 429).

Environment:

  All of the flags take the defaults from $GOATCOUNTER_«FLAG», where «FLAG» is
  the flag name. The commandline flag will override the environment variable.

  Additional environment variables:

    GOATCOUNTER_API_KEY   API key; requires "Record pageviews" permission.

Flags:

  -listen      Address to listen on. Default: "*:8080". See "goatcounter help
               listen" for detailed documentation.

  -site        Site to forward to, as an URL (e.g. "https://stats.example.com")

  -batch       Maximum number of pageviews to send in one batch; capped at 500,
               which is the maximum the API accepts. Default: 100.

  -queue       Maximum number of pageviews to keep in memory while they wait to
               be forwarded.

               One pageview needs roughly 600 bytes, so a full queue needs about
               60M at the default. Default: 100000.

  -batch-min-time
               Minimum time to wait before sending a batch: a batch is sent once
               no new pageview has come in for this long. Accepts a Go duration
               such as "500ms", "2s", or "1m". Default: 1s.

  -batch-max-time
               Maximum time to wait before sending a batch: a pageview is never
               held longer than this. Accepts a Go duration. Default: 15s.

  -ratelimit   Rate limit for the /count endpoint, as "requests/seconds"; the
               limit is applied per client (IP + User-Agent). Use "off" to
               disable. Default: 4/1 (4 requests per second).

  -debug       Modules to debug, comma-separated or 'all' for all modules.
               See "goatcounter help debug" for a list of modules.

  -json        Output logs as JSON instead of aligned text.
`

func cmdProxy(f zli.Flags, ready chan<- struct{}, stop chan struct{}) error {
	var (
		debugFlag = f.StringList(nil, "debug")
		jsonFlag  = f.Bool(false, "json")
		listen    = f.String(":8080", "listen")
		siteFlag  = f.String("", "site")
		batch     = f.Int(100, "batch")
		queue     = f.Int(100_000, "queue")
		batchMin  = f.String("1s", "batch-min-time")
		batchMax  = f.String("15s", "batch-max-time")
		ratelimit = f.String("4/1", "ratelimit")
	)
	if err := f.Parse(zli.FromEnv("GOATCOUNTER")); err != nil && !errors.As(err, &zli.ErrUnknownEnv{}) {
		return err
	}

	setupLog(false, jsonFlag.Bool(), debugFlag.StringsSplit(","))

	site := strings.TrimRight(siteFlag.String(), "/")
	if site == "" {
		return errors.New("-site must be set")
	}
	if !zstring.HasPrefixes(site, "http://", "https://") {
		site = "https://" + site
	}

	key := os.Getenv("GOATCOUNTER_API_KEY")
	if key == "" {
		return errors.New("GOATCOUNTER_API_KEY must be set")
	}

	if batch.Int() < 1 {
		return errors.New("-batch must be at least 1")
	}
	if batch.Int() > 500 {
		*batch.Pointer() = 500
	}
	if queue.Int() < batch.Int() {
		return fmt.Errorf("-queue (%d) can't be smaller than -batch (%d)", queue.Int(), batch.Int())
	}

	minWait, err := time.ParseDuration(batchMin.String())
	if err != nil {
		return fmt.Errorf("-batch-min-time: %w", err)
	}
	maxWait, err := time.ParseDuration(batchMax.String())
	if err != nil {
		return fmt.Errorf("-batch-max-time: %w", err)
	}
	if minWait <= 0 || maxWait <= 0 {
		return errors.New("-batch-min-time and -batch-max-time must be positive")
	}
	if minWait > maxWait {
		return errors.New("-batch-min-time can't be larger than -batch-max-time")
	}

	rlStore, err := proxyRatelimit(ratelimit.String())
	if err != nil {
		return err
	}

	if err := checkSite(site, key, goatcounter.APIPermCount); err != nil {
		return err
	}

	p := &proxy{
		url:      site,
		key:      key,
		maxBatch: batch.Int(),
		minWait:  minWait,
		maxWait:  maxWait,
		queue:    newHitQueue(queue.Int()),
		in:       make(chan handlers.APICountRequestHit, min(queue.Int(), hitQueueMin)),
		want:     make(chan struct{}),
		batches:  make(chan []handlers.APICountRequestHit, 1),
	}

	// The batcher has its own stop channel rather than reusing the server's:
	// zhttp.Serve consumes `stop` (and handles OS signals) itself, so we only
	// stop the batcher once the server has shut down and no more pageviews can
	// come in.
	batcherStop := make(chan struct{})
	batcherDone := make(chan struct{})
	go p.run(batcherStop, batcherDone)

	r := chi.NewRouter()
	r.Use(mware.RealIP(), mware.WrapWriter())
	var rr chi.Router = r
	if rlStore != nil {
		rr = r.With(handlers.Ratelimit(true, func(*http.Request) ([]limiter.Store, string) {
			return []limiter.Store{rlStore}, ""
		}))
	}
	rr.Get("/count", zhttp.Wrap(p.count))
	rr.Post("/count", zhttp.Wrap(p.count)) // to support navigator.sendBeacon (JS)

	ch, err := zhttp.Serve(0, stop, &http.Server{
		Addr:    listen.String(),
		Handler: r,
	})
	if err != nil {
		return err
	}

	<-ch // Server is set up.
	log.Module("startup").Info(context.Background(), "GoatCounter proxy ready",
		"listen", listen.String(), "site", site,
		"batch", p.maxBatch, "batch_min", minWait.String(), "batch_max", maxWait.String(),
		"queue", queue.Int())
	ready <- struct{}{}

	<-ch // Server is shut down; no more pageviews will come in.

	// Tell the batcher to flush whatever is left and wait for it.
	close(batcherStop)
	<-batcherDone
	return nil
}

// proxyRatelimit builds the rate-limit store for the /count endpoint from a
// "requests/seconds" spec. A value of "off" (or empty) returns a nil store,
// which disables rate limiting.
func proxyRatelimit(spec string) (limiter.Store, error) {
	if spec == "" || strings.EqualFold(spec, "off") {
		return nil, nil
	}

	reqs, secs, ok := strings.Cut(spec, "/")
	if !ok {
		return nil, fmt.Errorf("-ratelimit: must be as requests/seconds, e.g. 4/1")
	}
	r, err := strconv.Atoi(reqs)
	if err != nil || r < 1 {
		return nil, fmt.Errorf("-ratelimit: invalid number of requests: %q", reqs)
	}
	s, err := strconv.Atoi(secs)
	if err != nil || s < 1 {
		return nil, fmt.Errorf("-ratelimit: invalid number of seconds: %q", secs)
	}

	store, err := memorystore.New(&memorystore.Config{
		Tokens:   uint64(r),
		Interval: time.Duration(s) * time.Second,
	})
	if err != nil {
		return nil, err
	}
	return store, nil
}

// proxy collects pageviews from /count requests, batches them, and forwards
// them to a GoatCounter server's /api/v0/count endpoint.
//
// The work is split over three stages, each owning its own data and talking to
// the next one over channels only:
//
//  1. Receiving: the /count handlers, one goroutine per request as usual for
//     net/http. They parse the pageview and put it on p.in, and do nothing else,
//     so a slow or unreachable upstream server never delays a response.
//
//  2. Keeping: a single goroutine (run) that owns the queue of the last -queue
//     pageviews and decides when a batch is ready. It never does any I/O, so it
//     always keeps up with stage 1.
//
//  3. Forwarding: a single goroutine (forward) that asks stage 2 for a batch,
//     sends it upstream, and retries for as long as that takes. It handles one
//     batch at a time, which keeps them in order and means a batch being
//     retried can't overtake the next one.
type proxy struct {
	url, key string
	maxBatch int
	minWait  time.Duration
	maxWait  time.Duration

	in      chan handlers.APICountRequestHit   // Stage 1 -> 2: one pageview.
	batches chan []handlers.APICountRequestHit // Stage 2 -> 3: the batch, nil when there's no more.
	want    chan struct{}                      // Stage 3 -> 2: ready for a batch.
	queue   *hitQueue                          // Owned by stage 2.
}

const (
	// Smallest the ring buffer ever gets: what it starts at, and what it goes
	// back down to once a backlog has cleared.
	hitQueueMin = 1024

	// How often to report that the queue is full and pageviews are being lost.
	dropLogInterval = time.Minute
)

// hitQueue holds the pageviews that are waiting to be forwarded, in a ring
// buffer with an upper bound on its size. Adding to a full queue overwrites the
// oldest pageview instead of growing further: if the upstream server is down for
// hours we can't keep everything, and the recent pageviews are the ones worth
// keeping.
//
// The ring starts at hitQueueMin and doubles as needed, so a large -queue only
// costs memory once there really is a backlog, and halves again as the backlog
// clears, so that memory is given back afterwards.
//
// It has no locking of its own: only the stage 2 goroutine touches it.
type hitQueue struct {
	max     int // Never hold more than this many pageviews.
	buf     []handlers.APICountRequestHit
	head    int       // Position of the oldest pageview in buf.
	n       int       // Number of pageviews in buf.
	dropped int64     // Total overwritten because the queue was full.
	lastLog time.Time // When we last reported dropping pageviews.
}

func newHitQueue(max int) *hitQueue {
	return &hitQueue{
		max: max,
		buf: make([]handlers.APICountRequestHit, min(max, hitQueueMin)),
	}
}

// push adds a pageview, overwriting the oldest one if the queue is full.
func (q *hitQueue) push(ctx context.Context, h handlers.APICountRequestHit) {
	if q.n == len(q.buf) && q.n < q.max {
		q.grow()
	}
	if q.n == len(q.buf) {
		// Full: overwrite the oldest pageview and move the head along.
		q.buf[q.head] = h
		q.head = (q.head + 1) % len(q.buf)
		q.dropped++

		if now := time.Now(); now.Sub(q.lastLog) >= dropLogInterval {
			q.lastLog = now
			log.Module("proxy").Warnf(ctx, "queue of %d pageviews is full; dropped %d pageviews so far",
				q.max, q.dropped)
		}
		return
	}

	q.buf[(q.head+q.n)%len(q.buf)] = h
	q.n++
}

// grow doubles the ring, up to max.
func (q *hitQueue) grow() {
	q.resize(min(2*len(q.buf), q.max))
}

// shrink halves the ring, as many times as it can, so that the memory claimed
// during a backlog is given back once that backlog has cleared.
func (q *hitQueue) shrink() {
	size := len(q.buf)
	for size/2 >= hitQueueMin && q.n <= size/4 {
		size /= 2
	}
	if size < len(q.buf) {
		q.resize(size)
	}
}

// resize moves the pageviews into a buffer of a different size. They go to the
// start of it, so the oldest one is back at position zero.
func (q *hitQueue) resize(size int) {
	buf := make([]handlers.APICountRequestHit, size)
	for i := range q.n {
		buf[i] = q.buf[(q.head+i)%len(q.buf)]
	}
	q.buf, q.head = buf, 0
}

// pop removes and returns up to max pageviews, oldest first.
func (q *hitQueue) pop(max int) []handlers.APICountRequestHit {
	if q.n == 0 {
		return nil
	}
	n := min(max, q.n)
	batch := make([]handlers.APICountRequestHit, n)
	for i := range n {
		p := (q.head + i) % len(q.buf)
		batch[i] = q.buf[p]
		q.buf[p] = handlers.APICountRequestHit{}
	}
	q.head = (q.head + n) % len(q.buf)
	q.n -= n
	q.shrink()
	return batch
}

// oldest returns when the oldest waiting pageview came in, and false if the
// queue is empty.
func (q *hitQueue) oldest() (time.Time, bool) {
	if q.n == 0 {
		return time.Time{}, false
	}
	return q.buf[q.head].CreatedAt, true
}

// Use GIF because it's the smallest filesize (PNG is 116 bytes, vs 43 for GIF).
var gif = []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x1, 0x0, 0x1, 0x0, 0x80,
	0x1, 0x0, 0x0, 0x0, 0x0, 0xff, 0xff, 0xff, 0x21, 0xf9, 0x4, 0x1, 0xa, 0x0,
	0x1, 0x0, 0x2c, 0x0, 0x0, 0x0, 0x0, 0x1, 0x0, 0x1, 0x0, 0x0, 0x2, 0x2, 0x4c,
	0x1, 0x0, 0x3b}

// count handles a /count request: it parses the pageview from the query string
// like the regular /count endpoint does, and queues it for forwarding.
func (p *proxy) count(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
	w.Header().Set("Connection", "close")

	bot := isbot.Bot(r)
	if bot == isbot.BotPrefetch {
		return zhttp.Bytes(w, gif)
	}

	// Site is not known here: the API key decides which site the pageview ends
	// up on. Validate only checks that the field is set.
	hit := goatcounter.Hit{
		Site:            1,
		UserAgentHeader: r.UserAgent(),
		CreatedAt:       time.Now().UTC(),
	}
	err := formam.NewDecoder(&formam.DecoderOptions{
		TagName:           "json",
		IgnoreUnknownKeys: true,
	}).Decode(r.URL.Query(), &hit)
	if err != nil {
		w.Header().Add("X-Goatcounter", fmt.Sprintf("error decoding parameters: %s", err))
		w.WriteHeader(400)
		return zhttp.Bytes(w, gif)
	}
	if hit.Bot > 0 && hit.Bot < 150 {
		w.Header().Add("X-Goatcounter", fmt.Sprintf("wrong value: b=%d", hit.Bot))
		w.WriteHeader(400)
		return zhttp.Bytes(w, gif)
	}
	for _, s := range hit.Size {
		if s > math.MaxInt32 {
			w.Header().Add("X-Goatcounter", fmt.Sprintf("ignored because screen size %v is out of range of int32", s))
			w.WriteHeader(400)
			return zhttp.Bytes(w, gif)
		}
	}
	if isbot.Is(bot) { // Prefer the backend detection.
		hit.Bot = int(bot)
	}

	err = hit.Validate(z18n.With(r.Context(), goatcounter.DefaultLocale), true)
	if err != nil {
		w.Header().Add("X-Goatcounter", fmt.Sprintf("not valid: %s", err))
		w.WriteHeader(400)
		return zhttp.Bytes(w, gif)
	}

	a := handlers.APICountRequestHit{
		Path:      hit.Path,
		Title:     hit.Title,
		Event:     hit.Event,
		Ref:       hit.Ref,
		Size:      hit.Size,
		Query:     hit.Query,
		Bot:       hit.Bot,
		UserAgent: hit.UserAgentHeader,
		IP:        r.RemoteAddr,
		CreatedAt: hit.CreatedAt,
	}
	// Forward the preferred language as BCP 47; the central server decides
	// whether to record it based on its own "Collect" settings.
	if tags, _, _ := language.ParseAcceptLanguage(r.Header.Get("Accept-Language")); len(tags) > 0 {
		a.Language = tags[0].String()
	}

	// Hand over to stage 2 and get out of the way. This only ever waits for
	// another goroutine to copy a value, never for the upstream server.
	p.in <- a
	return zhttp.Bytes(w, gif)
}

// How long to keep retrying after the HTTP server has stopped, before giving
// up on the pageviews that are still waiting.
const shutdownGrace = 5 * time.Second

// run is stage 2: it owns the queue, takes pageviews from the /count handlers,
// and hands a batch to the forwarding goroutine whenever that one asks for
// work and there's a batch worth sending.
//
// A batch is ready when it holds -batch pageviews, when -batch-max-time has
// passed since the oldest waiting pageview came in, or when -batch-min-time has
// passed without any new pageview.
//
// It returns (closing done) once stop is closed and everything left in the
// queue has been forwarded, or given up on.
func (p *proxy) run(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)

	ctx := context.Background()

	// Start forwarding goroutine. When stopping this one, allows a few seconds
	// for the forwarding gorouting to stop.
	sendStop := make(chan struct{})
	forwardDone := make(chan struct{})
	go func() {
		<-stop
		time.Sleep(shutdownGrace)
		close(sendStop)
	}()
	go p.forward(sendStop, forwardDone)

	var (
		pending  bool // The forwarder is waiting for a batch.
		flush    bool // A timer went off: hand over what we have.
		minTimer = stoppedTimer()
		maxTimer = stoppedTimer()
	)
	for {
		if pending && p.queue.n > 0 && (flush || p.queue.n >= p.maxBatch) {
			p.batches <- p.queue.pop(p.maxBatch)
			pending, flush = false, false

			minTimer.Stop()
			maxTimer.Stop()
			// Anything left over has been waiting since before this batch, so
			// take that off its -batch-max-time.
			if oldest, ok := p.queue.oldest(); ok {
				minTimer.Reset(p.minWait)
				maxTimer.Reset(max(0, p.maxWait-time.Since(oldest)))
			}
			continue
		}

		select {
		case h := <-p.in:
			if p.queue.n == 0 { // First of a new batch: start the clock.
				maxTimer.Reset(p.maxWait)
				flush = false
			}
			minTimer.Reset(p.minWait) // Wait for a quiet period again.
			p.queue.push(ctx, h)
		case <-p.want:
			pending = true
		case <-minTimer.C:
			flush = true
		case <-maxTimer.C:
			flush = true
		case <-stop:
			p.drain(ctx, pending, forwardDone)
			if p.queue.dropped > 0 {
				log.Module("proxy").Warnf(ctx, "dropped %d pageviews because the queue was full",
					p.queue.dropped)
			}
			return
		}
	}
}

// drain forwards whatever is left once the HTTP server has stopped and no new
// pageviews can come in, and then waits for the forwarding goroutine to finish.
func (p *proxy) drain(ctx context.Context, pending bool, forwardDone <-chan struct{}) {
	for {
		// Pick up anything the handlers left behind.
		for done := false; !done; {
			select {
			case h := <-p.in:
				p.queue.push(ctx, h)
			default:
				done = true
			}
		}

		if !pending {
			<-p.want
		}
		batch := p.queue.pop(p.maxBatch)
		p.batches <- batch // A nil batch tells the forwarder to stop.
		if batch == nil {
			<-forwardDone
			return
		}
		pending = false
	}
}

// stoppedTimer returns a stopped timer that can be (re)started with Reset.
func stoppedTimer() *time.Timer {
	t := time.NewTimer(time.Hour)
	t.Stop()
	return t
}

// forward is stage 3: it asks the queue for a batch, sends it upstream, and
// only comes back for the next one when it's done. It returns (closing done)
// when the queue says there's nothing left.
func (p *proxy) forward(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)

	for {
		p.want <- struct{}{}
		batch := <-p.batches
		if batch == nil {
			return
		}
		p.send(batch, stop)
	}
}

var proxyClient = http.Client{Timeout: 10 * time.Second}

const (
	// Upper bound for the wait between two retries.
	maxRetryWait = time.Minute

	// Upper bound for how long we believe an upstream server that asks us to
	// slow down.
	maxRateLimitWait = 5 * time.Minute
)

// sendResult says what to do with a batch after trying to forward it.
type sendResult int

const (
	sendOK    sendResult = iota // Accepted by the upstream server.
	sendRetry                   // Temporary problem; the same batch can be sent again.
	sendDrop                    // The batch or our configuration is wrong; sending it again won't help.
)

// send forwards a batch of pageviews to the upstream server's /api/v0/count
// endpoint. It keeps retrying as long as the problem looks temporary, so an
// outage of any length is survivable; it only gives up when stop is closed, or
// when the upstream server says the batch itself is the problem.
func (p *proxy) send(hits []handlers.APICountRequestHit, stop <-chan struct{}) {
	ctx := context.Background()

	body, err := json.Marshal(handlers.APICountRequest{Hits: hits})
	if err != nil {
		log.Module("proxy").Error(ctx, err)
		return
	}

	for attempt := 0; ; attempt++ {
		result, wait, reason := p.post(ctx, body, len(hits))
		switch result {
		case sendOK:
			if attempt > 0 {
				log.Module("proxy").Infof(ctx, "%s is back; forwarded %d pageviews after %d retries",
					p.url, len(hits), attempt)
			}
			return
		case sendDrop:
			log.Module("proxy").Warnf(ctx, "dropping %d pageviews: %s", len(hits), reason)
			return
		}

		if wait <= 0 {
			wait = retryWait(attempt)
		}
		if attempt == 0 { // Only report the first failure; the rest is noise.
			log.Module("proxy").Warnf(ctx, "can't forward %d pageviews, retrying until it works: %s",
				len(hits), reason)
		} else {
			log.Module("proxy").Debugf(ctx, "retry %d for %d pageviews in %s: %s",
				attempt, len(hits), wait, reason)
		}

		t := time.NewTimer(wait)
		select {
		case <-t.C:
		case <-stop:
			t.Stop()
			log.Module("proxy").Warnf(ctx, "shutting down while %s is unreachable: dropped %d pageviews",
				p.url, len(hits))
			return
		}
	}
}

// post sends one batch and reports what to do next. The returned duration is
// how long the upstream server asked us to wait, and is zero if it didn't ask
// for anything in particular.
func (p *proxy) post(ctx context.Context, body []byte, n int) (sendResult, time.Duration, error) {
	r, err := newRequest("POST", p.url+"/api/v0/count", p.key, bytes.NewReader(body))
	if err != nil {
		return sendDrop, 0, err
	}

	log.Module("proxy").Debugf(ctx, "POST %s with %d pageviews", p.url, n)
	resp, err := proxyClient.Do(r)
	if err != nil {
		// Connection refused, DNS failure, timeout: the server should come back.
		return sendRetry, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 202 {
		io.Copy(io.Discard, resp.Body) // Read to the end so the connection can be reused.
		return sendOK, 0, nil
	}

	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	reason := fmt.Errorf("%s: %s: %s", p.url+"/api/v0/count", resp.Status,
		zstring.ElideLeft(strings.TrimSpace(string(b)), 500))

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return sendRetry, retryAfter(resp.Header), reason
	case retryableStatus(resp.StatusCode):
		return sendRetry, 0, reason
	default:
		return sendDrop, 0, reason
	}
}

// retryableStatus reports whether it's worth sending the same batch again after
// this status code.
//
// A 5xx means the upstream server, or a proxy in front of it, is having trouble
// ("no backend available" and such) and should come back later. A 4xx means the
// request is not acceptable and repeating it gets the same answer, so the batch
// is given up on; that also covers a wrong -site or a revoked API key. The
// exceptions are 429, handled by the caller, and the two codes that report a
// timing problem rather than a bad request.
func retryableStatus(code int) bool {
	switch code {
	case http.StatusRequestTimeout, http.StatusTooEarly:
		return true
	}
	return code >= 500
}

// retryAfter reads how long the upstream server wants us to wait, in seconds.
// It returns zero if there's no usable value, and the caller then falls back to
// its own backoff.
func retryAfter(h http.Header) time.Duration {
	for _, k := range []string{"X-Rate-Limit-Reset", "Retry-After"} {
		if s, err := strconv.Atoi(h.Get(k)); err == nil && s > 0 {
			return min(time.Duration(s)*time.Second, maxRateLimitWait)
		}
	}
	return 0
}

// retryWait returns how long to wait before retry number n: it starts at a
// second and doubles every time, up to maxRetryWait.
func retryWait(n int) time.Duration {
	return min(time.Second<<min(n, 10), maxRetryWait)
}
