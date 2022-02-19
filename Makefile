VERSION := $(shell git log -n1 --format='%h_%cI')
VERSION_STAMP := -X zgo.at/goatcounter/v2.Version=${VERSION}

.PHONY:
build:
	go build -ldflags="${VERSION_STAMP}" ./cmd/goatcounter

without-cgo:
	CGO_ENABLED=0 $(MAKE) build

static:
	go build -tags osusergo,netgo,sqlite_omit_load_extension \
		-ldflags="${VERSION_STAMP} -extldflags=-static" \
		./cmd/goatcounter
