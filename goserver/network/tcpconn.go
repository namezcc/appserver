package network

import (
	"goserver/util"
	"io"
	"net"
)

const (
	CONN_STATE_DISCONNECT = -1
)

type Tcpconn struct {
	_conn     net.Conn
	_sendchan chan []byte
	_connId   int
	_close    bool
	_onClose  onClose
	_type     int
	_udata    interface{}
}

type ReadCallBack func(int, int, []byte, *net.Conn)
type onClose func(int)

func newTcpconn(id int, conn net.Conn, _c onClose, _ct int) *Tcpconn {
	_tc := new(Tcpconn)
	_tc._conn = conn
	_tc._connId = id
	_tc._sendchan = make(chan []byte, 100)
	_tc._close = false
	_tc._onClose = _c
	_tc._type = _ct

	go func() {
		for s := range _tc._sendchan {
			_, err := conn.Write(s)
			if err != nil {
				break
			}
		}

		if !_tc._close {
			_tc._onClose(_tc._connId)
		}
	}()

	return _tc
}

func (_tc *Tcpconn) StartRead(_call ReadCallBack) {
	go func() {
		head := make([]byte, HEAD_SIZE)
		for {
			_, e := io.ReadFull(_tc._conn, head)
			if e != nil {
				util.Log_error(e.Error())
				break
			}

			pack := Msgpack{}
			pack.Init(0, 0, head)
			_s := pack.ReadInt32() - HEAD_SIZE
			mid := pack.ReadInt32()

			if _s < 0 || _s > 655350 {
				util.Log_error("read size too big:%d", _s)
				break
			}

			buff := make([]byte, _s)
			if _s > 0 {
				_, e2 := io.ReadFull(_tc._conn, buff)
				if e2 != nil {
					util.Log_error(e2.Error())
					break
				}
			}
			_call(_tc._connId, int(mid), buff, &_tc._conn)
		}

		if !_tc._close {
			_tc._onClose(_tc._connId)
		}
	}()
}

func (_tc *Tcpconn) Write(b []byte) {
	_tc._sendchan <- b
}

func (_tc *Tcpconn) Close() {
	_tc._close = true
	_tc._conn.Close()
	close(_tc._sendchan)
}

func (_tc *Tcpconn) GetConnType() int {
	return _tc._type
}

func (_tc *Tcpconn) SetUserData(ud interface{}) {
	_tc._udata = ud
}

func (_tc *Tcpconn) GetUserData() interface{} {
	return _tc._udata
}

func (_tc *Tcpconn) GetConnId() int {
	return _tc._connId
}
