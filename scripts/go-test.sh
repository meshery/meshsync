#!/bin/zsh

go test -failfast -race -v ./...
echo $?
