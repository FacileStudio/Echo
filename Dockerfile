FROM oven/bun:1 AS client-build
WORKDIR /client
ARG VITE_API_BASE=""
ENV VITE_API_BASE=$VITE_API_BASE
COPY apps/client/package.json apps/client/bun.lock* ./
RUN bun install --frozen-lockfile
COPY apps/client/ .
RUN bun run build

FROM alpine:3.22 AS models
RUN apk add --no-cache curl unzip
COPY scripts/vosk-models.sh /vosk-models.sh
RUN sh /vosk-models.sh /models

FROM golang:1.26-alpine AS api-build

ARG TARGETOS=linux
ARG TARGETARCH

WORKDIR /repo/apps/api

COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download

COPY apps/api ./
COPY apps/client/package.json /repo/apps/client/package.json
COPY scripts/version.sh /repo/scripts/version.sh

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w -X main.version=$(sh /repo/scripts/version.sh)" -o bin/api .

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=api-build /repo/apps/api/bin/api /api
COPY --from=client-build /client/build /client
COPY --from=models /models /client/models

ENV CLIENT_DIR=/client
ENV PORT=4020

EXPOSE 4020

USER nonroot:nonroot

ENTRYPOINT ["/api"]
