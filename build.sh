#!/bin/sh

CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o robot-linux main.go