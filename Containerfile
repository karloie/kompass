# syntax=docker/dockerfile:1.7

# ---------- FRONTEND BUILDER ----------
FROM node:22-alpine3.20 AS frontend
WORKDIR /build
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY web/ ./web/
COPY vite.config.js ./
RUN npm run build


# ---------- BACKEND BUILDER ----------
FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	go mod download
COPY . .
COPY --from=frontend /build/pkg/tree/dist ./pkg/tree/dist
ARG BUILD_VERSION=dev
ARG BUILD_COMMIT=none
ARG BUILD_DATE=unknown
RUN --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	CGO_ENABLED=0 go build -tags release -ldflags="-w -s -X main.buildVersion=${BUILD_VERSION} -X main.buildCommit=${BUILD_COMMIT} -X main.buildDate=${BUILD_DATE}" -o kompass cmd/kompass/*.go


# ---------- CILIUM BUILDER ----------
FROM alpine:3.20 AS cilium
ARG CILIUM_CLI_VERSION=v0.19.2
ARG HUBBLE_VERSION=v1.18.6
RUN apk --no-cache add curl
RUN \
	CLI_ARCH=amd64; \
	if [ "$(uname -m)" = "aarch64" ]; then CLI_ARCH=arm64; fi && \
	curl -L --fail --remote-name-all https://github.com/cilium/cilium-cli/releases/download/${CILIUM_CLI_VERSION}/cilium-linux-${CLI_ARCH}.tar.gz{,.sha256sum} && \
	sha256sum -c cilium-linux-${CLI_ARCH}.tar.gz.sha256sum && \
	tar -xvf cilium-linux-${CLI_ARCH}.tar.gz -C /usr/local/bin
RUN \
	HUBBLE_ARCH=amd64; \
	if [ "$(uname -m)" = "aarch64" ]; then HUBBLE_ARCH=arm64; fi && \
	curl -L --fail --remote-name-all https://github.com/cilium/hubble/releases/download/$HUBBLE_VERSION/hubble-linux-${HUBBLE_ARCH}.tar.gz{,.sha256sum} && \
	sha256sum -c hubble-linux-${HUBBLE_ARCH}.tar.gz.sha256sum && \
	tar -xvf hubble-linux-${HUBBLE_ARCH}.tar.gz -C /usr/local/bin

	
# ---------- RUNTIME IMAGE ----------
FROM alpine:3.20
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder --chmod=0555 /build/kompass /app/kompass
COPY --from=cilium --chmod=0555 /usr/local/bin/cilium /usr/local/bin/cilium
COPY --from=cilium --chmod=0555 /usr/local/bin/hubble /usr/local/bin/hubble
RUN addgroup -g 1000 -S kompass \
	&& adduser -u 1000 -S -D -G kompass kompass \
	&& chown 1000:1000 /app
USER 1000:1000
EXPOSE 8080
ENTRYPOINT ["/app/kompass"]
CMD ["--service", "0.0.0.0:8080"]
