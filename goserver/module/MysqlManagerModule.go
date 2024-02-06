package module

import "goserver/util"

type MysqlManagerModule struct {
	modulebase
	_mysqlmod   []*MysqlWorker
	_work_index int
}

const MYSQL_WOKER_NUM = 5

func (m *MysqlManagerModule) Init(*moduleMgr) {
	m._mysqlmod = make([]*MysqlWorker, 0)
	m._work_index = 0
	host := util.GetConfValue("mysql")
	for i := 0; i < MYSQL_WOKER_NUM; i++ {
		rdm := MysqlWorker{}
		rdm.Init(host)
		m._mysqlmod = append(m._mysqlmod, &rdm)
	}

}

func (m *MysqlManagerModule) AfterInit() {

}

func (m *MysqlManagerModule) rangeWokerIndex() int {
	rid := m._work_index
	m._work_index++
	if m._work_index >= MYSQL_WOKER_NUM {
		m._work_index = 0
	}
	return rid
}

// 阻塞执行
func (m *MysqlManagerModule) Request(cmdid int, d interface{}, backdata bool) interface{} {
	rid := m.rangeWokerIndex()
	return m._mysqlmod[rid].RequestCmd(cmdid, d, backdata, true)
}

// 阻塞执行
func (m *MysqlManagerModule) RequestHash(hashval uint32, cmdid int, d interface{}, backdata bool) interface{} {
	var rid int
	if hashval == 0 {
		rid = int(hashval % MYSQL_WOKER_NUM)
	} else {
		rid = m.rangeWokerIndex()
	}
	return m._mysqlmod[rid].RequestCmd(cmdid, d, backdata, true)
}

// 不阻塞函数
func (m *MysqlManagerModule) WorkNoBlock(cmdid int, d interface{}) {
	rid := m.rangeWokerIndex()
	m._mysqlmod[rid].RequestCmd(cmdid, d, false, false)
}

// 不阻塞函数
func (m *MysqlManagerModule) WorkNoBlockHash(hashval uint32, cmdid int, d interface{}) {
	var rid int
	if hashval == 0 {
		rid = int(hashval % MYSQL_WOKER_NUM)
	} else {
		rid = m.rangeWokerIndex()
	}
	m._mysqlmod[rid].RequestCmd(cmdid, d, false, false)
}

// 阻塞函数,有返回值
func (m *MysqlManagerModule) RequestFuncCall(f MysqlDynamicFunc) interface{} {
	return m.Request(SQLCMD_FUNC_CALL, f, true)
}

// 阻塞函数,有返回值
func (m *MysqlManagerModule) RequestFuncCallHash(hashval uint32, f MysqlDynamicFunc) interface{} {
	return m.RequestHash(hashval, SQLCMD_FUNC_CALL, f, true)
}

// 阻塞函数,无返回值
func (m *MysqlManagerModule) RequestFuncCallNoRes(f MysqlDynamicFuncNoRes) {
	m.Request(SQLCMD_FUNC_CALL, f, false)
}

// 阻塞函数,无返回值
func (m *MysqlManagerModule) RequestFuncCallNoResHash(hashval uint32, f MysqlDynamicFuncNoRes) {
	m.RequestHash(hashval, SQLCMD_FUNC_CALL, f, false)
}

// 不阻塞函数,不知道是否有闭包问题,参数提前被释放
func (m *MysqlManagerModule) WorkFuncCallNoBlock(f MysqlDynamicFuncNoRes) {
	m.WorkNoBlock(SQLCMD_FUNC_CALL, f)
}

// 不阻塞函数,不知道是否有闭包问题,参数提前被释放
func (m *MysqlManagerModule) WorkFuncCallNoBlockHash(hashval uint32, f MysqlDynamicFuncNoRes) {
	m.WorkNoBlockHash(hashval, SQLCMD_FUNC_CALL, f)
}
