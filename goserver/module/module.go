package module

import (
	"goserver/handle"
	"goserver/interface_func"
	"goserver/network"
	"net"
	"time"
)

type module interface {
	interface_func.Imodule
	InitMsgSize(int)
	Init(*moduleMgr)
	AfterInit()
	BeforRun()
	Run()
	GetModuleBase() *modulebase
}

type timeCall struct {
	d    int64
	call func(int64)
}

type eventCallData struct {
	d    interface{}
	call interface_func.EventCall
}

type modulebase struct {
	_mod_mgr  *moduleMgr
	_msg_chan chan *handle.BaseMsg
}

var defalut_msg_size int = 100

func (m *modulebase) InitMsgSize(s int) {
	m._msg_chan = make(chan *handle.BaseMsg, s)

	handle.Handlemsg.AddMsgCall(handle.M_TIME_CALL, m.onTimeCall)
	handle.Handlemsg.AddMsgCall(handle.M_EVENT_CALL, m.onEventCall)
}

func (m *modulebase) BeforRun() {}
func (m *modulebase) GetModuleBase() *modulebase {
	return m
}

func (m *modulebase) SendEvent(f interface_func.EventCall, d interface{}) {
	m.sendMsg(handle.M_EVENT_CALL, eventCallData{
		d:    d,
		call: f,
	})
}

func (m *modulebase) sendMsg(mid int, d interface{}) {
	m._msg_chan <- &handle.BaseMsg{
		Mid:  mid,
		Data: d,
	}
}

func (m *modulebase) onReadConnBuff(cid int, mid int, buf []byte, conn *net.Conn) {
	pack := &network.Msgpack{}
	pack.Init(cid, mid, buf)
	pack.SetConn(conn)
	m.sendMsg(mid, pack)
}

func (m *modulebase) onTimeCall(msg *handle.BaseMsg) {
	d := msg.Data.(timeCall)
	d.call(d.d)
}

func (m *modulebase) onEventCall(msg *handle.BaseMsg) {
	d := msg.Data.(eventCallData)
	d.call(d.d)
}

func (m *modulebase) setAfterFunc(d time.Duration, call func(int64)) {
	go func() {
		c := time.After(d)
		dt := <-c
		msg := timeCall{
			d:    dt.UnixNano() / 1e6,
			call: call,
		}
		m.sendMsg(handle.M_TIME_CALL, msg)
	}()
}

func (m *modulebase) setTikerFunc(d time.Duration, call func(int64)) {
	go func() {
		for v := range time.Tick(d) {
			msg := timeCall{
				d:    v.UnixNano() / 1e6,
				call: call,
			}
			m.sendMsg(handle.M_TIME_CALL, msg)
		}
	}()
}

func (m *modulebase) Run() {
	go func() {
		for msg := range m._msg_chan {
			handle.Handlemsg.CallHandle(msg)
		}
	}()
}
