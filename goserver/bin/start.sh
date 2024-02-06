#!/bin/sh

_dir=pids
_stdlog=stdlog

if [[ ! -d "$_dir" ]] ;then
		mkdir $_dir
		echo "makedir $_dir"
fi

if [[ ! -d "$_stdlog" ]] ;then
		mkdir $_stdlog
fi

function startserver()
{
	chmod 777 $1
	nohup ./$1 $2 > $_stdlog/$1_$2.log 2>&1 &
	echo $!> $_dir/$1_$2.pid
	sleep 2
}

if [[ "$1" == "" || "$2" == "" ]]
then

startserver service_find 1
startserver service_find 2
startserver service_find 3
startserver k8service 1
startserver monitor 1
startserver monitor 2
startserver clientService 1
startserver loginService 1
startserver master 1

else
startserver $1 $2
fi
