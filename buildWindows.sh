#!/bin/sh
set -e

LDFLAGS=""
if [ -n "$1" ]; then
  LDFLAGS="$LDFLAGS -X main.Version=$1"
fi
if [ -n "$2" ]; then
  LDFLAGS="$LDFLAGS -X main.PostHogAPIKey=$2"
fi

if [ -n "$LDFLAGS" ]; then
  wails build -platform windows/amd64 -webview2 embed -ldflags "$LDFLAGS"
else
  wails build -platform windows/amd64 -webview2 embed
fi

echo "Build complete: build/bin/contrails.exe"
