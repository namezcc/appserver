#!/bin/sh

cd goserver/bin

go build -gcflags=all="-N -l" ../server/master/master.go
echo "build master finish"
