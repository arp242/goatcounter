# syntax=docker/dockerfile:1.10

### Build GoatCounter
from docker.io/golang:latest as build
workdir /goatcounter
env CGO_ENABLED=1
env GOTOOLCHAIN=auto
copy go.mod go.sum ./
run --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	go mod download
copy . .
run --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	go build -trimpath -ldflags='-s -w -extldflags=-static' \
	-tags='osusergo,netgo,sqlite_omit_load_extension' \
	./cmd/goatcounter

### Build container
from docker.io/alpine
copy --from=build /goatcounter/goatcounter /bin/goatcounter
run <<EOF
	set -euC

	addgroup goatcounter
	adduser -DG goatcounter goatcounter

	mkdir /home/goatcounter/goatcounter-data
	chown goatcounter:goatcounter /home/goatcounter/goatcounter-data
EOF

expose     80 443 8080
workdir    /home/goatcounter
user       goatcounter:goatcounter
volume     ["/home/goatcounter/goatcounter-data"]
entrypoint ["goatcounter"]
cmd        ["serve", "-automigrate"]
