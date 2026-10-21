# Assembled by goreleaser from binaries it has already built. No RUN steps, so
# building the arm64 layer on an amd64 runner needs no emulation.
FROM gcr.io/distroless/static:nonroot@sha256:d62067879de890a1def9da0e7766b7d20810a5e5555cef8506b752437d7a3e88

ARG TARGETPLATFORM
COPY $TARGETPLATFORM/semstat /usr/local/bin/semstat

ENTRYPOINT ["/usr/local/bin/semstat"]
