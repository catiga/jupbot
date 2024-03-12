#!/bin/sh

HOME="/home/ec2-user/shelfrobot"

export DALINK_GO_CONFIG_PATH="$HOME/prod.yml"

nohup ./robot-linux &


# GOOS=linux GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-linux-gnu-gcc go build -o robot-linux
# CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o robot-linux main.go


# GOOS=linux GOARCH=amd64 CGO_ENABLED=1  go build -o robot-linux