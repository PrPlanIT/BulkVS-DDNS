FROM docker.io/library/golang:1.27.1-alpine3.24 AS builder
WORKDIR /src
COPY go.mod ./
COPY go.sum* ./
COPY . .
RUN go mod tidy
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -ldflags="-s -w \
      -X github.com/PrPlanIT/bulkvs-ddns/src/version.Version=${VERSION} \
      -X github.com/PrPlanIT/bulkvs-ddns/src/version.Commit=${COMMIT} \
      -X github.com/PrPlanIT/bulkvs-ddns/src/version.BuildDate=${BUILD_DATE}" \
    -o /bulkvs-ddns ./cmd/bulkvs-ddns

FROM scratch
LABEL maintainer="PrPlanIT <precisionplanit@gmail.com>" \
      org.opencontainers.image.title="bulkvs-ddns" \
      org.opencontainers.image.description="Keeps a BulkVS IP-based-auth host (/ipHost) in sync with the site's current public IP — DDNS for a BulkVS SIP trunk" \
      org.opencontainers.image.source="https://github.com/PrPlanIT/bulkvs-ddns" \
      org.opencontainers.image.url="https://hub.docker.com/r/prplanit/bulkvs-ddns" \
      org.opencontainers.image.documentation="https://github.com/PrPlanIT/bulkvs-ddns#readme" \
      org.opencontainers.image.vendor="PrPlanIT"
# TLS roots for the BulkVS API + the trace endpoint; the static binary is all else.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /bulkvs-ddns /bulkvs-ddns
USER 65532:65532
ENTRYPOINT ["/bulkvs-ddns"]
