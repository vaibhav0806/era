FROM node:22-alpine

# Tools needed by the driver (git) and by Pi's bash tool for common repo ops.
RUN apk add --no-cache git bash ca-certificates coreutils

# Pin the Pi version at build time by passing --build-arg PI_VERSION=x.y.z.
# Default "latest" is OK for M1 — version is recorded in the image labels.
ARG PI_VERSION=latest
RUN npm install -g @mariozechner/pi-coding-agent@${PI_VERSION} \
    && pi --version > /pi-version.txt

# Pi session dir — tmpfs-backed at runtime via --tmpfs /tmp (optional);
# for M1 we just let it live in the image's /tmp (wiped on each `--rm`).
ENV PI_CODING_AGENT_DIR=/tmp/pi-state

# Go-compiled driver. Built by `make runner-linux`. Copied in here.
COPY bin/era-runner-linux /usr/local/bin/era-runner
RUN chmod +x /usr/local/bin/era-runner

WORKDIR /workspace

ENTRYPOINT ["/usr/local/bin/era-runner"]
