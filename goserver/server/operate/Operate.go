package main

import (
	"goserver/handle"
	"goserver/module"
	"goserver/util"
	"time"
)

func main() {
	util.ParseArgsId(util.ST_OPERATE)
	util.Init_loger("operate")

	util.Readconf("conf.ini")

	module.ModuleMgr.Init()
	util.Log_info("start operate")

	module.ModuleMgr.InitMsg(handle.MS_BEGIN, handle.MS_END)

	module.ModuleMgr.AddModule(module.MOD_OPERATE, &module.OperateModule{})
	module.ModuleMgr.AddModule(module.MOD_HTTP, &module.OperateHttpModule{})

	module.ModuleMgr.StartRun()
	for range time.Tick(time.Second * 60) {

	}
}
