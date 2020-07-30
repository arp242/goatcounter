// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package cache

import (
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"zgo.at/zcache"
)

type Cache interface {
	SetDefault(k string, x interface{})
	Get(k string) (interface{}, bool)
}

func New(defaultExpiration, cleanupInterval time.Duration) Cache {
	//if false {
	//	return &memcached{mc: memcache.New("10.0.0.1:11211", "10.0.0.2:11211")}
	//}
	return &local{cache: zcache.New(defaultExpiration, cleanupInterval)}
}

// Simple local memory cache.
type local struct{ cache *zcache.Cache }

func (c *local) Get(k string) (interface{}, bool)   { return c.cache.Get(k) }
func (c *local) SetDefault(k string, x interface{}) { c.cache.SetDefault(k, x) }

// Memcached cache.
type memcached struct{ mc *memcache.Client }

func (c *memcached) Get(k string) (interface{}, bool) {
	// TODO: unserialize this.
	item, err := c.mc.Get("foo")
	return item, err == nil
}
func (c *memcached) SetDefault(k string, x interface{}) {
	c.mc.Set(&memcache.Item{
		Key: k,
		// TODO: serialize this.
		Value: []byte("my value"),
	})
}
