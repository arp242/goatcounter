FROM golang:1-bullseye as go

COPY . /go/src/goatcounter
WORKDIR /go/src/goatcounter

RUN CGO_ENABLED=0 \
    go build -tags osusergo,netgo,sqlite_omit_load_extension \
    -ldflags="-X zgo.at/goatcounter/v2.Version=$(git log -n1 --format='%h_%cI') -extldflags=-static"\
    ./cmd/goatcounter

FROM scratch
WORKDIR /
COPY --from=go /go/src/goatcounter/goatcounter /bin/goatcounter

ENTRYPOINT ["/bin/goatcounter"]
CMD ["serve"]
