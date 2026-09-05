#!/usr/bin/env sh
set -eu
compose_file="$(CDPATH= cd -- "$(dirname -- "$0")/../infra/docker-compose" && pwd)/docker-compose.yml"
docker compose -f "$compose_file" --profile ocr up -d postgres redis minio api-gateway ocr-worker
