package module

import (
	"goserver/handle"
	"goserver/network"
	"goserver/util"
	"os"
)

type OperateModule struct {
	ServerModule
	_http_mod *OperateHttpModule
}

func (m *OperateModule) Init(mgr *moduleMgr) {
	m.ServerModule.Init(mgr)

	m._http_mod = mgr.GetModule(MOD_HTTP).(*OperateHttpModule)

	handle.Handlemsg.AddMsgCall(handle.M_SEND_BAN_INFO, m.onHttpBanInfo)
}

func (m *OperateModule) AfterInit() {
	m.ServerModule.AfterInit()
	if !m.getConfigFromHttp(util.ST_OPERATE, 1) {
		os.Exit(1)
	}
}

func (m *OperateModule) onHttpBanInfo(msg *handle.BaseMsg) {
	p := msg.Data.(*network.Msgpack)
	m.sendServer(util.ST_MASTER, 1, handle.N_SEND_BAN_INFO, p)
}
