#!/bin/sh
mkdir -p jsback
\cp ../../../../../_out/bin/commonconf/* jsback/

if [ "$1" = "" ]
then
docker build -t goserver .
else
docker build -t goserver:$1 .
fi
