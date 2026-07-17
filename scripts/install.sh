#!/usr/bin/env bash
# install.sh — curl-able installer for drift.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/SteelSprint/Drift/main/scripts/install.sh | bash
#
# Or pin a version:
#   DRIFT_VERSION=v1.0.0 curl -fsSL ... | bash
#
# Installs to ~/.local/bin/drift (or $DESTDIR/drift if DESTDIR is set).
# Prints a PATH hint if the install location is not on $PATH.
set -euo pipefail

REPO="SteelSprint/Drift"
DESTDIR="${DESTDIR:-${HOME}/.local/bin}"

err() {
	echo "install: error: $*" >&2
	exit 1
}

# --- detect GOOS ---
case "$(uname -s)" in
	Linux*)  GOOS=linux ;;
	Darwin*) GOOS=darwin ;;
	FreeBSD*) GOOS=freebsd ;;
	OpenBSD*) GOOS=openbsd ;;
	NetBSD*) GOOS=netbsd ;;
	*) err "unsupported OS: $(uname -s)" ;;
esac

# --- detect GOARCH ---
case "$(uname -m)" in
	x86_64|amd64)    GOARCH=amd64 ;;
	aarch64|arm64)   GOARCH=arm64 ;;
	i386|i686)       GOARCH=386 ;;
	armv7l|armv6l)   GOARCH=arm ;;
	ppc64le|powerpc64le) GOARCH=ppc64le ;;
	s390x)           GOARCH=s390x ;;
	riscv64)         GOARCH=riscv64 ;;
	mips64|mips64le) GOARCH=mips64le ;;
	*) err "unsupported arch: $(uname -m)" ;;
esac

echo "Detected: ${GOOS}/${GOARCH}"

# --- resolve version ---
if [ -n "${DRIFT_VERSION:-}" ]; then
	TAG="$DRIFT_VERSION"
else
	echo "Fetching latest release..."
	TAG="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
		| grep -m1 '"tag_name"' \
		| sed -E 's/.*"([^"]+)".*/\1/')"
	if [ -z "$TAG" ]; then
		err "could not determine latest release tag from GitHub API"
	fi
fi

# strip leading 'v' for the archive version string
VER="${TAG#v}"
ARCHIVE="drift_${VER}_${GOOS}_${GOARCH}"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE}.tar.gz"

echo "Version: ${TAG}"
echo "Downloading: ${URL}"

# --- download to temp dir ---
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

if ! curl -fsSL "$URL" -o "${TMPDIR}/${ARCHIVE}.tar.gz"; then
	err "download failed for ${URL}

If your platform (${GOOS}/${GOARCH}) is not in the release assets, open an issue:
  https://github.com/${REPO}/issues
Available platforms are listed in each release: https://github.com/${REPO}/releases"
fi

# --- verify checksum if checksums.txt is available ---
CHECKSUM_URL="https://github.com/${REPO}/releases/download/${TAG}/checksums.txt"
if curl -fsSL "$CHECKSUM_URL" -o "${TMPDIR}/checksums.txt" 2>/dev/null; then
	EXPECTED="$(grep -E "$(basename "${URL}")" "${TMPDIR}/checksums.txt" | awk '{print $1}')"
	if [ -n "$EXPECTED" ]; then
		ACTUAL="$(sha256sum "${TMPDIR}/${ARCHIVE}.tar.gz" | awk '{print $1}')"
		if [ "$ACTUAL" != "$EXPECTED" ]; then
			err "checksum mismatch
expected: ${EXPECTED}
actual:   ${ACTUAL}"
		fi
		echo "Checksum verified."
	else
		echo "Warning: archive not found in checksums.txt, skipping verification."
	fi
else
	echo "Warning: checksums.txt not available, skipping verification."
fi

# --- extract ---
tar -xzf "${TMPDIR}/${ARCHIVE}.tar.gz" -C "$TMPDIR"
if [ ! -f "${TMPDIR}/drift" ]; then
	err "archive did not contain a 'drift' binary"
fi

# --- install ---
mkdir -p "$DESTDIR"
INSTALL_PATH="${DESTDIR}/drift"

# don't overwrite a running binary
if [ -f "$INSTALL_PATH" ] && [ -w "$INSTALL_PATH" ]; then
	mv "$INSTALL_PATH" "${INSTALL_PATH}.old" 2>/dev/null || true
fi

mv "${TMPDIR}/drift" "$INSTALL_PATH"
chmod +x "$INSTALL_PATH"

echo
echo "Installed drift ${TAG} → ${INSTALL_PATH}"

# --- PATH hint ---
case ":${PATH}:" in
	*":${DESTDIR}:"*) ;;
	*)
		echo
		echo "WARNING: ${DESTDIR} is not on your PATH."
		echo "Add it to your shell profile:"
		echo "  echo 'export PATH=\"${DESTDIR}:\$PATH\"' >> ~/.bashrc"
		echo "  # or for zsh: ~/.zshrc"
		;;
esac

# --- verify ---
if "$INSTALL_PATH" version 2>/dev/null; then
	echo
	echo "Run 'drift help' to get started."
else
	echo
	echo "Installed. Run '${INSTALL_PATH} help' to get started."
fi
