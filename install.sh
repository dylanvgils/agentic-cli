#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="${HOME}/.local/bin"
INSTALL_PATH="${INSTALL_DIR}/agentic"
DATA_DIR="${AGENTIC_HOME:-${HOME}/.agentic}"

REMOVE=0
FROM_SOURCE=0
while [[ $# -gt 0 ]]; do
  case "$1" in
  --remove | -r)
    REMOVE=1
    shift
    ;;
  --from-source)
    FROM_SOURCE=1
    shift
    ;;
  *)
    echo "Unknown argument: $1" >&2
    exit 1
    ;;
  esac
done

if [[ "${REMOVE}" -eq 1 ]]; then
  if [[ -f "${INSTALL_PATH}" ]]; then
    rm -f "${INSTALL_PATH}"
    echo "Removed ${INSTALL_PATH}"
  fi

  if [[ -d "${DATA_DIR}" ]]; then
    read -r -p "Remove data directory ${DATA_DIR}? [y/N] " confirm
    if [[ "${confirm}" =~ ^[Yy]$ ]]; then
      rm -rf "${DATA_DIR}"
      echo "Removed ${DATA_DIR}"
    fi
  fi

  exit 0
fi

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "${ARCH}" in
x86_64) ARCH=amd64 ;;
aarch64 | arm64) ARCH=arm64 ;;
*)
  echo "Unsupported architecture: ${ARCH}" >&2
  exit 1
  ;;
esac

if [[ "${FROM_SOURCE}" -eq 1 ]]; then
  INSTALL_METHOD="script-sh"

  if ! command -v docker &>/dev/null; then
    echo "Error: Docker is not installed or not on PATH." >&2
    exit 1
  fi

  BINARY_SRC="${SCRIPT_DIR}/dist/agentic-${OS}-${ARCH}"

  echo "Building agentic for ${OS}/${ARCH}..."
  docker buildx build \
    --target export \
    --build-arg "INSTALL_METHOD=${INSTALL_METHOD}" \
    --output "${SCRIPT_DIR}/dist/" \
    "${SCRIPT_DIR}"

  if [[ ! -f "${BINARY_SRC}" ]]; then
    echo "Error: expected binary not found at ${BINARY_SRC}" >&2
    exit 1
  fi

  mkdir -p "${INSTALL_DIR}"
  cp "${BINARY_SRC}" "${INSTALL_PATH}"
  chmod +x "${INSTALL_PATH}"
  echo "Installed agentic to ${INSTALL_PATH}"
else
  if ! command -v curl &>/dev/null; then
    echo "Error: curl is not installed or not on PATH." >&2
    exit 1
  fi

  echo "Fetching latest release..."
  VERSION=$(curl -fsSL https://api.github.com/repos/dylanvgils/agentic-cli/releases/latest |
    grep '"tag_name"' |
    sed 's/.*"tag_name": *"v\([^"]*\)".*/\1/')

  if [[ -z "${VERSION}" ]]; then
    echo "Error: failed to fetch latest release version." >&2
    exit 1
  fi

  ARCHIVE="agentic-${VERSION}-${OS}-${ARCH}.tar.gz"
  URL="https://github.com/dylanvgils/agentic-cli/releases/download/v${VERSION}/${ARCHIVE}"
  CHECKSUMS_URL="https://github.com/dylanvgils/agentic-cli/releases/download/v${VERSION}/checksums.txt"

  TMPDIR=$(mktemp -d)
  trap 'rm -rf "${TMPDIR}"' EXIT

  echo "Downloading agentic ${VERSION} for ${OS}/${ARCH}..."
  curl -fsSL "${URL}" -o "${TMPDIR}/${ARCHIVE}"
  curl -fsSL "${CHECKSUMS_URL}" -o "${TMPDIR}/checksums.txt"

  echo "Verifying checksum..."
  EXPECTED=$(grep " ${ARCHIVE}$" "${TMPDIR}/checksums.txt" | awk '{print $1}')
  if [[ -z "${EXPECTED}" ]]; then
    echo "Error: checksum not found for ${ARCHIVE}." >&2
    exit 1
  fi

  if command -v sha256sum &>/dev/null; then
    ACTUAL=$(sha256sum "${TMPDIR}/${ARCHIVE}" | awk '{print $1}')
  elif command -v shasum &>/dev/null; then
    ACTUAL=$(shasum -a 256 "${TMPDIR}/${ARCHIVE}" | awk '{print $1}')
  else
    echo "Warning: cannot verify checksum, neither sha256sum nor shasum found." >&2
    ACTUAL="${EXPECTED}"
  fi

  if [[ "${ACTUAL}" != "${EXPECTED}" ]]; then
    echo "Error: checksum mismatch for ${ARCHIVE}." >&2
    exit 1
  fi

  tar -xzf "${TMPDIR}/${ARCHIVE}" -C "${TMPDIR}"

  mkdir -p "${INSTALL_DIR}"
  cp "${TMPDIR}/agentic" "${INSTALL_PATH}"
  chmod +x "${INSTALL_PATH}"
  echo "Installed agentic to ${INSTALL_PATH}"
fi

if ! echo "${PATH}" | grep -q "${HOME}/.local/bin"; then
  echo "Note: add ~/.local/bin to your PATH (e.g. export PATH=\"\${HOME}/.local/bin:\${PATH}\")"
fi
