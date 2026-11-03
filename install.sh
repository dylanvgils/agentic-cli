#!/usr/bin/env bash
set -euo pipefail

# --- Variables ---

INSTALL_DIR="${HOME}/.local/bin"
INSTALL_PATH="${INSTALL_DIR}/agentic"
DATA_DIR="${AGENTIC_HOME:-${HOME}/.agentic}"
CERT_IDENTITY="https://github.com/dylanvgils/agentic-cli/.github/workflows/nightly-release.yml@refs/heads/main"
CERT_ISSUER="https://token.actions.githubusercontent.com"
FIRST_SIGNED_VERSION="0.53.0"
VERIFY_DOCS="https://github.com/dylanvgils/agentic-cli/blob/main/docs/installation.md#verifying-a-release"

# --- Functions ---

# version_at_least succeeds when dotted version $1 is at least $2; a "-suffix" is ignored.
version_at_least() {
  local -a have want
  IFS=. read -r -a have <<<"${1%%-*}"
  IFS=. read -r -a want <<<"${2%%-*}"

  local i h w
  for ((i = 0; i < ${#have[@]} || i < ${#want[@]}; i++)); do
    h=$((10#${have[i]:-0}))
    w=$((10#${want[i]:-0}))
    if ((h != w)); then
      ((h > w))
      return
    fi
  done
}

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
    --file "${script_dir}/build/Dockerfile" \
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
  version=$(curl --proto '=https' --tlsv1.2 -fsSL https://api.github.com/repos/dylanvgils/agentic-cli/releases/latest |
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
  curl --proto '=https' --tlsv1.2 -fsSL "${url}" -o "${tmpdir}/${archive}"
  curl --proto '=https' --tlsv1.2 -fsSL "${checksums_url}" -o "${tmpdir}/checksums.txt"

  # Optional signature check: only with cosign installed; a missing bundle is accepted only before signing started
  local tip=""
  local bundle="${tmpdir}/checksums.txt.sigstore.json"
  local status=""
  if command -v cosign &>/dev/null; then
    status=$(curl --proto '=https' --tlsv1.2 -sSL -w '%{http_code}' -o "${bundle}" "${checksums_url}.sigstore.json")
  fi

  if [[ -z "${status}" ]]; then
    tip="Tip: install cosign to also verify the release signature, see ${VERIFY_DOCS}"
  elif [[ "${status}" == "404" ]]; then
    if version_at_least "${version}" "${FIRST_SIGNED_VERSION}"; then
      echo "Error: release ${version} has no signature bundle (checksums.txt.sigstore.json); refusing to install." >&2
      exit 1
    fi
    tip="Tip: release ${version} is unsigned, verified by checksum only."
  elif [[ "${status}" != "200" ]]; then
    echo "Error: downloading the signature bundle failed with HTTP ${status}." >&2
    exit 1
  else
    echo "Verifying signature..."
    if ! cosign verify-blob \
      --bundle "${bundle}" \
      --certificate-identity "${CERT_IDENTITY}" \
      --certificate-oidc-issuer "${CERT_ISSUER}" \
      "${tmpdir}/checksums.txt"; then
      echo "Error: signature verification failed for checksums.txt." >&2
      exit 1
    fi
  fi

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

  if [[ -n "${tip}" ]]; then
    echo "${tip}"
  fi
}

# --- Main ---

main() {
  local remove=0
  local from_source=0

  while [[ $# -gt 0 ]]; do
    local arg="$1"
    case "${arg}" in
    --remove | -r)
      remove=1
      shift
      ;;
    --from-source)
      from_source=1
      shift
      ;;
    *)
      echo "Unknown argument: ${arg}" >&2
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
