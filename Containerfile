FROM node:22-alpine AS frontend
WORKDIR /build
COPY package.json package-lock.json ./
RUN npm ci
COPY web/ ./web/
COPY vite.config.js ./
RUN npm run build

FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /build/pkg/tree/dist ./pkg/tree/dist
RUN CGO_ENABLED=0 go build -tags release -ldflags="-w -s" -o kompass cmd/kompass/*.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /build/kompass .
EXPOSE 8080
ENTRYPOINT ["/app/kompass"]
CMD ["--service", "0.0.0.0:8080"]
