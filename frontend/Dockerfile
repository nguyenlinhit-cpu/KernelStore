# Frontend KernelStore (Go SSR + templ + HTMX + Tailwind), cổng 8080.

# 1) Sinh code templ + build binary
FROM golang:1.27-alpine AS build
WORKDIR /src
RUN go install github.com/a-h/templ/cmd/templ@v0.3.1020
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN templ generate && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/kernelstore-web ./cmd/web

# 2) Build CSS bằng Tailwind v3 (khớp tailwind.config.js), quét cả file _templ.go vừa sinh
FROM node:22-alpine AS css
WORKDIR /src
RUN npm install --no-save --no-audit --no-fund tailwindcss@3
COPY --from=build /src/tailwind.config.js ./
COPY --from=build /src/internal ./internal
COPY --from=build /src/static ./static
RUN npx tailwindcss -c tailwind.config.js -i static/css/input.css -o static/css/app.css --minify

# 3) Runtime
FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata wget
WORKDIR /app
COPY --from=build /out/kernelstore-web /app/kernelstore-web
COPY --from=css /src/static /app/static
ENV WEB_ADDR=:8080 \
    STATIC_DIR=/app/static
EXPOSE 8080
ENTRYPOINT ["/app/kernelstore-web"]
