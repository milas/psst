# syntax=docker/dockerfile:1
FROM --platform=${BUILDPLATFORM} golang:1.20 AS base

ARG TARGETOS
ARG TARGETARCH
ENV GOOS=${TARGETOS}
ENV GOARCH=${TARGETARCH}

WORKDIR /src

FROM base AS deps
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod download

FROM deps AS build
COPY . .

ENV CGO_ENABLED=0
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build -trimpath -ldflags='-w' -o /out/psst .

FROM scratch AS psst-bin

COPY --from=build --link --chmod=0755 /out/psst /

FROM gcr.io/distroless/base-debian11 AS psst

COPY --from=build --link --chmod=0755 /out/psst /

ENTRYPOINT ["/psst"]
