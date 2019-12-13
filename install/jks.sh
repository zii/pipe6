#!/bin/bash
# pem to jks
set -e

password=""

while getopts "p:" arg
do
    case $arg in
        p)
            password=$OPTARG
            ;;
        ?)
            echo "usage: jks.sh -p <PASSWORD>"
            exit 1
            ;;
    esac
done

if [[ $password == "" ]]; then
    echo "usage: jks.sh -p <PASSWORD>"
    exit 1
fi

rm -f ./local.jks

openssl pkcs12 -export -in local.pem -inkey local.key -out local.p12 -passout pass:${password}
set +e
keytool -importkeystore -srckeystore local.p12 -srcstoretype PKCS12 -destkeystore local.jks -srcstorepass ${password} -deststorepass ${password}

rm -f ./local.p12
