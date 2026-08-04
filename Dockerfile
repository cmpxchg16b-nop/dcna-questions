# Multi-stage build for the exam-server.
#
# The builder always runs on the build host's native platform ($BUILDPLATFORM)
# and uses Go's built-in cross-compilation — via the TARGETOS/TARGETARCH args
# that buildx injects — to emit a static binary for each requested target. This
# means multi-arch builds (linux/amd64 + linux/arm64) need no QEMU emulation:
# the builder runs natively every time and Go handles the arch.
#
# The web assets (web/exam-lab/out) must exist in the build context; they are
# embedded into the binary at compile time via //go:embed all:web/exam-lab/out.
# Build them first (cd web/exam-lab && npm ci && npm run build) or let CI do it.
#
# Build a multi-arch image:
#   docker buildx build --platform linux/amd64,linux/arm64 -t exam-server .

# --- Builder -----------------------------------------------------------------
FROM --platform=$BUILDPLATFORM golang:1.24 AS builder

WORKDIR /src

# Module files first so go mod download is cached across source changes.
COPY go.mod go.sum ./
RUN go mod download

# Rest of the source, including web/exam-lab/out for the embed.
COPY . .

# Declared here (after go mod download) so preceding layers stay cached across
# target architectures.
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /exam-server ./cmd/server

# --- Runtime -----------------------------------------------------------------
FROM scratch

LABEL org.opencontainers.image.title="exam-server" \
      org.opencontainers.image.source="https://github.com/cmpxchg16b-nop/dcna-questions"

WORKDIR /app

# Run as a non-root user (numeric UID; no /etc/passwd needed with scratch).
USER 65532:65532

COPY --from=builder /exam-server /usr/local/bin/exam-server

# Ship the exam documents and static assets alongside the binary so the server
# starts with sensible defaults. Paths are relative to WORKDIR (/app).
COPY assets/ ./assets/
COPY exams/ ./exams/

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/exam-server", "--assets-dir=assets", "--load-exam-dir=exams"]
