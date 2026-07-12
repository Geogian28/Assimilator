#!/bin/bash

if ! command -v protoc &>/dev/null ; then
    echo "to Sam: protoc isnt installed. Install with one of these:"
    echo -e " - \e[31mDebian\e[0m: \e[1msudo apt update && sudo apt install -y protobuf-compiler\e[0m"
    echo -e " - \e[31mFedora\e[0m: \e[1msudo dnf install -y protobuf-compiler\e[0m"
    exit
fi

protoc --go_out=. \
    --go_opt=paths=source_relative \
    --go-grpc_out=. \
    --go-grpc_opt=paths=source_relative \
    assctl.proto
