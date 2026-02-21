FROM golang:1.22-alpine

WORKDIR /app

RUN apk add --no-cache git make

COPY go.mod ./
COPY . .

RUN make build

EXPOSE 389 636 8080

CMD ["./bin/oba", "serve"]
