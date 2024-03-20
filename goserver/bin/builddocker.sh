#!/bin/sh

if [ "$1" = "" ]
then
docker build -t appserver .
else
docker build -t appserver:$1 .
fi
