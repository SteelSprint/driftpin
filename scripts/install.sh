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

umask 022

REPO="SteelSprint/Drift"

# --- guards: never rm -rf an unset/empty/inherited path ---
WORKDIR=""
STAGE=""
cleanup() {
	if [ -n "$WORKDIR" ] && [ -d "$WORKDIR" ]; then
		rm -rf "$WORKDIR"
	fi
	if [ -n "$STAGE" ] && [ -f "$STAGE" ]; then
		rm -f "$STAGE"
	fi
}
trap cleanup EXIT INT TERM HUP

err() {
	echo "install: error: $*" >&2
	exit 1
}

# portable sha256: macOS ships `shasum -a 256`, Linux ships `sha256sum`.
# emits the hex digest on stdout (first field), matching the checksums.txt format.
if command -v sha256sum >/dev/null 2>&1; then
	sha256() { sha256sum "$1" | awk '{print $1}'; }
elif command -v shasum >/dev/null 2>&1; then
	sha256() { shasum -a 256 "$1" | awk '{print $1}'; }
else
	err "neither sha256sum nor shasum found; cannot verify checksums. Install coreutils or perl."
fi

# --- validate HOME and DESTDIR before any mkdir/mv ---
if [ -z "${HOME:-}" ] || [ ! -d "${HOME:-}" ]; then
	err "HOME is unset or not a directory; refusing to install. Set \$HOME or \$DESTDIR explicitly."
fi
DESTDIR="${DESTDIR:-${HOME}/.local/bin}"
if [ -z "$DESTDIR" ]; then
	err "DESTDIR is empty; refusing to install."
fi
# reject obviously dangerous install targets
case "$DESTDIR" in
	/|/etc|/etc/*|/usr|/usr/*|/bin|/bin/*|/sbin|/sbin/*|/lib|/lib/*|/boot|/boot/*|/dev|/dev/*|/proc|/proc/*|/sys|/sys/*)
		err "DESTDIR='$DESTDIR' looks like a system path; refusing to install there. Use ~/.local/bin or a user-writable dir."
		;;
esac

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

# --- validate TAG: must be vMAJOR.MINOR.PATCH with optional pre-release/build suffix ---
# this prevents a corrupted/malicious API response from injecting garbage into the URL/filename.
if ! printf '%s' "$TAG" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+([+-][A-Za-z0-9._-]+)?$'; then
	err "invalid version tag '$TAG'; expected format like v1.0.0"
fi

# strip leading 'v' for the archive version string
VER="${TAG#v}"
ARCHIVE="drift_${VER}_${GOOS}_${GOARCH}"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE}.tar.gz"

echo "Version: ${TAG}"
echo "Downloading: ${URL}"

# --- create work dir (renamed from TMPDIR to avoid shadowing the standard env var) ---
WORKDIR="$(mktemp -d 2>/dev/null)" || err "could not create temp directory"
# belt-and-suspenders: confirm WORKDIR is a real directory we just created
if [ ! -d "$WORKDIR" ] || [ "$(cd "$WORKDIR" && pwd)" = "/" ]; then
	err "temp directory creation returned an unsafe path: '$WORKDIR'"
fi

if ! curl -fsSL --retry 3 "$URL" -o "${WORKDIR}/${ARCHIVE}.tar.gz"; then
	err "download failed for ${URL}

If your platform (${GOOS}/${GOARCH}) is not in the release assets, open an issue:
  https://github.com/${REPO}/issues
Available platforms are listed in each release: https://github.com/${REPO}/releases"
fi

# --- verify checksum (strict: checksums.txt must be present AND list our archive) ---
CHECKSUM_URL="https://github.com/${REPO}/releases/download/${TAG}/checksums.txt"
CHECKSUM_VERIFIED=false
if curl -fsSL --retry 3 "$CHECKSUM_URL" -o "${WORKDIR}/checksums.txt" 2>/dev/null; then
	# use grep -F (fixed string) so '.' in the filename isn't treated as a regex wildcard
	EXPECTED="$(grep -F "$(basename "${URL}")" "${WORKDIR}/checksums.txt" | awk '{print $1}')"
	if [ -z "$EXPECTED" ]; then
		err "checksums.txt is available but does not list '${ARCHIVE}.tar.gz'.
This is either a release packaging error or a tampering attempt — refusing to install."
	fi
	ACTUAL="$(sha256 "${WORKDIR}/${ARCHIVE}.tar.gz")"
	if [ "$ACTUAL" != "$EXPECTED" ]; then
		err "checksum mismatch — the downloaded archive was tampered with or corrupted.
expected: ${EXPECTED}
actual:   ${ACTUAL}"
	fi
	CHECKSUM_VERIFIED=true
	echo "Checksum verified."
else
	err "could not download checksums.txt from ${CHECKSUM_URL}
Refusing to install without checksum verification."
fi

# --- extract ONLY the 'drift' binary (not the whole archive) ---
# this refuses path-traversal entries and unexpected files.
if ! tar -xzf "${WORKDIR}/${ARCHIVE}.tar.gz" -C "$WORKDIR" drift 2>/dev/null; then
	err "archive did not contain a 'drift' binary at its root"
fi
if [ ! -f "${WORKDIR}/drift" ]; then
	err "extraction succeeded but drift binary is missing"
fi

# --- install (atomic) ---
# Stage the new binary in the same directory as the target so the final
# `mv` is a single atomic rename(2) syscall. Either INSTALL_PATH becomes
# the new binary or it's unchanged — there is no intermediate state.
# This is crash-safe: if any step fails (disk full, signal, permission),
# the previous binary at INSTALL_PATH is preserved.
mkdir -p "$DESTDIR"
INSTALL_PATH="${DESTDIR}/drift"
STAGE="${DESTDIR}/.drift.new.$$"

# if INSTALL_PATH is a directory, refuse (don't silently install inside it)
if [ -d "$INSTALL_PATH" ]; then
	err "${INSTALL_PATH} is a directory, not a file; refusing to overwrite. Remove it first."
fi

# stage the new binary in the target directory (same filesystem → atomic rename)
cp "${WORKDIR}/drift" "$STAGE" || err "could not stage new binary to ${STAGE} (disk full?)"
chmod +x "$STAGE" || { rm -f "$STAGE"; err "could not chmod staged binary"; }

# keep a backup COPY of the existing binary for manual rollback.
# this is a copy (not a move) — the original stays in place until the
# atomic rename below, so INSTALL_PATH is never missing.
if [ -e "$INSTALL_PATH" ] && [ ! -d "$INSTALL_PATH" ]; then
	cp "$INSTALL_PATH" "${INSTALL_PATH}.old" 2>/dev/null || true
fi

# atomic install: rename(2) either replaces INSTALL_PATH entirely or does nothing.
# if this fails, INSTALL_PATH still holds the previous (working) binary.
if ! mv -f "$STAGE" "$INSTALL_PATH"; then
	rm -f "$STAGE"
	err "could not install to ${INSTALL_PATH} (permission denied?). The previous binary is unchanged."
fi

echo
echo "Installed drift ${TAG} → ${INSTALL_PATH}"
if [ -f "${INSTALL_PATH}.old" ]; then
	echo "Previous version backed up to ${INSTALL_PATH}.old"
fi

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

# --- verify (only if checksum was verified — never run an unverified binary) ---
if $CHECKSUM_VERIFIED; then
	if "$INSTALL_PATH" version 2>/dev/null; then
		echo
		echo "Run 'drift help' to get started."
	else
		echo
		echo "Installed, but 'drift version' failed. Run '${INSTALL_PATH} help' to check."
	fi
else
	echo
	echo "Run '${INSTALL_PATH} help' to get started."
fi
