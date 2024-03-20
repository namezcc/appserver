#/bin/bash

version=latest
if [ "$1" != "" ]
then
version=$1
fi

echo "push version $version"

docker tag appserver:$version localhost:8008/appserver:$version
docker push localhost:8008/appserver:$version
docker rmi $(docker images -f "dangling=true" -q)
