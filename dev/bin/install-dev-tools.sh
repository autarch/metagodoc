#!/bin/bash

set -eo pipefail

BINDIR="$GOPATH/bin"
if [ -n "$CI" ]; then
    BINDIR="./.bin"
fi

set -x

go get -u github.com/ActiveState/gometalinter-helper/cmd/gometalinter-helper
./dev/bin/install-gometalinter.sh -b "$BINDIR"

VER=$(curl -s https://api.github.com/repos/go-swagger/go-swagger/releases/latest | jq -r .tag_name)
ARCH=amd64
if [ $(uname -p) == "i386" ]; then
    ARCH=386
fi
OS=$(uname|tr '[:upper:]' '[:lower:]')
URL=https://github.com/go-swagger/go-swagger/releases/download/$VER/swagger_${OS}_${ARCH}
curl -o "$GOPATH/bin/swagger" -L'#' $URL
chmod +x "$GOPATH/bin/swagger"
