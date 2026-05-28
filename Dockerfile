FROM golang:1.23-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go test ./...
RUN go build -o /out/coast-monitoring ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates
RUN addgroup -S app && adduser -S -G app app

WORKDIR /app
COPY --from=build --chown=app:app /out/coast-monitoring /app/coast-monitoring
COPY --chown=app:app migrations /app/migrations
COPY --chown=app:app web /app/web

EXPOSE 8090

USER app

CMD ["/app/coast-monitoring"]
