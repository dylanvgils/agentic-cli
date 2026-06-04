ARG GO_VERSION=1.26.3
ARG TARGETARCH
ARG INSTALL_METHOD=script

FROM debian:bookworm-slim AS builder

ARG GO_VERSION
ARG TARGETARCH

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

RUN apt-get update \
  && apt-get install -y --no-install-recommends \
  ca-certificates curl git jq make \
  && rm -rf /var/lib/apt/lists/*

RUN \
  # Detect arch: use TARGETARCH (buildx) or fall back to uname -m
  _ARCH="${TARGETARCH:-$(uname -m)}" \
  \
  # Map Docker/uname arch → Go arch
  && case "${_ARCH}" in \
  amd64|x86_64)   GO_ARCH=amd64  ;; \
  arm64|aarch64)  GO_ARCH=arm64  ;; \
  arm)            GO_ARCH=armv6l ;; \
  *)              echo "Unsupported arch: ${_ARCH}" && exit 1 ;; \
  esac \
  \
  # Source os-release for PRETTY_NAME used in the install log
  && . /etc/os-release \
  \
  # Fetch checksum from the official API
  && EXPECTED_SHA=$(curl -fsSL "https://go.dev/dl/?mode=json&include=all" \
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

WORKDIR /src
COPY . .
ARG INSTALL_METHOD
RUN make dist INSTALL_METHOD="${INSTALL_METHOD}"

FROM scratch AS export
COPY --from=builder /src/dist/ /
