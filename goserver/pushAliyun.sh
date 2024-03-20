#/bin/bash

version=latest
if [ "$1" != "" ]
then
version=$1
fi

echo "push version $version"

url=registry.cn-hangzhou.aliyuncs.com/loopserver
image=$url/appserver:$version

docker tag appserver:$version $image
docker push $image
docker rmi $(docker images -f "dangling=true" -q)
