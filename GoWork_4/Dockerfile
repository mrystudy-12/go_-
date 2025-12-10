FROM golang:1.24-alpine AS builder
WORKDIR /app
ENV GOPROXY=https://goproxy.cn,direct
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/chat-server ./chat_server/main

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/chat-server .
EXPOSE 15000
CMD ["./chat-server"]