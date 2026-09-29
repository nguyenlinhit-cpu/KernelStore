# Backend KernelStore (Go 1.27) — REST API + WebSocket, cổng 5000.
# Migration được nhúng trong binary, tự chạy khi khởi động.

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/kernelstore-api ./cmd/api

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata wget
WORKDIR /app
COPY --from=build /out/kernelstore-api /app/kernelstore-api
# Ảnh mẫu có sẵn; volume gắn vào đây sẽ được khởi tạo từ nội dung này ở lần chạy đầu.
COPY uploads /app/uploads
ENV SERVER_ADDR=:5000 \
    UPLOAD_DIR=/app/uploads
EXPOSE 5000
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=5 \
  CMD wget -qO- http://127.0.0.1:5000/api/categories >/dev/null || exit 1
# `docker compose run --rm backend seed` để seed dữ liệu mẫu.
ENTRYPOINT ["/app/kernelstore-api"]
