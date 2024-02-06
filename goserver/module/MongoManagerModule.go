package module

import "goserver/util"

type MongoManagerModule struct {
	modulebase
	_worker     []*MongoWorker
	_work_index int
}

const MONGO_WOKER_NUM = 5

func (m *MongoManagerModule) Init(*moduleMgr) {
	m._worker = make([]*MongoWorker, 0)
	m._work_index = 0
	host := util.GetConfValue("mongo-host")
	for i := 0; i < MONGO_WOKER_NUM; i++ {
		rdm := MongoWorker{}
		rdm.Init(host)
		m._worker = append(m._worker, &rdm)
	}

}

func (m *MongoManagerModule) AfterInit() {

}

func (m *MongoManagerModule) rangeWokerIndex() int {
	rid := m._work_index
	m._work_index++
	if m._work_index >= MONGO_WOKER_NUM {
		m._work_index = 0
	}
	return rid
}

// 阻塞执行
func (m *MongoManagerModule) Request(cmdid int, d interface{}, backdata bool) interface{} {
	rid := m.rangeWokerIndex()
	return m._worker[rid].RequestCmd(cmdid, d, backdata, true)
}

// 阻塞执行
func (m *MongoManagerModule) RequestHash(hashval uint32, cmdid int, d interface{}, backdata bool) interface{} {
	var rid int
	if hashval == 0 {
		rid = int(hashval % MONGO_WOKER_NUM)
	} else {
		rid = m.rangeWokerIndex()
	}
	return m._worker[rid].RequestCmd(cmdid, d, backdata, true)
}

// 不阻塞函数
func (m *MongoManagerModule) WorkNoBlock(cmdid int, d interface{}) {
	rid := m.rangeWokerIndex()
	m._worker[rid].RequestCmd(cmdid, d, false, false)
}

// 不阻塞函数
func (m *MongoManagerModule) WorkNoBlockHash(hashval uint32, cmdid int, d interface{}) {
	var rid int
	if hashval == 0 {
		rid = int(hashval % MONGO_WOKER_NUM)
	} else {
		rid = m.rangeWokerIndex()
	}
	m._worker[rid].RequestCmd(cmdid, d, false, false)
}

// 阻塞函数,有返回值
func (m *MongoManagerModule) RequestFuncCall(f MongoDynamicFunc) interface{} {
	return m.Request(MGCMD_FUNC_CALL, f, true)
}

// 阻塞函数,有返回值
func (m *MongoManagerModule) RequestFuncCallHash(hashval uint32, f MongoDynamicFunc) interface{} {
	return m.RequestHash(hashval, MGCMD_FUNC_CALL, f, true)
}

// 阻塞函数,无返回值
func (m *MongoManagerModule) RequestFuncCallNoRes(f MongoDynamicFuncNoRes) {
	m.Request(MGCMD_FUNC_CALL, f, false)
}

// 阻塞函数,无返回值
func (m *MongoManagerModule) RequestFuncCallNoResHash(hashval uint32, f MongoDynamicFuncNoRes) {
	m.RequestHash(hashval, MGCMD_FUNC_CALL, f, false)
}

// 不阻塞函数,不知道是否有闭包问题,参数提前被释放
func (m *MongoManagerModule) WorkFuncCallNoBlock(f MongoDynamicFuncNoRes) {
	m.WorkNoBlock(MGCMD_FUNC_CALL, f)
}

// 不阻塞函数,不知道是否有闭包问题,参数提前被释放
func (m *MongoManagerModule) WorkFuncCallNoBlockHash(hashval uint32, f MongoDynamicFuncNoRes) {
	m.WorkNoBlockHash(hashval, MGCMD_FUNC_CALL, f)
}
