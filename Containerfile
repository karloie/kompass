FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o kompass cmd/kompass/*.go

FROM alpine:latest AS tools
ARG TARGETOS=linux
ARG TARGETARCH
RUN apk --no-cache add ca-certificates curl tar
RUN set -eux; \
	case "$TARGETARCH" in \
		amd64) ARCH=amd64 ;; \
		arm64) ARCH=arm64 ;; \
		*) echo "unsupported TARGETARCH: $TARGETARCH"; exit 1 ;; \
	esac; \
	KUBECTL_VERSION="$(curl -fsSL https://dl.k8s.io/release/stable.txt)"; \
	curl -fsSL "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/${TARGETOS}/${ARCH}/kubectl" -o /usr/local/bin/kubectl; \
	chmod +x /usr/local/bin/kubectl; \
	CILIUM_VERSION="$(curl -fsSL https://raw.githubusercontent.com/cilium/cilium-cli/main/stable.txt)"; \
	curl -fsSL "https://github.com/cilium/cilium-cli/releases/download/${CILIUM_VERSION}/cilium-${TARGETOS}-${ARCH}.tar.gz" -o /tmp/cilium.tar.gz; \
	tar -xzf /tmp/cilium.tar.gz -C /usr/local/bin cilium hubble; \
	chmod +x /usr/local/bin/cilium /usr/local/bin/hubble

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /build/kompass .
COPY --from=tools /usr/local/bin/kubectl /usr/local/bin/kubectl
COPY --from=tools /usr/local/bin/cilium /usr/local/bin/cilium
COPY --from=tools /usr/local/bin/hubble /usr/local/bin/hubble
EXPOSE 8080
ENTRYPOINT ["/app/kompass"]
CMD ["--service", "0.0.0.0:8080"]
