FROM golang:1.23-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go test ./...
RUN go build -o /out/coast-monitoring ./cmd/server
RUN go build -o /out/sync-cwa-marine ./cmd/sync-cwa-marine

FROM alpine:3.20

RUN apk add --no-cache ca-certificates
RUN addgroup -S app && adduser -S -G app app

WORKDIR /app
COPY --from=build --chown=app:app /out/coast-monitoring /app/coast-monitoring
COPY --from=build --chown=app:app /out/sync-cwa-marine /app/sync-cwa-marine
COPY --chown=app:app migrations /app/migrations
COPY --chown=app:app web /app/web

EXPOSE 8080

USER app

CMD ["/app/coast-monitoring"]
