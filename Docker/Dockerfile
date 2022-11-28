FROM debian:bookworm-slim

WORKDIR /goatcounter

ENV GOATCOUNTER_VERSION 'v2.0.4'
ENV GOATCOUNTER_LISTEN '0.0.0.0:8080'
ENV GOATCOUNTER_DB 'sqlite:///goatcounter/db/goatcounter.sqlite3'
ENV GOATCOUNTER_SMTP ''

RUN apt-get update \
  && apt-get install -y ca-certificates wget \
  && update-ca-certificates --fresh

RUN wget "https://github.com/zgoat/goatcounter/releases/download/$GOATCOUNTER_VERSION/goatcounter-$GOATCOUNTER_VERSION-linux-amd64.gz" \
  && gzip -d "goatcounter-$GOATCOUNTER_VERSION-linux-amd64.gz" \
  && mv "goatcounter-$GOATCOUNTER_VERSION-linux-amd64" /usr/bin/goatcounter \
  && chmod +x /usr/bin/goatcounter

COPY goatcounter.sh ./
COPY entrypoint.sh /entrypoint.sh

VOLUME ["/goatcounter/db"]
EXPOSE 8080

ENTRYPOINT ["./entrypoint.sh"]
CMD ["./goatcounter.sh"]