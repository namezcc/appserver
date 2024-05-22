package main

import (
	"goserver/handle"
	"goserver/module"
	"goserver/util"
	"math/rand"
	"time"
)

func main() {
	util.ParseArgsId(util.ST_USER_MANAGER)
	util.Init_loger("userManager")

	util.Readconf("conf.ini")
	// 随机数种子
	rand.Seed(util.GetMillisecond())
	module.ModuleMgr.Init()
	util.Log_info("start userManager")

	module.ModuleMgr.InitMsg(handle.MS_BEGIN, handle.MS_END)

	module.ModuleMgr.AddModule(module.MOD_HTTP, &module.UserManagerModule{})
	// module.ModuleMgr.AddModule(module.MOD_REDIS_MGR, &module.RedisManagerModule{})
	module.ModuleMgr.AddModule(module.MOD_MYSQL_MGR, &module.MysqlManagerModule{
		HostKey: "mysql",
	})
	module.ModuleMgr.AddModule(module.MOD_MONGO_MGR, &module.MongoManagerModule{})
	// module.ModuleMgr.AddModule(module.MOD_CLIENT_SERVER, &module.ClientServerModule{})

	module.ModuleMgr.StartRun()
	for range time.Tick(time.Second * 60) {

	}
}
