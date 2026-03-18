# ---------- FRONTEND BUILDER ----------
FROM node:22-alpine AS frontend
WORKDIR /build
COPY package.json package-lock.json ./
RUN npm ci
COPY web/ ./web/
COPY vite.config.js ./
RUN npm run build


# ---------- BACKEND BUILDER ----------
FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /build/pkg/tree/dist ./pkg/tree/dist
RUN CGO_ENABLED=0 go build -tags release -ldflags="-w -s" -o kompass cmd/kompass/*.go


# ---------- CILIUM BUILDER ----------
FROM alpine:latest AS cilium
RUN apk --no-cache add coreutils curl
RUN CILIUM_CLI_VERSION=$(curl -s https://raw.githubusercontent.com/cilium/cilium-cli/main/stable.txt) \
	CLI_ARCH=amd64; \
	if [ "$(uname -m)" = "aarch64" ]; then CLI_ARCH=arm64; fi && \
	curl -L --fail --remote-name-all https://github.com/cilium/cilium-cli/releases/download/${CILIUM_CLI_VERSION}/cilium-linux-${CLI_ARCH}.tar.gz{,.sha256sum} && \
	sha256sum --check cilium-linux-${CLI_ARCH}.tar.gz.sha256sum && \
	tar -xvf cilium-linux-${CLI_ARCH}.tar.gz -C /usr/local/bin
RUN HUBBLE_VERSION=$(curl -s https://raw.githubusercontent.com/cilium/hubble/main/stable.txt) \
	HUBBLE_ARCH=amd64; \
	if [ "$(uname -m)" = "aarch64" ]; then HUBBLE_ARCH=arm64; fi && \
	curl -L --fail --remote-name-all https://github.com/cilium/hubble/releases/download/$HUBBLE_VERSION/hubble-linux-${HUBBLE_ARCH}.tar.gz{,.sha256sum} && \
	sha256sum --check hubble-linux-${HUBBLE_ARCH}.tar.gz.sha256sum && \
	tar -xvf hubble-linux-${HUBBLE_ARCH}.tar.gz -C /usr/local/bin

	
# ---------- RUNTIME IMAGE ----------
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /build/kompass .
COPY --from=cilium /usr/local/bin/cilium /usr/local/bin/cilium
COPY --from=cilium /usr/local/bin/hubble /usr/local/bin/hubble
RUN addgroup -g 1000 -S kompass \
	&& adduser -u 1000 -S -D -G kompass kompass \
	&& chown -R 1000:1000 /app
USER 1000:1000
EXPOSE 8080
ENTRYPOINT ["/app/kompass"]
CMD ["--service", "0.0.0.0:8080"]
