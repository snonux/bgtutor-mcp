# Container image for the bgtutor MCP server.
# Build from the repository root (the context must include go.mod):
#   docker build -t bgtutor:0.30.0 .
#
# Only `bgtutor serve` runs in the container. Episodes are prepared offline by
# a coding agent following PREPARE.md and copied into the mounted data
# directory.
FROM docker.io/library/golang:1.27.3-alpine@sha256:fa3d596898d9f126af7c01d14ee7597bab3f39cccf392ef36b69fe77cd681f18 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# The server has no cgo dependencies, so a static binary works on a minimal
# base image.
RUN GOMAXPROCS=2 CGO_ENABLED=0 go build -p 2 -trimpath -ldflags="-s -w" -o /out/bgtutor ./cmd/bgtutor

# Build the test target explicitly before publishing a runtime image.
FROM build AS test
RUN apk add --no-cache gcc musl-dev
RUN GOMAXPROCS=2 CGO_ENABLED=1 go test -p 2 -race ./...

FROM gcr.io/distroless/static-debian13:nonroot@sha256:25ac8f0c3b2334a9eb05452266bd4697864a8c8ef0ef71e5e6480d7f92868329 AS runtime
COPY --from=build /out/bgtutor /usr/local/bin/bgtutor
# /data holds episodes/, citizenship-test/ and personal vocabulary/progress
# (see FORMAT.md and CITIZENSHIP.md). Mount a volume so learning data persists.
# Set BGTUTOR_MODE=citizenship to select citizenship preparation.
ENV BGTUTOR_DATA_DIR=/data BGTUTOR_ADDR=:8080
EXPOSE 8080
# BGTUTOR_TOKEN must be set: serve refuses a non-localhost address without it.
ENTRYPOINT ["/usr/local/bin/bgtutor", "serve"]
