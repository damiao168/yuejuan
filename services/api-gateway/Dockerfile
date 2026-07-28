ARG GO_BUILD_IMAGE=golang:1.26.5-alpine
ARG API_RUNTIME_IMAGE=alpine:3.22

FROM ${GO_BUILD_IMAGE} AS builder

WORKDIR /src
COPY services/api-gateway/go.mod services/api-gateway/go.sum ./
RUN go mod download

COPY services/api-gateway/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/api-gateway ./cmd/api-gateway

FROM ${API_RUNTIME_IMAGE}

RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S -g 10001 edugrade \
    && adduser -S -D -H -u 10001 -G edugrade edugrade
WORKDIR /app
COPY --from=builder /out/api-gateway /usr/local/bin/api-gateway

USER edugrade

EXPOSE 8080
ENTRYPOINT ["api-gateway"]
