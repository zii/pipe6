#!/bin/bash

set -e

if [[ $1 == 1 ]]; then
    echo "run pipe6 remote..."
    go build -o ./remote github.com/zii/pipe6/cmd/remote
    ./remote -p 18443
elif [[ $1 == 2 ]]; then
    echo "run pipe6 local..."
    go build -o ./local github.com/zii/pipe6/cmd/local
    ./local
elif [[ $1 == 3 ]]; then
    echo "run pipe6 local..."
    go build -o ./local github.com/zii/pipe6/cmd/local
    ./local -remote 172.96.225.179:18443
fi
