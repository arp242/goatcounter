package goatcounter

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"zgo.at/goatcounter/v2/pkg/log"
	"zgo.at/json"
	"zgo.at/zdb"
	"zgo.at/zstd/zbool"
	"zgo.at/zstd/zint"
	"zgo.at/zstd/ztime"
	"zgo.at/zstd/ztype"
	"zgo.at/zvalidate"
)

var (
	// Valid UUID for testing: 00112233-4455-6677-8899-aabbccddeeff
	TestSession    = zint.Uint128{0x11223344556677, 0x8899aabbccddeeff}
	TestSeqSession = zint.Uint128{TestSession[0], TestSession[1] + 1}
)

var (
	memlog  = log.Module("memstore")
	sesslog = log.Module("session")
)

type sessionKey string

type ms struct {
	hitMu sync.RWMutex
	hits  []Hit

	sessionMu     sync.RWMutex
	sessions      map[sessionKey]zint.Uint128          // sessionKey → sessionID
	sessionHashes map[zint.Uint128]sessionKey          // sessionID → sessionKey
	sessionPaths  map[zint.Uint128]map[PathID]struct{} // SessionID → path_id
	sessionSeen   map[zint.Uint128]int64               // SessionID → lastseen

	testHook bool
}

var Memstore ms

type storedSession struct {
	Sessions map[sessionKey]zint.Uint128          `json:"sessions"`
	Hashes   map[zint.Uint128]sessionKey          `json:"hashes"`
	Paths    map[zint.Uint128]map[PathID]struct{} `json:"paths"`
	Seen     map[zint.Uint128]int64               `json:"seen"`
}

func (m *ms) Reset() {
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()

	m.sessions = make(map[sessionKey]zint.Uint128)
	m.sessionHashes = make(map[zint.Uint128]sessionKey)
	m.sessionPaths = make(map[zint.Uint128]map[PathID]struct{})
	m.sessionSeen = make(map[zint.Uint128]int64)
	TestSeqSession = zint.Uint128{TestSession[0], TestSession[1] + 1}
}

// TestInit is like Init(), but enables the test hook to return sequential UUIDs
// instead of random ones.
func (m *ms) TestInit(db zdb.DB) error {
	m.testHook = true
	return m.Init(db)
}

func (m *ms) Init(db zdb.DB) error {
	m.hitMu.Lock()
	defer m.hitMu.Unlock()

	m.Reset()
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()
	defer func() {
		err := db.Exec(context.Background(), `delete from store where key='session'`)
		if err != nil {
			log.Errorf(context.Background(), "Memstore.Init: delete DB store: %s", err)
		}
	}()

	var s []byte
	err := db.Get(context.Background(), &s, `select value from store where key='session'`)
	if err != nil {
		if zdb.ErrNoRows(err) {
			memlog.Debugf(context.Background(), "no sessions stored in DB")
			return nil
		}
		memlog.Errorf(context.Background(), "load from DB store: %s", err)
		return nil
	}

	var stored storedSession
	err = json.Unmarshal(s, &stored)
	if err != nil {
		memlog.Errorf(context.Background(), "unmarshal from DB store: %s", err)
		return nil
	}

	if stored.Sessions != nil {
		m.sessions = stored.Sessions
	}
	if stored.Hashes != nil {
		m.sessionHashes = stored.Hashes
	}
	if stored.Paths != nil {
		m.sessionPaths = stored.Paths
	}
	if stored.Seen != nil {
		m.sessionSeen = stored.Seen
	}
	memlog.Debug(context.Background(), "restored sessions from DB",
		"sessions", len(m.sessions),
		"sessionHashes", len(m.sessionHashes),
		"sessionPaths", len(m.sessionPaths),
		"sessionSeen", len(m.sessionSeen))
	return nil
}

func (m *ms) StoreSessions(db zdb.DB) {
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()

	d, err := json.Marshal(storedSession{
		Sessions: m.sessions,
		Paths:    m.sessionPaths,
		Seen:     m.sessionSeen,
		Hashes:   m.sessionHashes,
	})
	if err != nil {
		memlog.Error(context.Background(), err)
		return
	}

	err = db.Exec(context.Background(),
		`insert into store (key, value) values ('session', $1)`, d)
	if err != nil {
		memlog.Error(context.Background(), err)
	}

	memlog.Debug(context.Background(), "stored sessions in DB on shutdown",
		"bytesize", len(d),
		"sessions", len(m.sessions),
		"sessionHashes", len(m.sessionHashes),
		"sessionPaths", len(m.sessionPaths),
		"sessionSeen", len(m.sessionSeen))
}

func (m *ms) Append(hits ...Hit) {
	m.hitMu.Lock()
	m.hits = append(m.hits, hits...)
	m.hitMu.Unlock()
}

func (m *ms) SessionsLen() int {
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()
	return len(m.sessions)
}

func (m *ms) Len() int {
	m.hitMu.Lock()
	defer m.hitMu.Unlock()
	return len(m.hits)
}

var (
	refspamSubdomains []string
	refspamOnce       sync.Once
)

func isRefspam(host string) bool {
	if _, ok := refspam[host]; ok {
		return true
	}

	refspamOnce.Do(func() {
		refspamSubdomains = make([]string, 0, len(refspam))
		for v := range refspam {
			refspamSubdomains = append(refspamSubdomains, "."+v)
		}
	})

	for _, v := range refspamSubdomains {
		if strings.HasSuffix(host, v) {
			return true
		}
	}
	return false
}

func (m *ms) Persist(ctx context.Context) ([]Hit, error) {
	if m.Len() == 0 {
		return nil, nil
	}

	m.hitMu.Lock()
	hits := make([]Hit, len(m.hits))
	copy(hits, m.hits)
	m.hits = make([]Hit, 0, 16)
	m.hitMu.Unlock()

	var (
		newHits = make([]Hit, 0, len(hits))
		bot     = zdb.NewBulkInsert(ctx, "bots", []string{"site_id", "path", "bot", "user_agent", "created_at"})
		ins     = zdb.NewBulkInsert(ctx, "hits", []string{"site_id", "path_id", "ref_id", "browser_id", "system_id",
			"width", "location", "language", "created_at", "session", "first_visit", "campaign"})
	)
	for _, h := range hits {
		if h.Bot > 0 {
			bot.Values(h.Site, h.Path, h.Bot, h.UserAgentHeader, h.CreatedAt)
			continue
		}
		if m.processHit(ctx, &h) {
			// Don't return hits that failed validation; otherwise cron will try to
			// insert them.
			newHits = append(newHits, h)

			if !h.NoStore {
				var w *float64
				if len(h.Size) > 0 {
					w = &h.Size[0]
				}
				ins.Values(h.Site, h.PathID, h.RefID, h.BrowserID, h.SystemID, w, h.Location,
					h.Language, h.CreatedAt.Round(time.Second), h.Session, h.FirstVisit, h.CampaignID)
			}
		}
	}

	// Just log errors on inserting bots; not that important.
	if err := bot.Finish(); err != nil {
		memlog.Errorf(ctx, "storing bots: %s", err)
	}
	return newHits, ins.Finish()
}

func (m *ms) processHit(ctx context.Context, h *Hit) bool {
	defer log.Recover(ctx, func(err error) { memlog.Error(ctx, err, "hit", h) })

	if h.noProcess {
		return true
	}

	// Ignore spammers.
	h.RefURL, _ = url.Parse(h.Ref)
	if h.RefURL != nil {
		if isRefspam(h.RefURL.Host) {
			memlog.Debugf(ctx, "refspam ignored: %q", h.RefURL.Host)
			return false
		}
	}

	var site Site
	err := site.ByID(ctx, h.Site)
	if err != nil {
		memlog.Error(ctx, err, "hit", h)
		return false
	}
	ctx = WithSite(ctx, &site)
	if !site.Settings.Collect.Has(CollectHits) {
		h.NoStore = true
	}

	if !site.Settings.Collect.Has(CollectReferrer) {
		h.Query, h.Ref, h.RefScheme, h.RefURL = "", "", "", nil
	}

	err = h.Defaults(ctx, false)
	if err != nil {
		if errors.As(err, ztype.Ptr(&zvalidate.Validator{})) {
			memlog.Debug(ctx, err.Error(), "hit", h)
		} else {
			memlog.Error(ctx, err, "hit", h)
		}
		return false
	}

	if h.Session.IsZero() && site.Settings.Collect.Has(CollectSession) {
		h.Session, h.FirstVisit = m.session(ctx, site.ID, h.PathID, h.UserSessionID, h.UserAgentHeader, h.RemoteAddr)
	}

	if !site.Settings.Collect.Has(CollectSession) {
		h.Session, h.FirstVisit = zint.Uint128{}, true
	}
	if !site.Settings.Collect.Has(CollectScreenSize) {
		h.Size = nil
	}
	if !site.Settings.Collect.Has(CollectUserAgent) {
		h.UserAgentHeader, h.BrowserID, h.SystemID = "", 0, 0
	}
	if !site.Settings.Collect.Has(CollectLanguage) {
		h.Language = nil
	}
	if !site.Settings.Collect.Has(CollectLocation) {
		h.Location = ""
	}
	if strings.ContainsRune(h.Location, '-') {
		trim := !site.Settings.Collect.Has(CollectLocationRegion)
		if !trim && len(site.Settings.CollectRegions) > 0 {
			trim = !slices.Contains(site.Settings.CollectRegions, h.Location[:2])
		}
		if trim {
			var loc Location
			err := loc.ByCode(ctx, h.Location[:2])
			if err != nil {
				memlog.Errorf(ctx, "lookup %q: %s", h.Location[:2], err)
			}
			h.Location = loc.ISO3166_2
		}
	}

	if h.Ignore() {
		return false
	}

	err = h.Validate(ctx, false)
	if err != nil {
		memlog.Error(ctx, err, "hit", h)
		return false
	}
	return true
}

// SessionTime is the maximum length of sessions; exported here for tests.
var SessionTime = 8 * time.Hour

// For 10k sessions this takes about 5ms on my laptop; that's a small enough
// delay to not overly worry about (there are rarely more than a few hundred
// sessions at a time).
func (m *ms) EvictSessions(ctx context.Context) {
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()

	ev := ztime.Now(ctx).Add(-SessionTime).Unix()
	for id, seen := range m.sessionSeen {
		if seen > ev {
			continue
		}

		sk := m.sessionHashes[id]

		sesslog.Debug(context.Background(), "evicting session",
			"session-id", id,
			"last-seen", seen,
			"session-key", sk)

		delete(m.sessions, sk)
		delete(m.sessionPaths, id)
		delete(m.sessionSeen, id)
		delete(m.sessionHashes, id)
	}
}

// SessionID gets a new UUID4 session ID.
func (m *ms) SessionID() zint.Uint128 {
	if m.testHook {
		TestSeqSession[1]++
		return TestSeqSession
	}
	return UUID()
}

func (m *ms) session(ctx context.Context, siteID SiteID, pathID PathID, userSessionID, ua, remoteAddr string) (zint.Uint128, zbool.Bool) {
	sk := sessionKey(userSessionID)
	if userSessionID == "" {
		sk = sessionKey(fmt.Sprintf("%s-%s-%d", ua, remoteAddr, siteID))
	}

	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()

	id, ok := m.sessions[sk]
	if ok { // Existing session
		m.sessionSeen[id] = ztime.Now(ctx).Unix()
		_, seenPath := m.sessionPaths[id][pathID]
		if !seenPath {
			m.sessionPaths[id][pathID] = struct{}{}
		}

		sesslog.Debug(ctx, "HIT",
			"session-key", sk,
			"session-id", id,
			"path", pathID,
			"seen-path", seenPath)
		return id, zbool.Bool(!seenPath)
	}

	// New session
	id = m.SessionID()
	m.sessions[sk] = id
	m.sessionPaths[id] = map[PathID]struct{}{pathID: struct{}{}}
	m.sessionSeen[id] = ztime.Now(ctx).Unix()
	m.sessionHashes[id] = sk

	sesslog.Debug(ctx, "MISS: created new",
		"session-key", sk,
		"session-id", id,
		"path", pathID)
	return id, true
}
