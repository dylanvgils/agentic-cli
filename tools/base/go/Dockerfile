ARG BASE_IMAGE=agentic-base:latest
FROM ${BASE_IMAGE}

ARG GO_VERSION=1.26.3
ARG TARGETARCH

SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN \
  # Map Docker arch → Go arch
  case "${TARGETARCH}" in \
  amd64)  GO_ARCH=amd64  ;; \
  arm64)  GO_ARCH=arm64  ;; \
  arm)    GO_ARCH=armv6l ;; \
  *)      echo "Unsupported arch: ${TARGETARCH}" && exit 1 ;; \
  esac \
  \
  # Source os-release for PRETTY_NAME used in the install log
  && . /etc/os-release \
  \
  # Fetch checksum from the official API
  && EXPECTED_SHA=$(curl -fsSL "https://go.dev/dl/?mode=json" \
  | jq -r --arg ver "go${GO_VERSION}" \
  --arg arch "${GO_ARCH}" \
  '.[].files[] | select(.version == $ver and .os == "linux" and .arch == $arch and .kind == "archive") | .sha256') \
  \
  && echo "Installing Go ${GO_VERSION} on ${PRETTY_NAME} (${GO_ARCH})" \
  && echo "Expected SHA256: ${EXPECTED_SHA}" \
  \
  # Download and verify
  && TARBALL="go${GO_VERSION}.linux-${GO_ARCH}.tar.gz" \
  && curl -fsSL "https://go.dev/dl/${TARBALL}" -o /tmp/go.tar.gz \
  && echo "${EXPECTED_SHA}  /tmp/go.tar.gz" | sha256sum -c - \
  \
  # Install and clean up
  && tar -C /usr/local -xzf /tmp/go.tar.gz \
  && rm /tmp/go.tar.gz

ENV PATH="${PATH}:/usr/local/go/bin"

# Version script
RUN printf '#!/bin/sh\ngo version\n' > /usr/local/bin/agentic-version-go \
  && chmod +x /usr/local/bin/agentic-version-go
