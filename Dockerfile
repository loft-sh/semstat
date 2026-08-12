# Assembled by goreleaser from binaries it has already built. No RUN steps, so
# building the arm64 layer on an amd64 runner needs no emulation.
FROM gcr.io/distroless/static:nonroot@sha256:f7f8f729987ad0fdf6b05eeeae94b26e6a0f613bdf46feea7fc40f7bd72953e6

ARG TARGETPLATFORM
COPY $TARGETPLATFORM/semstat /usr/local/bin/semstat

ENTRYPOINT ["/usr/local/bin/semstat"]
