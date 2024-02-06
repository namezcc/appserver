package main

import (
	"goserver/handle"
	"goserver/module"
	"goserver/util"
	"time"
)

func main() {
	util.ParseArgsId(util.ST_MASTER)
	util.Init_loger("master")

	util.Readconf("conf.ini")

	module.ModuleMgr.Init()
	util.Log_info("start master")

	module.ModuleMgr.InitMsg(handle.MS_BEGIN, handle.MS_END)

	module.ModuleMgr.AddModule(module.MOD_MASTER, &module.MasterModule{})
	module.ModuleMgr.AddModule(module.MOD_HTTP, &module.MasterHttpModule{})

	module.ModuleMgr.StartRun()
	for range time.Tick(time.Second * 60) {

	}
}
