// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"zgo.at/json"
	zcache2 "zgo.at/zcache/v2"
	"zgo.at/zlog"
	"zgo.at/zstd/zint"
)

// On dashboard view we generate a unique ID we send to the frontend, and
// register a new loader:
//
//	loader.register(someUnqiueID)
//
// The frontend initiatsed a WS connection, and we create a new connection here
// too:
//
//	loader.connect(someUniqueID)
//
// When we want to send a message:
//
//	loader.send(someUniqueID, msg)
//
// Because we want to start rendering the charts *before* we send out any data,
// we can't use just the connection itself as an ID. We also can't use the
// userID because a user can have two tabs open. So, we need a connection ID.
type loaderT struct {
	conns *zcache2.Cache[zint.Uint128, *loaderClient]
}

var loader = loaderT{
	conns: zcache2.New[zint.Uint128, *loaderClient](zcache2.DefaultExpiration, zcache2.NoExpiration),
}

type loaderClient struct {
	sync.Mutex
	conn *websocket.Conn
}

func (l *loaderT) register(id zint.Uint128)   { l.conns.Set(id, nil) }
func (l *loaderT) unregister(id zint.Uint128) { l.conns.Delete(id) }

func (l *loaderT) connect(r *http.Request, id zint.Uint128, c *websocket.Conn) {
	c.SetCloseHandler(func(code int, text string) error {
		l.unregister(id)
		return nil
	})
	l.conns.Set(id, &loaderClient{conn: c})
}

func (l *loaderT) sendJSON(r *http.Request, id zint.Uint128, data any) {
	c, ok := l.conns.Get(id)
	if !ok {
		// No connection yet; this shouldn't happen, but does happen quite a lot
		// for bot requests and the like: they get the HTML, background stuff
		// starts spinning up, but don't run JS and will never establish a
		// websocket connection.
		//
		// So just ignore it; logging here will produce a ton of errors.
		return
	}
	if c == nil {
		// Wait for connection in cases where we send data before the frontend
		// established a connection.
		for i := 0; i < 1500; i++ {
			time.Sleep(10 * time.Millisecond)
			c, _ = l.conns.Get(id)
			if c != nil {
				break
			}
		}
		if c == nil {
			// Probably a bot or the like which doesn't support WebSockets.
			l.unregister(id)
			return
		}
	}

	c.Lock()
	defer c.Unlock()
	w, err := c.conn.NextWriter(websocket.TextMessage)
	if err != nil {
		if w != nil {
			w.Close()
		}
		if !strings.Contains(err.Error(), "use of closed network connection") {
			zlog.Fields(zlog.F{
				"connectID": id,
				"siteID":    Site(r.Context()).ID,
				"userID":    User(r.Context()).ID,
			}).FieldsRequest(r).Errorf("loader.send: NextWriter: %s", err)
		}
		return
	}

	j, err := json.Marshal(data)
	if err != nil {
		if !strings.Contains(err.Error(), "use of closed network connection") {
			zlog.Fields(zlog.F{
				"connectID": id,
				"siteID":    Site(r.Context()).ID,
				"userID":    User(r.Context()).ID,
			}).FieldsRequest(r).Errorf("loader.send: %s", err)
		}
		return
	}

	_, err = w.Write(j)
	w.Close()
	if err != nil {
		if !strings.Contains(err.Error(), "use of closed network connection") {
			zlog.Fields(zlog.F{
				"connectID": id,
				"siteID":    Site(r.Context()).ID,
				"userID":    User(r.Context()).ID,
			}).FieldsRequest(r).Errorf("loader.send: Write: %s", err)
		}
		return
	}
}

func (h backend) loader(w http.ResponseWriter, r *http.Request) error {
	ids := r.URL.Query().Get("id")
	if ids == "" {
		return fmt.Errorf("no id parameter")
	}
	id, err := zint.ParseUint128(ids, 16)
	if err != nil {
		return fmt.Errorf("id parameter: %w", err)
	}

	u := websocket.Upgrader{
		HandshakeTimeout:  10 * time.Second,
		ReadBufferSize:    4096,
		WriteBufferSize:   4096,
		EnableCompression: true,
	}
	c, err := u.Upgrade(w, r, nil)
	if err != nil {
		return err
	}

	loader.connect(r, id, c)

	// Read messages.
	go func() {
		defer zlog.Recover()
		for {
			t, m, err := c.ReadMessage()
			if err != nil {
				break
			}
			fmt.Println("websocket msg:", t, string(m))
		}
		c.Close()
	}()

	return nil
}
