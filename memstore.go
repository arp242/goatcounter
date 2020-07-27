// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"context"
	"crypto/sha256"
	"encoding"
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"zgo.at/json"
	"zgo.at/zdb"
	"zgo.at/zdb/bulk"
	"zgo.at/zlog"
	"zgo.at/zstd/zcrypto"
	"zgo.at/zstd/zint"
)

var (
	// Valid UUID for testing: 00112233-4455-6677-8899-aabbccddeeff
	TestSession    = zint.Uint128{0x11223344556677, 0x8899aabbccddeeff}
	TestSeqSession = zint.Uint128{TestSession[0], TestSession[1] + 1}
)

// The json encoder doesn't like binary data, so base64 it; need struct as it'll
// ignore MarshalText on "type hash string" (but not UnmarshalText? Hmm)
type hash struct{ v string }

var (
	_ encoding.TextMarshaler   = hash{}
	_ encoding.TextUnmarshaler = &hash{}
)

// MarshalText converts the data to a human readable representation.
func (h hash) MarshalText() ([]byte, error) {
	b := base64.StdEncoding.EncodeToString([]byte(h.v))
	return []byte(b), nil
}

// UnmarshalText parses text in to the Go data structure.
func (h *hash) UnmarshalText(v []byte) error {
	b, err := base64.StdEncoding.DecodeString(string(v))
	h.v = string(b)
	return err
}

type ms struct {
	hitMu sync.RWMutex
	hits  []Hit

	sessionMu     sync.RWMutex
	sessions      map[hash]zint.Uint128                // Hash → sessionID
	sessionHashes map[zint.Uint128]hash                // sessionID → hash
	sessionPaths  map[zint.Uint128]map[string]struct{} // SessionID → Path
	sessionSeen   map[zint.Uint128]int64               // SessionID → lastseen
	curSalt       []byte
	prevSalt      []byte
	saltRotated   time.Time

	testHook bool
}

var Memstore ms

type storedSession struct {
	Sessions    map[hash]zint.Uint128                `json:"sessions"`
	Hashes      map[zint.Uint128]hash                `json:"hashes"`
	Paths       map[zint.Uint128]map[string]struct{} `json:"paths"`
	Seen        map[zint.Uint128]int64               `json:"seen"`
	CurSalt     []byte                               `json:"cur_salt"`
	PrevSalt    []byte                               `json:"prev_salt"`
	SaltRotated time.Time                            `json:"salt_rotated"`
}

func (m *ms) Reset() {
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()

	m.sessions = make(map[hash]zint.Uint128)
	m.sessionHashes = make(map[zint.Uint128]hash)
	m.sessionPaths = make(map[zint.Uint128]map[string]struct{})
	m.sessionSeen = make(map[zint.Uint128]int64)
	m.curSalt = []byte(zcrypto.Secret256())
	m.prevSalt = []byte(zcrypto.Secret256())
	m.saltRotated = Now()
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

	var s []byte
	err := db.GetContext(context.Background(), &s,
		`select value from store where key='session'`)
	if err != nil {
		if zdb.ErrNoRows(err) {
			return nil
		}
		return fmt.Errorf("Memstore.Init: load from DB store: %w", err)
	}

	var stored storedSession
	err = json.Unmarshal(s, &stored)
	if err != nil {
		return fmt.Errorf("Memstore.Init: %w", err)
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
	if len(stored.CurSalt) > 0 {
		m.curSalt = stored.CurSalt
	}
	if len(stored.PrevSalt) > 0 {
		m.prevSalt = stored.PrevSalt
	}
	if !stored.SaltRotated.IsZero() {
		m.saltRotated = stored.SaltRotated
	}

	_, err = db.ExecContext(context.Background(), `delete from store where key='session'`)
	if err != nil {
		return fmt.Errorf("Memstore.Init: delete DB store: %w", err)
	}

	return nil
}

func (m *ms) StoreSessions(db zdb.DB) {
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()

	d, err := json.Marshal(storedSession{
		Sessions:    m.sessions,
		Paths:       m.sessionPaths,
		Seen:        m.sessionSeen,
		Hashes:      m.sessionHashes,
		CurSalt:     m.curSalt,
		PrevSalt:    m.prevSalt,
		SaltRotated: m.saltRotated,
	})
	if err != nil {
		zlog.Error(err)
		return
	}

	_, err = db.ExecContext(context.Background(),
		`insert into store (key, value) values ('session', $1)`, d)
	if err != nil {
		zlog.Error(err)
	}
}

func (m *ms) Append(hits ...Hit) {
	m.hitMu.Lock()
	m.hits = append(m.hits, hits...)
	m.hitMu.Unlock()
}

func (m *ms) Len() int {
	m.hitMu.Lock()
	l := len(m.hits)
	m.hitMu.Unlock()
	return l
}

func (m *ms) Persist(ctx context.Context) ([]Hit, error) {
	if m.Len() == 0 {
		return nil, nil
	}

	m.hitMu.Lock()
	hits := make([]Hit, len(m.hits))
	copy(hits, m.hits)
	m.hits = []Hit{}
	m.hitMu.Unlock()

	sites := make(map[int64]*Site)

	l := zlog.Module("memstore")

	ins := bulk.NewInsert(ctx, "hits", []string{"site", "path", "ref",
		"ref_scheme", "browser", "size", "location", "created_at", "bot",
		"title", "event", "session2", "first_visit"})
	for i, h := range hits {
		// Ignore spammers.
		h.RefURL, _ = url.Parse(h.Ref)
		if h.RefURL != nil {
			if _, ok := refspam[h.RefURL.Host]; ok {
				l.Debugf("refspam ignored: %q", h.RefURL.Host)
				continue
			}
		}

		site, ok := sites[h.Site]
		if !ok {
			site = new(Site)
			err := site.ByID(ctx, h.Site)
			if err != nil {
				l.Field("hit", h).Error(err)
				continue
			}
			sites[h.Site] = site
		}
		ctx = WithSite(ctx, site)

		if h.Session.IsZero() {
			h.Session, h.FirstVisit = m.session(ctx, site.ID, h.UserSessionID, h.Path, h.Browser, h.RemoteAddr)
		}

		// Persist.
		h.Defaults(ctx)
		err := h.Validate(ctx)
		if err != nil {
			l.Field("hit", h).Error(err)
			continue
		}

		// Some values are sanitized in Hit.Defaults(), make sure this is
		// reflected in the hits object too, which matters for the hit_stats
		// generation later.
		hits[i] = h

		ins.Values(h.Site, h.Path, h.Ref, h.RefScheme, h.Browser, h.Size,
			h.Location, h.CreatedAt.Format(zdb.Date), h.Bot, h.Title, h.Event,
			h.Session, h.FirstVisit)
	}

	return hits, ins.Finish()
}

func (m *ms) GetSalt() (cur []byte, prev []byte) {
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()
	return m.curSalt, m.prevSalt
}

func (m *ms) RefreshSalt() {
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()

	if m.saltRotated.Add(4 * time.Hour).After(Now()) {
		return
	}

	m.prevSalt = m.curSalt[:]
	m.curSalt = []byte(zcrypto.Secret256())
}

// For 10k sessions this takes about 5ms on my laptop; that's a small enough
// delay to not overly worry about (there are rarely more than a few hundred
// sessions at a time).
func (m *ms) EvictSessions() {
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()

	ev := Now().Add(-4 * time.Hour).Unix()
	for sID, seen := range m.sessionSeen {
		if seen > ev {
			continue
		}

		hash := m.sessionHashes[sID]
		delete(m.sessions, hash)
		delete(m.sessionPaths, sID)
		delete(m.sessionSeen, sID)
		delete(m.sessionHashes, sID)
	}
}

// SessionID gets a new UUID4 session ID.
func (m *ms) SessionID() zint.Uint128 {
	if m.testHook {
		TestSeqSession[1]++
		return TestSeqSession
	}

	u, err := uuid.NewRandom()
	if err != nil {
		// Only failure here is if reading random failed.
		panic(fmt.Sprintf("Memstore.SessionID: uuid.NewRandom: %s", err))
	}

	i, err := zint.NewUint128(u[:])
	if err != nil {
		panic(fmt.Sprintf("Memstore.SessionID: %s", err))
	}

	return i
}

func (m *ms) session(ctx context.Context, siteID int64, userSessionID, path, ua, remoteAddr string) (zint.Uint128, zdb.Bool) {
	sessionHash := hash{userSessionID}

	if userSessionID == "" {
		h := sha256.New()
		h.Write(append(append(append(m.curSalt, ua...), remoteAddr...), strconv.FormatInt(siteID, 10)...))
		sessionHash = hash{string(h.Sum(nil))}
	}

	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()

	id, ok := m.sessions[sessionHash]
	if !ok && userSessionID == "" { // Try previous hash
		h := sha256.New()
		h.Write(append(append(append(m.prevSalt, ua...), remoteAddr...), strconv.FormatInt(siteID, 10)...))
		prev := hash{string(h.Sum(nil))}
		id, ok = m.sessions[prev]
		if ok {
			sessionHash = prev
		}
	}

	if ok { // Existing session
		m.sessionSeen[id] = Now().Unix()
		_, seenPath := m.sessionPaths[id][path]
		if !seenPath {
			m.sessionPaths[id][path] = struct{}{}
		}
		return id, zdb.Bool(!seenPath)
	}

	// New session
	id = m.SessionID()
	m.sessions[sessionHash] = id
	m.sessionPaths[id] = map[string]struct{}{path: struct{}{}}
	m.sessionSeen[id] = Now().Unix()
	m.sessionHashes[id] = sessionHash
	return id, true
}
