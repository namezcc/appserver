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

startserver appService 1

else
startserver $1 $2
fi
