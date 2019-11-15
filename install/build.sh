#!/bin/bash

set -e

if [[ $1 == "win" ]]; then
    CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -ldflags "-s -w" -o ./pipe6c.exe "github.com/zii/pipe6/cmd/local"
    tar czvf pipe6c.tar.gz ./pipe6c.exe ./local.pem ./local.key ./remote.pem
    rm ./pipe6c.exe
else
    echo "usage: ./build.sh [win/mac/linux]"
fi