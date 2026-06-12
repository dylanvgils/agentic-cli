#!/usr/bin/env bash
set -euo pipefail

# --- Variables ---

INSTALL_DIR="${HOME}/.local/bin"
INSTALL_PATH="${INSTALL_DIR}/agentic"
DATA_DIR="${AGENTIC_HOME:-${HOME}/.agentic}"

# --- Functions ---

install_from_source() {
  if ! command -v docker &>/dev/null; then
    echo "Error: Docker is not installed or not on PATH." >&2
    exit 1
  fi

  local script_dir
  script_dir="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
  local binary_src="${script_dir}/dist/agentic-${OS}-${ARCH}"

  echo "Building agentic for ${OS}/${ARCH}..."
  docker buildx build \
    --target export \
    --build-arg "INSTALL_METHOD=script-sh" \
    --output "${script_dir}/dist/" \
    "${script_dir}"

  if [[ ! -f "${binary_src}" ]]; then
    echo "Error: expected binary not found at ${binary_src}" >&2
    exit 1
  fi

  mkdir -p "${INSTALL_DIR}"
  cp "${binary_src}" "${INSTALL_PATH}"
  chmod +x "${INSTALL_PATH}"
  echo "Installed agentic to ${INSTALL_PATH}"
}

install_from_release() {
  if ! command -v curl &>/dev/null; then
    echo "Error: curl is not installed or not on PATH." >&2
    exit 1
  fi

  echo "Fetching latest release..."
  local version
  version=$(curl -fsSL https://api.github.com/repos/dylanvgils/agentic-cli/releases/latest |
    grep '"tag_name"' |
    sed 's/.*"tag_name": *"v\([^"]*\)".*/\1/')

  if [[ -z "${version}" ]]; then
    echo "Error: failed to fetch latest release version." >&2
    exit 1
  fi

  local archive="agentic-${version}-${OS}-${ARCH}.tar.gz"
  local url="https://github.com/dylanvgils/agentic-cli/releases/download/v${version}/${archive}"
  local checksums_url="https://github.com/dylanvgils/agentic-cli/releases/download/v${version}/checksums.txt"

  local tmpdir
  tmpdir=$(mktemp -d)
  # shellcheck disable=SC2064 # tmpdir is local; must expand now, not when trap fires
  trap "rm -rf '${tmpdir}'" EXIT

  echo "Downloading agentic ${version} for ${OS}/${ARCH}..."
  curl -fsSL "${url}" -o "${tmpdir}/${archive}"
  curl -fsSL "${checksums_url}" -o "${tmpdir}/checksums.txt"

  echo "Verifying checksum..."
  local expected
  expected=$(grep " ${archive}$" "${tmpdir}/checksums.txt" | awk '{print $1}')
  if [[ -z "${expected}" ]]; then
    echo "Error: checksum not found for ${archive}." >&2
    exit 1
  fi

  local actual
  if command -v sha256sum &>/dev/null; then
    actual=$(sha256sum "${tmpdir}/${archive}" | awk '{print $1}')
  elif command -v shasum &>/dev/null; then
    actual=$(shasum -a 256 "${tmpdir}/${archive}" | awk '{print $1}')
  else
    echo "Warning: cannot verify checksum, neither sha256sum nor shasum found." >&2
    actual="${expected}"
  fi

  if [[ "${actual}" != "${expected}" ]]; then
    echo "Error: checksum mismatch for ${archive}." >&2
    exit 1
  fi

  tar -xzf "${tmpdir}/${archive}" -C "${tmpdir}"

  mkdir -p "${INSTALL_DIR}"
  cp "${tmpdir}/agentic" "${INSTALL_PATH}"
  chmod +x "${INSTALL_PATH}"
  echo "Installed agentic to ${INSTALL_PATH}"
}

# --- Main ---

main() {
  local remove=0
  local from_source=0

  while [[ $# -gt 0 ]]; do
    case "$1" in
    --remove | -r)
      remove=1
      shift
      ;;
    --from-source)
      from_source=1
      shift
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 1
      ;;
    esac
  done

  if [[ "${remove}" -eq 1 ]]; then
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

  if [[ "${from_source}" -eq 1 ]]; then
    install_from_source
  else
    install_from_release
  fi

  if ! echo "${PATH}" | grep -q "${HOME}/.local/bin"; then
    echo "Note: add ~/.local/bin to your PATH (e.g. export PATH=\"\${HOME}/.local/bin:\${PATH}\")"
  fi
}

main "$@"
