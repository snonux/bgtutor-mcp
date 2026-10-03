# Container image for the bgtutor MCP server.
# Build from the repository root (the context must include go.mod):
#   docker build -t bgtutor:0.30.0 .
#
# Only `bgtutor serve` runs in the container. Episodes are prepared offline by
# a coding agent following PREPARE.md and copied into the mounted data
# directory.
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# The server has no cgo dependencies, so a static binary works on a minimal
# base image.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/bgtutor ./cmd/bgtutor

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/bgtutor /usr/local/bin/bgtutor
# /data holds episodes/, citizenship-test/ and personal vocabulary/progress
# (see FORMAT.md and CITIZENSHIP.md). Mount a volume so learning data persists.
# Set BGTUTOR_MODE=citizenship to select citizenship preparation.
ENV BGTUTOR_DATA_DIR=/data BGTUTOR_ADDR=:8080
EXPOSE 8080
# BGTUTOR_TOKEN must be set: serve refuses a non-localhost address without it.
ENTRYPOINT ["/usr/local/bin/bgtutor", "serve"]
