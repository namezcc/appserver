package module

import "goserver/handle"

const (
	MOD_LOGIC = iota
	MOD_NET
	MOD_HTTP
	MOD_MONITOR_SERVER
	MOD_MASTER
	MOD_OPERATE
	MOD_REDIS_MGR
	MOD_MYSQL_MGR
	MOD_MONGO_MGR
	MOD_CLIENT_SERVER
	MOD_END
)

type moduleMgr struct {
	_mod map[int]module
}

var ModuleMgr moduleMgr

func (m *moduleMgr) Init() {
	m._mod = make(map[int]module)

	m._mod[MOD_NET] = &netModule{}
}

func (m *moduleMgr) InitMsg(msgbegin int, msgend int) {
	handle.Handlemsg.InitMsg(msgbegin, msgend)
}

func (m *moduleMgr) AddModule(mtype int, mod module) {
	m._mod[mtype] = mod
}

func (m *moduleMgr) StartRun() {
	for _, v := range m._mod {
		v.InitMsgSize(defalut_msg_size)
		v.Init(m)
	}

	for _, v := range m._mod {
		v.AfterInit()
	}

	for _, v := range m._mod {
		v.BeforRun()
	}

	for _, v := range m._mod {
		v.Run()
	}
}

func (m *moduleMgr) GetModule(mid int) module {
	return m._mod[mid]
}
