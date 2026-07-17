#!/usr/bin/env bash
# release.sh — build cross-platform drift binaries + archives + checksums.
#
# Usage:
#   ./scripts/release.sh [version]
#
# If version is omitted, falls back to `git describe --tags --dirty` or "dev".
# Outputs to dist/:
#   drift_<ver>_<os>_<arch>.tar.gz   (unix)
#   drift_<ver>_<os>_<arch>.zip      (windows)
#   checksums.txt                    (sha256 of all archives)
#
# Designed to run locally (snapshot) and in GitHub Actions (release).
set -euo pipefail

VER="${1:-$(git describe --tags --dirty 2>/dev/null || echo dev)}"
OUT="dist"

# 17 targets — Go cross-compiles all of these natively with CGO_ENABLED=0.
TARGETS=(
	linux/amd64 linux/arm64 linux/386 linux/arm
	linux/ppc64le linux/s390x linux/riscv64 linux/mips64le
	darwin/amd64 darwin/arm64
	windows/amd64 windows/arm64 windows/386
	freebsd/amd64 freebsd/arm64
	openbsd/amd64 netbsd/amd64
)

echo "Building drift ${VER} for ${#TARGETS[@]} targets → ${OUT}/"

rm -rf "$OUT"
mkdir -p "$OUT"

LDFLAGS="-s -w -X main.version=${VER}"

for t in "${TARGETS[@]}"; do
	GOOS="${t%/*}"
	GOARCH="${t#*/}"
	BIN="drift"
	if [ "$GOOS" = "windows" ]; then
		BIN="drift.exe"
	fi
	STAGE="${OUT}/drift_${GOOS}_${GOARCH}"
	ARCHIVE="${OUT}/drift_${VER}_${GOOS}_${GOARCH}"

	echo "  → ${GOOS}/${GOARCH}"

	CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" \
		go build -trimpath -ldflags="$LDFLAGS" \
		-o "${STAGE}/${BIN}" ./cmd/drift

	if [ "$GOOS" = "windows" ]; then
		zip -rq "${ARCHIVE}.zip" "${STAGE}/${BIN}" -j
	else
		tar -czf "${ARCHIVE}.tar.gz" -C "$STAGE" "$BIN"
	fi

	rm -rf "$STAGE"
done

echo "Generating checksums..."
(cd "$OUT" && sha256sum *.tar.gz *.zip > checksums.txt)

echo "Done. Artifacts in ${OUT}/:"
ls -1 "$OUT"

echo
echo "Checksums:"
cat "$OUT/checksums.txt"
