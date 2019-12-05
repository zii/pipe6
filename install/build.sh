#!/bin/bash

set -e

if [[ $1 == "win" ]]; then
    CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -ldflags "-s -w" -o ./local.exe "github.com/zii/pipe6/cmd/local"
    CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -ldflags "-s -w" -o ./remote.exe "github.com/zii/pipe6/cmd/remote"
    tar czvf pipe6.windows-386.tar.gz ./local.exe ./remote.exe ./local.pem ./local.key ./remote.key ./remote.pem
    rm ./local.exe ./remote.exe
elif [[ $1 == "mac" ]]; then
    CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o ./local "github.com/zii/pipe6/cmd/local"
    CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o ./remote "github.com/zii/pipe6/cmd/remote"
    tar czvf pipe6.mac-amd64.tar.gz ./local ./remote ./local.pem ./local.key ./remote.key ./remote.pem
    rm ./local ./remote
elif [[ $1 == "linux" ]]; then
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ./local "github.com/zii/pipe6/cmd/local"
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ./remote "github.com/zii/pipe6/cmd/remote"
    tar czvf pipe6.linux-amd64.tar.gz ./local ./remote ./local.pem ./local.key ./remote.key ./remote.pem
    rm ./local ./remote
else
    echo "usage: ./build.sh [win/mac/linux]"
fi