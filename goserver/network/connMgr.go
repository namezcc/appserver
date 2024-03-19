package network

import (
	"goserver/util"
	"net"
)

type HandleFunc func(Msgpack)

type connMgr struct {
	_connvec     []connI
	_connId_pool []int
	_pool_index  int
}

type ConnMgr struct {
	connMgr
}

func NewconnMgr() *ConnMgr {
	mgr := ConnMgr{}

	mgr._pool_index = 0
	mgr._connvec = make([]connI, 1000)
	mgr._connId_pool = make([]int, 1000)

	for i := 0; i < 1000; i++ {
		mgr._connId_pool[i] = i
	}

	return &mgr
}

func (_m *connMgr) popConnId() int {
	_m._pool_index++
	return _m._connId_pool[_m._pool_index-1]
}

func (_m *connMgr) pushConnId(id int) {
	_m._pool_index--
	_m._connId_pool[_m._pool_index] = id
}

func (_m *connMgr) AddConn(c net.Conn, readcall ReadCallBack, closecall onClose, _ct int) *Tcpconn {
	cid := _m.popConnId()
	conn := newTcpconn(cid, c, closecall, _ct)
	conn.StartRead(readcall)
	_m._connvec[cid] = conn
	return conn
}

func (_m *connMgr) AddWsConn(c net.Conn, readcall ReadCallBack, closecall onClose, _ct int) {
	cid := _m.popConnId()
	conn := newWsconn(cid, c, closecall, _ct)
	conn.StartRead(readcall)
	_m._connvec[cid] = conn
}

func (_m *connMgr) CloseConn(cid int) connI {
	conn := _m._connvec[cid]
	if conn == nil {
		util.Log_error("error close conn nil")
		return nil
	}

	_m._connvec[cid] = nil
	_m.pushConnId(cid)
	conn.Close()
	return conn
}

func (_m *connMgr) ConnectTo(addr string, readcall ReadCallBack, closecall onClose, _ct int) *Tcpconn {
	conn, e := net.Dial("tcp", addr)
	if e != nil {
		util.Log_error(e.Error())
		return nil
	}

	return _m.AddConn(conn, readcall, closecall, _ct)
}

func (_m *connMgr) SendMsg(cid int, buf []byte) {
	conn := _m._connvec[cid]
	if conn == nil {
		return
	}
	conn.Write(buf)
}
