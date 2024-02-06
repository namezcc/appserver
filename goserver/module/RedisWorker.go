package module

import (
	"context"
	"goserver/util"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	RCMD_NONE = iota
	RCMD_FUNC_CALL
	RCMD_END
)

type redisCmd struct {
	cid      int
	chanback chan interface{}
	data     interface{}
	backdata bool
	block    bool
}

type RedisCmdCall func(context.Context, interface{}, *redisCmd)

type RedisWorker struct {
	_rdb     *redis.Client
	_cmdlist chan *redisCmd
	_cmdCall []RedisCmdCall
}

func (m *RedisWorker) Init() {
	m._cmdlist = make(chan *redisCmd, 1000)
	m._cmdCall = make([]RedisCmdCall, RCMD_END)

	m.bindCmdFunc(RCMD_FUNC_CALL, m.cmdFuncCall)

	m.AfterInit()
}

func (m *RedisWorker) AfterInit() {
	m.initRedis()
	m.runCmd()
}

func (m *RedisWorker) initRedis() {
	rhost := util.GetConfValue("redis-host")
	rpass := util.GetConfValue("redis-pass")
	rdb := util.GetConfInt("redis-db")

	m._rdb = redis.NewClient(&redis.Options{
		Addr:     rhost,
		Password: rpass,
		DB:       rdb,
	})

	doRedis(func(ctx context.Context) {
		_, err := m._rdb.Ping(ctx).Result()
		if err != nil {
			panic(err)
		}
	})
	util.Log_info("redis connect %s\n", rhost)
}

func (m *RedisWorker) bindCmdFunc(cmdid int, f RedisCmdCall) {
	m._cmdCall[cmdid] = f
}

func (m *RedisWorker) runCmd() {
	go func() {
		for cmd := range m._cmdlist {
			call := m._cmdCall[cmd.cid]
			if call != nil {
				m.callCmd(cmd, call)
			} else {
				util.Log_error("redis cmd call func nil id:%d\n", cmd.cid)
				if cmd.block {
					cmd.chanback <- nil
				}
			}
		}
	}()
}

func (m *RedisWorker) callCmd(cmd *redisCmd, call RedisCmdCall) {
	ctx, cancle := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancle()
	call(ctx, cmd.data, cmd)
}

func (m *RedisWorker) RequestCmd(cid int, d interface{}, backdata bool, block bool) interface{} {
	c := make(chan interface{}, 1)

	rc := &redisCmd{
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

type RedisDynamicFunc func(context.Context, *redis.Client) interface{}
type RedisDynamicFuncNoRes func(context.Context, *redis.Client)

func (m *RedisWorker) cmdFuncCall(ctx context.Context, d interface{}, cmd *redisCmd) {
	if cmd.backdata == false {
		df := d.(RedisDynamicFuncNoRes)
		df(ctx, m._rdb)
		if cmd.block {
			cmd.chanback <- nil
		}
	} else {
		df := d.(RedisDynamicFunc)
		res := df(ctx, m._rdb)
		cmd.chanback <- res
	}
}
