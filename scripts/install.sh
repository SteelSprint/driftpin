#!/bin/bash
# #F id:s84shvi5 install.scripts install.platform_detection install.version install.checksum install.archive_validation install.no_overwrite install.permissions install.no_temp_cleanup install.install_dir install.path_check install.network_security install.errors
# #F id:ws7j2yhv tool.binary
# #F id:xr2m4kqt tool.location
set -eu
set -o pipefail
set -u
umask 022

REPO="steelsprint/filament"
BINARY="filament"

detect_platform() {
  os="$(uname -s)"
  arch="$(uname -m)"

  case "$os" in
    Linux)  os="linux" ;;
    Darwin) os="darwin" ;;
    *)
      echo "error: unsupported OS: $os" >&2
      echo "download manually from https://github.com/$REPO/releases" >&2
      exit 1
      ;;
  esac

  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *)
      echo "error: unsupported architecture: $arch" >&2
      exit 1
      ;;
  esac

  echo "${os}_${arch}"
}

detect_install_dir() {
  # prefer ~/.local/bin if it exists or can be created (no sudo needed)
  if [ -d "$HOME/.local/bin" ] || mkdir -p "$HOME/.local/bin" 2>/dev/null; then
    echo "$HOME/.local/bin"
    return
  fi

  # fallback to /usr/local/bin (needs sudo)
  echo "/usr/local/bin"
}

latest_version() {
  url="https://api.github.com/repos/$REPO/releases/latest"
  if command -v jq >/dev/null 2>&1; then
    version=$(curl --tlsv1.2 -fsSL "$url" | jq -r .tag_name)
  else
    version=$(curl --tlsv1.2 -fsSL "$url" | grep -Eo '"tag_name"[[:space:]]*:[[:space:]]*"[^"]+"' | sed 's/.*"//;s/"$//')
  fi
  if [ -z "$version" ]; then
    echo "error: could not determine latest version" >&2
    exit 1
  fi
  echo "$version"
}

verify_checksum() {
  file="$1"
  checksums="$2"
  filename="$(basename "$file")"

  expected=$(awk -v fn="$filename" '$2==fn{print $1; exit}' "$checksums")
  if [ -z "$expected" ]; then
    echo "error: no checksum found for $filename in checksums file" >&2
    exit 1
  fi

  if command -v sha256sum >/dev/null 2>&1; then
    actual=$(sha256sum "$file" | awk '{print $1}')
  elif command -v shasum >/dev/null 2>&1; then
    actual=$(shasum -a 256 "$file" | awk '{print $1}')
  else
    echo "error: no sha256sum or shasum found, cannot verify checksum" >&2
    exit 1
  fi

  if [ "$actual" != "$expected" ]; then
    echo "error: checksum mismatch for $filename" >&2
    echo "  expected: $expected" >&2
    echo "  actual:   $actual" >&2
    exit 1
  fi
}

validate_archive() {
  archive="$1"

  entries=$(tar -tzf "$archive")
  found_binary=0

  while IFS= read -r entry; do
    # reject path traversal
    case "$entry" in
      ../* | */../* | /* | */..)
        echo "error: archive contains unsafe path '$entry'" >&2
        exit 1
        ;;
    esac

    case "$entry" in
      "$BINARY") found_binary=1 ;;
      LICENSE|README.md) ;; # allowed extra files from goreleaser
      *)
        echo "error: archive contains unexpected entry '$entry'" >&2
        echo "  only $BINARY, LICENSE, and README.md are allowed" >&2
        exit 1
        ;;
    esac
  done <<< "$entries"

  if [ "$found_binary" -ne 1 ]; then
    echo "error: archive does not contain '$BINARY'" >&2
    exit 1
  fi
}

main() {
  platform="$(detect_platform)"
  version="$(latest_version)"
  install_dir="$(detect_install_dir)"

  archive="${BINARY}_${platform}.tar.gz"
  base_url="https://github.com/${REPO}/releases/download/${version}"

  tmpdir="$(mktemp -d)"

  echo "Downloading ${BINARY} ${version} for ${platform}..."
  curl --tlsv1.2 -fsSL -o "$tmpdir/$archive" "${base_url}/${archive}"
  curl --tlsv1.2 -fsSL -o "$tmpdir/checksums.txt" "${base_url}/checksums.txt"

  echo "Verifying checksum..."
  verify_checksum "$tmpdir/$archive" "$tmpdir/checksums.txt"

  echo "Validating archive..."
  validate_archive "$tmpdir/$archive"

  tar -xzf "$tmpdir/$archive" -C "$tmpdir"

  target="$install_dir/$BINARY"
  if [ -e "$target" ]; then
    echo "error: $target already exists. Remove it first:" >&2
    echo "  rm \"$target\"" >&2
    exit 1
  fi

  if [ -w "$install_dir" ]; then
    mv "$tmpdir/$BINARY" "$target"
    chmod 0755 "$target"
  else
    echo ""
    echo "  This script needs sudo to install to $install_dir."
    echo "  You may be prompted for your password."
    echo ""
    sudo mv "$tmpdir/$BINARY" "$target"
    sudo chmod 0755 "$target"
  fi

  echo "Installed $BINARY $version to $target"

  # check if on PATH
  case ":${PATH}:" in
    *":${install_dir}:"*) ;;
    *)
      echo ""
      echo "NOTE: $install_dir is not in your PATH."
      echo "Add it with:"
      echo ""
      if [ -f "$HOME/.bashrc" ]; then
        echo "  echo 'export PATH=\"$install_dir:\$PATH\"' >> ~/.bashrc"
      fi
      if [ -f "$HOME/.zshrc" ]; then
        echo "  echo 'export PATH=\"$install_dir:\$PATH\"' >> ~/.zshrc"
      fi
      echo ""
      echo "Or run: export PATH=\"$install_dir:\$PATH\""
      ;;
  esac
}

main "$@"
