package module

import (
	"context"
	"goserver/util"
	"time"

	"github.com/qiniu/qmgo"
)

const (
	MGCMD_NONE = iota
	MGCMD_FUNC_CALL
	MGCMD_END
)

type mongoCmd struct {
	cid      int
	chanback chan interface{}
	data     interface{}
	backdata bool
	block    bool
}

type MongoCmdCall func(context.Context, interface{}, *mongoCmd)

type MongoWorker struct {
	_client  *qmgo.QmgoClient
	_cmdlist chan *mongoCmd
	_cmdCall []MongoCmdCall
}

func (m *MongoWorker) Init(host string) {
	m._cmdlist = make(chan *mongoCmd, 1000)
	m._cmdCall = make([]MongoCmdCall, MGCMD_END)

	m.bindCmdFunc(MGCMD_FUNC_CALL, m.cmdFuncCall)

	m.initClient(host)
	m.AfterInit()
}

func (m *MongoWorker) AfterInit() {
	m.runCmd()
}

func (m *MongoWorker) initClient(host string) {
	ctx, cancle := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancle()
	cli, err := qmgo.Open(ctx, &qmgo.Config{Uri: host, Database: util.GetConfValue("mongo-db")})
	if err != nil {
		panic(err)
	}
	m._client = cli

	err = cli.Ping(3)
	if err != nil {
		panic(err)
	}
	util.Log_info("mongodb connect %s", host)
}

func (m *MongoWorker) bindCmdFunc(cmdid int, f MongoCmdCall) {
	m._cmdCall[cmdid] = f
}

func (m *MongoWorker) runCmd() {
	go func() {
		for cmd := range m._cmdlist {
			call := m._cmdCall[cmd.cid]
			if call != nil {
				m.callCmd(cmd, call)
			} else {
				util.Log_error("mongo cmd call func nil id:%d\n", cmd.cid)
				if cmd.block {
					cmd.chanback <- nil
				}
			}
		}
	}()
}

func (m *MongoWorker) callCmd(cmd *mongoCmd, call MongoCmdCall) {
	ctx, cancle := context.WithTimeout(context.Background(), 500*time.Second)
	defer cancle()
	call(ctx, cmd.data, cmd)
}

func (m *MongoWorker) RequestCmd(cid int, d interface{}, backdata bool, block bool) interface{} {
	c := make(chan interface{}, 1)

	rc := &mongoCmd{
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

type MongoDynamicFunc func(context.Context, *qmgo.QmgoClient) interface{}
type MongoDynamicFuncNoRes func(context.Context, *qmgo.QmgoClient)

func (m *MongoWorker) cmdFuncCall(ctx context.Context, d interface{}, cmd *mongoCmd) {
	if cmd.backdata == false {
		df := d.(MongoDynamicFuncNoRes)
		df(ctx, m._client)
		if cmd.block {
			cmd.chanback <- nil
		}
	} else {
		df := d.(MongoDynamicFunc)
		res := df(ctx, m._client)
		cmd.chanback <- res
	}
}
