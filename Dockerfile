# Assembled by goreleaser from binaries it has already built. No RUN steps, so
# building the arm64 layer on an amd64 runner needs no emulation.
FROM gcr.io/distroless/static:nonroot@sha256:25ac8f0c3b2334a9eb05452266bd4697864a8c8ef0ef71e5e6480d7f92868329

ARG TARGETPLATFORM
COPY $TARGETPLATFORM/semstat /usr/local/bin/semstat

ENTRYPOINT ["/usr/local/bin/semstat"]
