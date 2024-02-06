#/bin/bash

version=latest
if [ "$1" != "" ]
then
version=$1
fi

echo "push version $version"

url=registry.cn-hangzhou.aliyuncs.com/loopserver
image=$url/goserver:$version

docker tag goserver:$version $image
docker push $image
docker rmi $(docker images -f "dangling=true" -q)
