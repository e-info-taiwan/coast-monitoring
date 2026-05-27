FROM golang:1.23-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go test ./...
RUN go build -o /out/coast-monitoring ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=build /out/coast-monitoring /app/coast-monitoring
COPY migrations /app/migrations
COPY web /app/web

EXPOSE 8090

CMD ["/app/coast-monitoring"]
