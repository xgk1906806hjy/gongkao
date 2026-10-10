FROM golang:1.23-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG CMD_PATH=./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/app ${CMD_PATH}

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /out/app /app/app
EXPOSE 8080
ENTRYPOINT ["/app/app"]