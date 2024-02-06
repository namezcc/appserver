package main

import (
	"goserver/handle"
	"goserver/module"
	"goserver/util"
	"time"
)

func main() {
	util.ParseArgsId(util.ST_LOGIN_SERVICE)
	util.Init_loger("loginService")

	util.Readconf("conf.ini")

	module.ModuleMgr.Init()
	util.Log_info("start login Service find")

	module.ModuleMgr.InitMsg(handle.MS_BEGIN, handle.MS_END)

	module.ModuleMgr.AddModule(module.MOD_HTTP, &module.LoginHttpModule{})

	module.ModuleMgr.StartRun()
	for range time.Tick(time.Second * 60) {

	}
}
