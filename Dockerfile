# Assembled by goreleaser from binaries it has already built. No RUN steps, so
# building the arm64 layer on an amd64 runner needs no emulation.
FROM gcr.io/distroless/static:nonroot@sha256:8ce6dd87e9823c5b48dcfb2c8604714ccb351ce59607327bcc4957448da279b5

ARG TARGETPLATFORM
COPY $TARGETPLATFORM/semstat /usr/local/bin/semstat

ENTRYPOINT ["/usr/local/bin/semstat"]
