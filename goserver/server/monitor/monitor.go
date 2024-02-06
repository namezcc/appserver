package main

import (
	"goserver/handle"
	"goserver/module"
	"goserver/util"
	"time"
)

func main() {
	util.ParseArgsId(util.ST_MONITOR)
	util.Init_loger("monitor")

	util.Readconf("conf.ini")

	module.ModuleMgr.Init()
	util.Log_info("start monitor find")

	module.ModuleMgr.InitMsg(handle.MS_BEGIN, handle.MS_END)

	module.ModuleMgr.AddModule(module.MOD_MONITOR_SERVER, &module.MonitorModule{})

	module.ModuleMgr.StartRun()
	for range time.Tick(time.Second * 60) {

	}
}
