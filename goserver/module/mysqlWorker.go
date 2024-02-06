package module

import (
	"goserver/util"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const (
	SQLCMD_NONE = iota
	SQLCMD_FUNC_CALL
	SQLCMD_END
)

type sqlCmd struct {
	cid      int
	chanback chan interface{}
	data     interface{}
	backdata bool
	block    bool
}

type MysqlCmdCall func(interface{}, *sqlCmd)

type MysqlWorker struct {
	_sql     *gorm.DB
	_cmdlist chan *sqlCmd
	_cmdCall []MysqlCmdCall
}

func (m *MysqlWorker) Init(host string) {
	m._cmdlist = make(chan *sqlCmd, 1000)
	m._cmdCall = make([]MysqlCmdCall, SQLCMD_END)

	m.bindCmdFunc(SQLCMD_FUNC_CALL, m.cmdFuncCall)

	m.initSql(host)
	m.AfterInit()
}

func (m *MysqlWorker) AfterInit() {
	m.runCmd()
}

func (m *MysqlWorker) initSql(host string) {
	godb, err := gorm.Open(mysql.Open(host), &gorm.Config{})
	if err != nil {
		panic(err.Error())
	}

	sql, err := godb.DB()
	if err != nil {
		panic(err.Error())
	}

	util.Log_info("mysql connect %s", host)

	sql.SetMaxOpenConns(20)
	// 闲置连接数
	sql.SetMaxIdleConns(10)
	// 最大连接周期
	sql.SetConnMaxLifetime(time.Hour)
	m._sql = godb
}

func (m *MysqlWorker) bindCmdFunc(cmdid int, f MysqlCmdCall) {
	m._cmdCall[cmdid] = f
}

func (m *MysqlWorker) runCmd() {
	go func() {
		for cmd := range m._cmdlist {
			call := m._cmdCall[cmd.cid]
			if call != nil {
				call(cmd.data, cmd)
			} else {
				util.Log_error("mysql cmd call func nil id:%d\n", cmd.cid)
				if cmd.block {
					cmd.chanback <- nil
				}
			}
		}
	}()
}

func (m *MysqlWorker) RequestCmd(cid int, d interface{}, backdata bool, block bool) interface{} {
	c := make(chan interface{}, 1)

	rc := &sqlCmd{
		cid:      cid,
		chanback: c,
		data:     d,
		backdata: backdata,
		block:    block,
	}

	m._cmdlist <- rc

	if block {
		return <-c
	} else {
		return nil
	}
}

type MysqlDynamicFunc func(*gorm.DB) interface{}
type MysqlDynamicFuncNoRes func(*gorm.DB)

func (m *MysqlWorker) cmdFuncCall(d interface{}, cmd *sqlCmd) {
	if cmd.backdata == false {
		df := d.(MysqlDynamicFuncNoRes)
		df(m._sql)
		if cmd.block {
			cmd.chanback <- nil
		}
	} else {
		df := d.(MysqlDynamicFunc)
		res := df(m._sql)
		cmd.chanback <- res
	}
}
