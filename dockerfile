# syntax=docker/dockerfile:1
FROM cgr.dev/chainguard/go:latest as go-builder

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
COPY main.go ./

RUN go mod download

RUN CGO_ENABLED=0 go build -o /tmp/drenforce /app/main.go


FROM cgr.dev/chainguard/wolfi-base:latest

COPY --from=go-builder /tmp/drenforce /usr/bin/

RUN chmod 755 /usr/bin/drenforce

USER 1000

ENTRYPOINT ["drenforce"]