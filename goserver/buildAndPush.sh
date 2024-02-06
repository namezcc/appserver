#!/bin/bash

version=$1

if [ "$version" = "" ]
then
	echo "need version"
	exit
fi

git pull

make

cd bin

sh builddocker.sh $version

cd ..

sh pushAliyun.sh $version
