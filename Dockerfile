# Build natively on the builder's arch and cross-compile, so multi-arch
# images don't need emulation.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
COPY web ./web
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/scratchpad . \
 && mkdir /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/scratchpad /scratchpad
# Owned by the nonroot user so a fresh named volume is writable.
COPY --from=build --chown=65532:65532 /out/data /data
ENV SCRATCHPAD_DATA_DIR=/data
VOLUME /data
EXPOSE 63001
ENTRYPOINT ["/scratchpad"]
