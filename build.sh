#!/bin/bash

# shellcheck disable=SC2034
PROJECT_NAME="search"
DIR_NAME="search-service"

# 取得時間和Git commit hash
BUILD_TIME=$(date +"%Y-%m-%dT%H:%M:%S.%3N%:z")
GIT_COMMIT=$(git rev-parse HEAD)

if [ -f "./$PROJECT_NAME" ]; then
    rm ./$PROJECT_NAME
fi

if [ -d "./output" ]; then
    rm -rf ./output
fi

# 執行 go build 並將變數注入
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X '$DIR_NAME/config.BuildTime=$BUILD_TIME' -X '$DIR_NAME/config.BuildHash=$GIT_COMMIT'" -o $PROJECT_NAME main.go

docker image build -t $PROJECT_NAME:$1 .

docker image save -o ${PROJECT_NAME}_$1.tar $PROJECT_NAME:$1

sshpass -p 'F7)q#vBPSRA%e+d,' scp -P 22 /home/pablo/projects/p03-tgbot-search-service/${PROJECT_NAME}_$1.tar linuxuser@149.28.155.179:/tmp/

rm ${PROJECT_NAME}_$1.tar