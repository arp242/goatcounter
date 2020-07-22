// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"sync"
	"time"

	"zgo.at/zdb"
	"zgo.at/zdb/bulk"
	"zgo.at/zhttp"
	"zgo.at/zlog"
)

type ms struct {
	hitMu sync.RWMutex
	hits  []Hit

	sessionMu     sync.RWMutex
	sessionID     int64                         // Incrementing session ID
	sessions      map[string]int64              // Hash → sessionID
	sessionHashes map[int64]string              // sessionID → hash
	sessionPaths  map[int64]map[string]struct{} // SessionID → Path
	sessionSeen   map[int64]int64               // SessionID → lastseen
	curSalt       []byte
	prevSalt      []byte
	saltRotated   time.Time
}

var Memstore ms

type storedSession struct {
	ID          int64                         `json:"id"`
	Sessions    map[string]int64              `json:"sessions"`
	Hashes      map[int64]string              `json:"hashes"`
	Paths       map[int64]map[string]struct{} `json:"paths"`
	Seen        map[int64]int64               `json:"seen"`
	CurSalt     []byte                        `json:"cur_salt"`
	PrevSalt    []byte                        `json:"prev_salt"`
	SaltRotated time.Time                     `json:"salt_rotated"`
}

func (m *ms) Reset() {
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()

	m.sessionID = 0
	m.sessions = make(map[string]int64)
	m.sessionHashes = make(map[int64]string)
	m.sessionPaths = make(map[int64]map[string]struct{})
	m.sessionSeen = make(map[int64]int64)
	m.curSalt = []byte(zhttp.Secret256())
	m.prevSalt = []byte(zhttp.Secret256())
	m.saltRotated = Now()
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
		return fmt.Errorf("NewMemstore: load from DB store: %w", err)
	}

	var stored storedSession
	err = json.Unmarshal(s, &stored)
	if err != nil {
		return fmt.Errorf("NewMemstore: %w", err)
	}

	m.sessionID = stored.ID
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
		return fmt.Errorf("NewMemstore: delete DB store: %w", err)
	}

	return nil
}

func (m *ms) StoreSessions(db zdb.DB) {
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()

	d, err := json.Marshal(storedSession{
		ID:          m.sessionID,
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
		"title", "event", "session", "first_visit"})
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

		if h.Session == nil || *h.Session == 0 {
			s, f := m.session(ctx, site.ID, h.Path, h.Browser, h.RemoteAddr)
			h.Session, h.FirstVisit = &s, zdb.Bool(f)
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
	m.curSalt = []byte(zhttp.Secret256())
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

// WARNING: this assumes m.sessionMu is not locked.
func (m *ms) NextSessionID() int64 {
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()
	return m.nextSessionIDWithoutLock()
}

func (m *ms) nextSessionIDWithoutLock() int64 {
	// TODO: there should be an option to get a unique sessionID across multiple
	// machines; using an UUID might be better, or perhaps rely on the DB and/or
	// some light-weight syncing.
	m.sessionID++
	return m.sessionID
}

func (m *ms) session(ctx context.Context, siteID int64, path, ua, remoteAddr string) (int64, bool) {
	h := sha256.New()
	h.Write(append(append(append(m.curSalt, ua...), remoteAddr...), strconv.FormatInt(siteID, 10)...))
	hash := string(h.Sum(nil))

	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()

	sessionID, ok := m.sessions[hash]
	if !ok { // Try previous hash
		h := sha256.New()
		h.Write(append(append(append(m.prevSalt, ua...), remoteAddr...), strconv.FormatInt(siteID, 10)...))
		prevHash := string(h.Sum(nil))
		sessionID, ok = m.sessions[prevHash]
		if ok {
			hash = prevHash
		}
	}

	if ok { // Existing session
		m.sessionSeen[m.sessionID] = Now().Unix()
		_, seenPath := m.sessionPaths[sessionID][path]
		if !seenPath {
			m.sessionPaths[sessionID][path] = struct{}{}
		}
		return sessionID, !seenPath
	}

	// New session
	id := m.nextSessionIDWithoutLock()
	m.sessions[hash] = id
	m.sessionPaths[id] = map[string]struct{}{path: struct{}{}}
	m.sessionSeen[id] = Now().Unix()
	m.sessionHashes[id] = hash
	return id, true
}
