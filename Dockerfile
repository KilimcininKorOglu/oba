FROM golang:1.22-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git make

COPY go.mod ./
COPY . .

RUN make build

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/bin/oba /usr/local/bin/oba
COPY config.example.yaml /app/config.example.yaml
COPY acl.example.yaml /app/acl.example.yaml

RUN adduser -D -H -s /sbin/nologin oba && \
    mkdir -p /var/lib/oba /var/log/oba && \
    chown -R oba:oba /var/lib/oba /var/log/oba /app

USER oba

EXPOSE 389 636 8080

ENTRYPOINT ["oba"]
CMD ["serve", "--config", "/app/config.yaml"]
