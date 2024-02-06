#/bin/bash

version=latest
if [ "$1" != "" ]
then
version=$1
fi

echo "push version $version"

docker tag goserver:$version localhost:8008/goserver:$version
docker push localhost:8008/goserver:$version
docker rmi $(docker images -f "dangling=true" -q)
