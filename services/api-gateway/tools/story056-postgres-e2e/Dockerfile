FROM golang:1.24-alpine

WORKDIR /src
COPY services/api-gateway/go.mod services/api-gateway/go.sum ./
RUN go mod download
COPY services/api-gateway/ ./

CMD ["go", "test", "-count=1", "-v", "./internal/server"]
