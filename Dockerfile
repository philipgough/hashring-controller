FROM golang:1.20-bullseye AS builder

WORKDIR /usr/src/app

RUN useradd -u 1001 nonroot

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . .

ENV CGO_ENABLED=0

RUN go build \
    -o /usr/local/bin/controller \
    -ldflags "-w -s" \
    ./cmd/controller/main.go


FROM cgr.dev/chainguard/static:latest

COPY --from=builder /etc/passwd /etc/passwd
USER nonroot

COPY --from=builder /usr/local/bin/controller .

CMD ["./controller"]

