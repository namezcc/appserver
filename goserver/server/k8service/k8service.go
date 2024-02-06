package main

import (
	"goserver/handle"
	"goserver/module"
	"goserver/util"
	"time"
)

func main() {
	util.ParseArgsId(util.ST_K8SERVICE)
	util.Init_loger("k8service")

	util.Readconf("conf.ini")

	module.ModuleMgr.Init()
	util.Log_info("start client Service find")

	module.ModuleMgr.InitMsg(handle.MS_BEGIN, handle.MS_END)

	module.ModuleMgr.AddModule(module.MOD_HTTP, &module.K8HttpModule{})

	module.ModuleMgr.StartRun()
	for range time.Tick(time.Second * 60) {

	}
}
