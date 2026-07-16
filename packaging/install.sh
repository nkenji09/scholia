#!/usr/bin/env bash
# scholia installer — curl -fsSL https://raw.githubusercontent.com/nkenji09/scholia/main/packaging/install.sh | sh
#
# Downloads the latest scholia release for this OS/arch from GitHub Releases
# and installs it into $SCHOLIA_INSTALL_DIR (default: ~/.local/bin).
set -euo pipefail

REPO="nkenji09/scholia"
BINARY_NAME="scholia"
INSTALL_DIR="${SCHOLIA_INSTALL_DIR:-$HOME/.local/bin}"

fail() {
  echo "error: $1" >&2
  exit 1
}

detect_os() {
  case "$(uname -s)" in
    Darwin) echo "darwin" ;;
    Linux) echo "linux" ;;
    *) fail "unsupported OS: $(uname -s) (scholia prebuilt binaries cover darwin/linux only; try 'go install github.com/${REPO}/cmd/scholia@latest')" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64 | amd64) echo "amd64" ;;
    arm64 | aarch64) echo "arm64" ;;
    *) fail "unsupported architecture: $(uname -m)" ;;
  esac
}

main() {
  command -v curl >/dev/null 2>&1 || fail "curl is required"
  command -v tar >/dev/null 2>&1 || fail "tar is required"

  os="$(detect_os)"
  arch="$(detect_arch)"
  archive="${BINARY_NAME}_${os}_${arch}.tar.gz"
  url="https://github.com/${REPO}/releases/latest/download/${archive}"

  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "$tmp_dir"' EXIT

  echo "Downloading ${url}..."
  curl -fsSL "$url" -o "$tmp_dir/$archive" || fail "failed to download $url (does a release exist yet?)"

  tar -xzf "$tmp_dir/$archive" -C "$tmp_dir" "$BINARY_NAME"

  mkdir -p "$INSTALL_DIR"
  mv "$tmp_dir/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
  chmod +x "$INSTALL_DIR/$BINARY_NAME"

  echo "Installed ${BINARY_NAME} to ${INSTALL_DIR}/${BINARY_NAME}"
  case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *) echo "note: ${INSTALL_DIR} is not on your PATH. Add it, e.g.: export PATH=\"${INSTALL_DIR}:\$PATH\"" ;;
  esac
}

main "$@"
