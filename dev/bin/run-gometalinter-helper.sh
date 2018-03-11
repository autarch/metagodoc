#!/bin/bash

set -eo pipefail

if [ -n "$CI" ]; then
    set -x
    PATH="$PATH:./.bin"
fi

gometalinter-helper \
    $@ \
    -- \
    --vendor \
    --sort=path \
    --disable-all \
    --enable=deadcode \
    --enable=errcheck \
    --enable=gofmt \
    --enable=golint \
    --enable=megacheck \
    --enable=misspell \
    --enable=structcheck \
    --enable=vet
