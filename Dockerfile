FROM golang:1.25-alpine AS build

WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/sarah-irene-dc-bot .

FROM alpine:3.22

RUN addgroup -S app && adduser -S -G app app
COPY --from=build /out/sarah-irene-dc-bot /usr/local/bin/sarah-irene-dc-bot

USER app
EXPOSE 8080
VOLUME ["/data"]
ENV BBOLT_PATH=/data/vc-sentry.db \
    SERVER_HTTP=:8080

ENTRYPOINT ["/usr/local/bin/sarah-irene-dc-bot"]
