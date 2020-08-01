// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package cache

import (
	"time"

	"zgo.at/zcache"
)

// This is abstracted in a separate package so it'll be easy to add support for
// memcached or redis in the future.

type Cache interface {
	SetDefault(k string, x interface{})
	Get(k string) (interface{}, bool)
	Delete(k string)
	Flush()
}

func New(defaultExpiration, cleanupInterval time.Duration) Cache {
	return zcache.New(defaultExpiration, cleanupInterval)
}
