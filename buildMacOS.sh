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
  wails build -platform darwin/universal -ldflags "$LDFLAGS"
else
  wails build -platform darwin/universal
fi

codesign --force --deep -s - build/bin/contrails.app

echo "Build complete: build/bin/contrails.app"
