FROM golang:alpine AS builder
RUN apk update && apk add --no-cache git
WORKDIR /app
COPY go.mod ./
RUN GOPROXY=direct go mod download
COPY . .
RUN GOPROXY=direct CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o main .

FROM alpine
WORKDIR /root/
COPY --from=builder /app/main .
# COPY --from=builder /app/.env .
EXPOSE 3002
CMD ["./main"]
