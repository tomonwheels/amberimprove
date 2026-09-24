# amberIMPROVE as a container — for NAS systems without systemd (Synology
# Container Manager, QNAP Container Station, ZimaOS, Unraid, …).
#
# The build expects the static bufhrt in build/bufhrt/bufhrt-linux-<arch>
# (see scripts/package.sh).
FROM golang:1.24-bookworm AS bau
ARG TARGETARCH
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -trimpath \
      -ldflags "-s -w -X main.version=$VERSION" -o /out/amberimprove ./cmd/amberimprove
RUN install -m 755 build/bufhrt/bufhrt-linux-$TARGETARCH /out/bufhrt

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
      rsync openssh-client util-linux tzdata ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=bau /out/amberimprove /opt/amberimprove/bin/amberimprove
COPY --from=bau /out/bufhrt /opt/amberimprove/bin/bufhrt
COPY LICENSE README.md /opt/amberimprove/
ENV AMBERIMPROVE_CONFIG=/config/config.json
EXPOSE 8093
VOLUME ["/config"]
ENTRYPOINT ["/opt/amberimprove/bin/amberimprove"]
CMD ["all", "-listen", ":8093"]
