# Packages a pre-built, statically-linked exam-server binary into a minimal OCI
# image. The binary must already exist at BINARY_PATH (default bin/exam-server)
# — it is cross-compiled by the CI matrix (or locally) before docker build, so
# this Dockerfile only copies it in.
#
# Build the binary first, then the image:
#
#   CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
#     go build -trimpath -ldflags="-s -w" -o bin/exam-server ./cmd/server
#   docker build --platform linux/amd64 -t exam-server .
#
# The base is scratch because the Go binary is fully static (CGO disabled) and
# the web assets are already embedded in the binary at compile time via
# //go:embed all:web/exam-lab/out, so nothing else is needed at runtime for the
# server to serve the UI. Exam documents and static assets are provided at run
# time via --load-exam / --load-exam-dir / --assets-dir (mount or bake them in).

FROM scratch

ARG BINARY_PATH=bin/exam-server

LABEL org.opencontainers.image.title="exam-server" \
      org.opencontainers.image.source="https://github.com/cmpxchg16b-nop/dcna-questions"

# Run as a non-root user (numeric UID; no /etc/passwd needed with scratch).
USER 65532:65532

COPY ${BINARY_PATH} /usr/local/bin/exam-server

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/exam-server"]
