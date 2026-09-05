# Assembled by goreleaser from binaries it has already built. No RUN steps, so
# building the arm64 layer on an amd64 runner needs no emulation.
FROM gcr.io/distroless/static:nonroot@sha256:1c2c046bc09ed40fad370b599a0b1ae7987f55b01e247cf27a7c27cd97e5bbc7

ARG TARGETPLATFORM
COPY $TARGETPLATFORM/semstat /usr/local/bin/semstat

ENTRYPOINT ["/usr/local/bin/semstat"]
