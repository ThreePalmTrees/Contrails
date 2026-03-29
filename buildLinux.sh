#!/bin/sh
set -e

LDFLAGS=""
if [ -n "$1" ]; then
  LDFLAGS="$LDFLAGS -X main.Version=$1"
fi
if [ -n "$2" ]; then
  LDFLAGS="$LDFLAGS -X main.PostHogAPIKey=$2"
fi

# Ubuntu 24.04+ requires webkit2_41 build tag for libwebkit2gtk-4.1
TAGS="-tags webkit2_41"

if [ -n "$LDFLAGS" ]; then
  wails build -platform linux/amd64 $TAGS -ldflags "$LDFLAGS"
else
  wails build -platform linux/amd64 $TAGS
fi

echo "Build complete: build/bin/contrails"
