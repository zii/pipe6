#!/bin/bash
set -e

SSH1="root@172.96.225.179"
DIR1="$SSH1:/tmp"
PORT=28464

run() {
#supervisorctl restart apigateway
ssh ${SSH1} -p ${PORT} << EOF
#!/bin/bash
set -e
cp /tmp/pipe6d.tar.gz ~/pipe6
cd ~/pipe6
tar xvzf pipe6d.tar.gz
chmod +x ./pipe6d
#./pipe6d
#echo "waiting..."
#sleep 60
EOF
}

deploy() {
    echo "build pipe6d..."
    rm -f pipe6d.tar.gz
    CGO_ENABLED=0 GOOS=linux GOARCH=386 go build -ldflags "-s -w" -o ./pipe6d "pipe6/cmd/remote"
    tar cvzf pipe6d.tar.gz ./pipe6d ../remote.pem ../remote.key ../local.pem
    echo "copy.."
    scp -P ${PORT} ./pipe6d.tar.gz ${DIR1}/
    rm -f pipe6d.tar.gz
    run
    echo "done!"
}

if [[ $1 == 1 ]]; then
    deploy pipe6d "pipe6/cmd/remote"
elif [[ $1 == 2 ]]; then
    # $2 is linux/windows
    echo "building client for" $2
fi