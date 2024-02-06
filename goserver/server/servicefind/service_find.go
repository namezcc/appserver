package main

import (
	"goserver/handle"
	"goserver/module"
	"goserver/util"
	"time"
)

func main() {
	util.ParseArgsId(util.ST_SERVICE_FIND)
	util.Init_loger("service_find")

	util.Readconf("conf.ini")

	module.ModuleMgr.Init()
	util.Log_info("start service find")

	module.ModuleMgr.InitMsg(handle.MS_BEGIN, handle.MS_END)

	module.ModuleMgr.AddModule(module.MOD_MASTER, &module.ServiceFindModule{})
	module.ModuleMgr.AddModule(module.MOD_HTTP, &module.ServiceFindHttpModule{})

	module.ModuleMgr.StartRun()
	for range time.Tick(time.Second * 60) {

	}
}
