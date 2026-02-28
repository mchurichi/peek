#!/bin/sh

set -eu

OWNER="mchurichi"
REPO="peek"
BIN_NAME="peek"
DEFAULT_INSTALL_DIR="${HOME}/.local/bin"

MODE=""
VERSION=""
INSTALL_DIR="${DEFAULT_INSTALL_DIR}"
INSTALL_DIR_SET=0
USE_SYSTEM=0
PURGE_ALL=0
FORCE_PURGE=0
USE_SUDO=0
TMP_DIR=""

log() {
  printf '%s\n' "$*"
}

fail() {
  printf 'Error: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Peek installer (Linux)

Usage:
  get-peek.sh [install|uninstall] [options]

Commands:
  install            Install peek
  uninstall          Remove peek from install directory

Options:
  --version <vX.Y.Z>   Install a specific version (default: latest release)
  --install-dir <dir>  Install directory (default: ~/.local/bin)
  --system             Install to /usr/local/bin
  --purge              With uninstall: remove all ~/.peek data and config
  --force              With uninstall --purge: skip confirmation prompt
  -h, --help           Show help

Examples:
  curl -fsSL https://raw.githubusercontent.com/mchurichi/peek/main/scripts/get-peek.sh | sh -s -- install
  curl -fsSL https://raw.githubusercontent.com/mchurichi/peek/main/scripts/get-peek.sh | sh -s -- install --version v0.1.0
  curl -fsSL https://raw.githubusercontent.com/mchurichi/peek/main/scripts/get-peek.sh | sh -s -- install --system
  curl -fsSL https://raw.githubusercontent.com/mchurichi/peek/main/scripts/get-peek.sh | sh -s -- uninstall
  curl -fsSL https://raw.githubusercontent.com/mchurichi/peek/main/scripts/get-peek.sh | sh -s -- uninstall --purge
  curl -fsSL https://raw.githubusercontent.com/mchurichi/peek/main/scripts/get-peek.sh | sh -s -- uninstall --purge --force
EOF
}

cleanup() {
  if [ -n "${TMP_DIR}" ] && [ -d "${TMP_DIR}" ]; then
    rm -rf "${TMP_DIR}"
  fi
}

trap cleanup EXIT INT TERM

normalize_version() {
  value="$1"
  case "${value}" in
    v*) printf '%s' "${value}" ;;
    *) printf 'v%s' "${value}" ;;
  esac
}

asset_version() {
  version_tag="$1"
  printf '%s' "${version_tag#v}"
}

is_linux() {
  os="$(uname -s 2>/dev/null || true)"
  [ "${os}" = "Linux" ]
}

detect_arch() {
  machine="$(uname -m 2>/dev/null || true)"
  case "${machine}" in
    x86_64|amd64) printf '%s' "amd64" ;;
    aarch64|arm64) printf '%s' "arm64" ;;
    *)
      fail "unsupported architecture '${machine}'. Supported: x86_64/amd64, aarch64/arm64"
      ;;
  esac
}

pick_downloader() {
  if command -v curl >/dev/null 2>&1; then
    printf '%s' "curl"
    return
  fi

  if command -v wget >/dev/null 2>&1; then
    printf '%s' "wget"
    return
  fi

  fail "missing downloader. Install curl or wget."
}

download_to_file() {
  downloader="$1"
  url="$2"
  output="$3"

  if [ "${downloader}" = "curl" ]; then
    curl -fsSL "${url}" -o "${output}"
  else
    wget -qO "${output}" "${url}"
  fi
}

download_to_stdout() {
  downloader="$1"
  url="$2"

  if [ "${downloader}" = "curl" ]; then
    curl -fsSL "${url}"
  else
    wget -qO- "${url}"
  fi
}

find_latest_version() {
  downloader="$1"
  api_url="https://api.github.com/repos/${OWNER}/${REPO}/releases/latest"
  payload="$(download_to_stdout "${downloader}" "${api_url}")"
  version="$(printf '%s' "${payload}" | tr -d '\n' | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"

  if [ -z "${version}" ]; then
    fail "could not determine latest version from GitHub API"
  fi

  printf '%s' "${version}"
}

resolve_checksum() {
  checksums_file="$1"
  archive_name="$2"
  awk -v target="${archive_name}" '
    $2 == target || $2 == "*" target { print $1; exit }
  ' "${checksums_file}"
}

sha256_of_file() {
  file_path="$1"

  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${file_path}" | awk '{print $1}'
    return
  fi

  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "${file_path}" | awk '{print $1}'
    return
  fi

  if command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 "${file_path}" | awk '{print $NF}'
    return
  fi

  fail "missing checksum tool. Install sha256sum, shasum, or openssl."
}

needs_sudo_for_path() {
  path_check="$1"

  if [ -d "${path_check}" ]; then
    [ -w "${path_check}" ] && return 1
    return 0
  fi

  parent_dir="${path_check}"
  while [ ! -d "${parent_dir}" ]; do
    parent_dir="$(dirname "${parent_dir}")"
  done

  [ -w "${parent_dir}" ] && return 1
  return 0
}

configure_privileges() {
  install_target="$1"
  USE_SUDO=0

  if needs_sudo_for_path "${install_target}"; then
    if command -v sudo >/dev/null 2>&1; then
      USE_SUDO=1
    else
      fail "need elevated privileges for '${install_target}' but sudo is not available"
    fi
  fi
}

run_maybe_sudo() {
  if [ "${USE_SUDO}" -eq 1 ]; then
    sudo "$@"
  else
    "$@"
  fi
}

validate_install_dir() {
  if [ -z "${INSTALL_DIR}" ]; then
    fail "install directory cannot be empty"
  fi
}

confirm_purge() {
  if [ "${FORCE_PURGE}" -eq 1 ]; then
    return 0
  fi

  if [ ! -t 1 ] || [ ! -e /dev/tty ]; then
    fail "uninstall --purge requires confirmation. Re-run with --force in non-interactive mode."
  fi

  printf 'This will permanently remove %s. Continue? [y/N]: ' "${HOME}/.peek" > /dev/tty
  read -r response < /dev/tty || fail "unable to read confirmation from terminal"
  case "${response}" in
    y|Y|yes|YES|Yes)
      return 0
      ;;
    *)
      fail "purge cancelled"
      ;;
  esac
}

install_binary() {
  [ -n "${VERSION}" ] || fail "internal error: empty version"

  downloader="$(pick_downloader)"
  arch="$(detect_arch)"

  release_tag="${VERSION}"
  release_asset_version="$(asset_version "${release_tag}")"
  archive_name_primary="${BIN_NAME}_${release_asset_version}_linux_${arch}.tar.gz"
  archive_name_fallback="${BIN_NAME}_${release_tag}_linux_${arch}.tar.gz"
  checksums_name="checksums.txt"
  release_base_url="https://github.com/${OWNER}/${REPO}/releases/download/${release_tag}"

  TMP_DIR="$(mktemp -d)"
  archive_path="${TMP_DIR}/${BIN_NAME}.tar.gz"
  checksums_path="${TMP_DIR}/${checksums_name}"
  downloaded_archive_name=""

  log "Downloading ${checksums_name}..."
  download_to_file "${downloader}" "${release_base_url}/${checksums_name}" "${checksums_path}" || fail "failed to download ${checksums_name}"

  log "Downloading ${archive_name_primary}..."
  if download_to_file "${downloader}" "${release_base_url}/${archive_name_primary}" "${archive_path}"; then
    downloaded_archive_name="${archive_name_primary}"
  else
    log "Primary asset not found, trying ${archive_name_fallback}..."
    if download_to_file "${downloader}" "${release_base_url}/${archive_name_fallback}" "${archive_path}"; then
      downloaded_archive_name="${archive_name_fallback}"
    fi
  fi

  if [ -z "${downloaded_archive_name}" ]; then
    fail "failed to download release archive (tried ${archive_name_primary} and ${archive_name_fallback})"
  fi

  expected_checksum="$(resolve_checksum "${checksums_path}" "${downloaded_archive_name}")"
  [ -n "${expected_checksum}" ] || fail "checksum not found for ${downloaded_archive_name}"

  actual_checksum="$(sha256_of_file "${archive_path}")"
  if [ "${expected_checksum}" != "${actual_checksum}" ]; then
    fail "checksum verification failed for ${downloaded_archive_name}"
  fi
  log "Checksum verified."

  tar -xzf "${archive_path}" -C "${TMP_DIR}" || fail "failed to extract ${downloaded_archive_name}"
  [ -f "${TMP_DIR}/${BIN_NAME}" ] || fail "archive does not contain ${BIN_NAME} binary"

  configure_privileges "${INSTALL_DIR}"
  if [ "${USE_SUDO}" -eq 1 ]; then
    log "Installing with sudo to ${INSTALL_DIR}/${BIN_NAME}"
  else
    log "Installing to ${INSTALL_DIR}/${BIN_NAME}"
  fi

  run_maybe_sudo mkdir -p "${INSTALL_DIR}"

  if command -v install >/dev/null 2>&1; then
    run_maybe_sudo install -m 0755 "${TMP_DIR}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"
  else
    run_maybe_sudo cp "${TMP_DIR}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"
    run_maybe_sudo chmod 0755 "${INSTALL_DIR}/${BIN_NAME}"
  fi

  log "Installed ${BIN_NAME} ${VERSION} at ${INSTALL_DIR}/${BIN_NAME}"
  log "Run: ${BIN_NAME} version"

  case ":${PATH}:" in
    *:"${INSTALL_DIR}":*)
      ;;
    *)
      log "Note: ${INSTALL_DIR} is not currently on PATH."
      log "Add this to your shell profile:"
      log "  export PATH=\"${INSTALL_DIR}:\$PATH\""
      ;;
  esac
}

uninstall_binary() {
  target="${INSTALL_DIR}/${BIN_NAME}"
  configure_privileges "${INSTALL_DIR}"

  if [ "${PURGE_ALL}" -eq 1 ]; then
    # Confirm purge intent before deleting anything to avoid partial actions.
    confirm_purge
  fi

  if [ -f "${target}" ]; then
    if [ "${USE_SUDO}" -eq 1 ]; then
      log "Removing ${target} with sudo..."
    else
      log "Removing ${target}..."
    fi
    run_maybe_sudo rm -f "${target}"
    log "Removed ${target}"
  else
    log "No binary found at ${target}"
  fi

  if [ "${PURGE_ALL}" -eq 1 ]; then
    peek_dir="${HOME}/.peek"
    if [ -d "${peek_dir}" ]; then
      log "Removing ${peek_dir}..."
      rm -rf "${peek_dir}"
      log "Removed ${peek_dir}"
    else
      log "No directory found at ${peek_dir}"
    fi
  fi
}

if [ "$#" -eq 0 ]; then
  usage
  exit 0
fi

while [ "$#" -gt 0 ]; do
  if [ "$1" = "install" ] || [ "$1" = "uninstall" ]; then
    MODE="$1"
    shift
    continue
  fi

  case "$1" in
    --version)
      shift
      [ "$#" -gt 0 ] || fail "--version requires a value"
      VERSION="$(normalize_version "$1")"
      ;;
    --install-dir)
      shift
      [ "$#" -gt 0 ] || fail "--install-dir requires a value"
      INSTALL_DIR="$1"
      INSTALL_DIR_SET=1
      ;;
    --system)
      USE_SYSTEM=1
      ;;
    --purge)
      PURGE_ALL=1
      ;;
    --force)
      FORCE_PURGE=1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "unknown option: $1"
      ;;
  esac
  shift
done

if [ -z "${MODE}" ]; then
  fail "missing command. Use: install or uninstall"
fi

if [ "${USE_SYSTEM}" -eq 1 ] && [ "${INSTALL_DIR_SET}" -eq 1 ]; then
  fail "use either --system or --install-dir, not both"
fi

if [ "${USE_SYSTEM}" -eq 1 ]; then
  INSTALL_DIR="/usr/local/bin"
fi

validate_install_dir

if ! is_linux; then
  fail "this installer currently supports Linux only"
fi

if [ "${MODE}" = "install" ]; then
  if [ "${PURGE_ALL}" -eq 1 ] || [ "${FORCE_PURGE}" -eq 1 ]; then
    fail "--purge and --force can only be used with the uninstall command"
  fi

  if [ -z "${VERSION}" ]; then
    VERSION="$(find_latest_version "$(pick_downloader)")"
  fi

  install_binary
  exit 0
fi

if [ -n "${VERSION}" ]; then
  fail "--version is not used with the uninstall command"
fi

if [ "${FORCE_PURGE}" -eq 1 ] && [ "${PURGE_ALL}" -eq 0 ]; then
  fail "--force can only be used with --purge"
fi

uninstall_binary
log "Uninstall complete."
