#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="${HOME}/.local/bin"
INSTALL_PATH="${INSTALL_DIR}/agentic"
DATA_DIR="${AGENTIC_HOME:-${HOME}/.agentic}"

REMOVE=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --remove|-r) REMOVE=1; shift ;;
    *) echo "Unknown argument: $1" >&2; exit 1 ;;
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

if ! command -v docker &>/dev/null; then
  echo "Error: Docker is not installed or not on PATH." >&2
  exit 1
fi

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "${ARCH}" in
  x86_64)        ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) echo "Unsupported architecture: ${ARCH}" >&2; exit 1 ;;
esac

BINARY_SRC="${SCRIPT_DIR}/dist/agentic-${OS}-${ARCH}"

echo "Building agentic for ${OS}/${ARCH}..."
docker buildx build --build-arg "REPO_ROOT=${SCRIPT_DIR}" --target export --output "${SCRIPT_DIR}/dist/" "${SCRIPT_DIR}"

if [[ ! -f "${BINARY_SRC}" ]]; then
  echo "Error: expected binary not found at ${BINARY_SRC}" >&2
  exit 1
fi

mkdir -p "${INSTALL_DIR}"
cp "${BINARY_SRC}" "${INSTALL_PATH}"
chmod +x "${INSTALL_PATH}"
echo "Installed agentic to ${INSTALL_PATH}"

if ! echo "${PATH}" | grep -q "${HOME}/.local/bin"; then
  echo "Note: add ~/.local/bin to your PATH (e.g. export PATH=\"\${HOME}/.local/bin:\${PATH}\")"
fi
