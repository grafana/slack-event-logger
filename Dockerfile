FROM golang:1.16 as builder

WORKDIR /app

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build .

FROM alpine:3.13

COPY --from=builder /app/slack-event-logger /app/slack-event-logger
ENTRYPOINT ["/app/slack-event-logger"]
