#!/bin/sh
# Build portable archives without a release service or runtime dependencies.
set -eu
version=${1:-dev}
case "$version" in *[!a-zA-Z0-9._-]*|'') echo 'Invalid version' >&2; exit 2;; esac
mkdir -p dist
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
    os=${target%/*}
    arch=${target#*/}
    name="reqdiff_${version}_${os}_${arch}"
    folder="dist/$name"
    mkdir -p "$folder"
    binary=reqdiff
    if [ "$os" = windows ]; then binary=reqdiff.exe; fi
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath \
        -ldflags "-s -w -X main.version=$version" -o "$folder/$binary" ./cmd/reqdiff
    cp README.md LICENSE "$folder/"
    if [ "$os" = windows ]; then
        (cd dist && zip -q -j "$name.zip" "$name/$binary" "$name/README.md" "$name/LICENSE")
    else
        tar -czf "dist/$name.tar.gz" -C "$folder" "$binary" README.md LICENSE
    fi
done
(cd dist && shasum -a 256 ./*.tar.gz ./*.zip > checksums.txt)
