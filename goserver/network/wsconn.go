package network

import (
	"goserver/util"
	"net"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

type Wsconn struct {
	_conn      net.Conn
	_sendchan  chan []byte
	_connId    int
	_close     bool
	_onClose   onClose
	_type      int
	_udata     interface{}
	_read_data []byte
	_have_err  bool
}

func newWsconn(id int, conn net.Conn, _c onClose, _ct int) *Wsconn {
	_tc := new(Wsconn)
	_tc._conn = conn
	_tc._connId = id
	_tc._sendchan = make(chan []byte, 100)
	_tc._close = false
	_tc._onClose = _c
	_tc._type = _ct
	_tc._read_data = make([]byte, 0, 1024)
	_tc._have_err = false

	go func() {
		for s := range _tc._sendchan {
			// _, err := conn.Write(s)
			err := wsutil.WriteServerMessage(conn, ws.OpBinary, s)
			if err != nil {
				util.Log_error("ws write data %s", err.Error())
				break
			}
		}
		// 设置报错，清空chan
		_tc.clearSendChan()

		if !_tc._close {
			_tc._onClose(_tc._connId)
		}
	}()

	return _tc
}

func (_tc *Wsconn) clearSendChan() {
	_tc._have_err = true
	if len(_tc._sendchan) > 0 {
		<-_tc._sendchan
	}
}

func (_tc *Wsconn) StartRead(_call ReadCallBack) {
	go func() {
		for {
			msg, _, err := wsutil.ReadClientData(_tc._conn)
			if err != nil {
				util.Log_error("ws read data %s", err.Error())
				break
			}

			_tc._read_data = append(_tc._read_data, msg...)

			haveErr := false
			readlen := 0
			for len(_tc._read_data)-readlen >= HEAD_SIZE {
				head := _tc._read_data[readlen:(readlen + HEAD_SIZE)]

				pack := Msgpack{}
				pack.Init(0, 0, head)
				_s := pack.ReadInt32() - HEAD_SIZE
				mid := pack.ReadInt32()

				if _s < 0 || _s > 655350 {
					util.Log_error("ws read size too big:%d", _s)
					haveErr = true
					break
				}

				buff := make([]byte, _s)
				if _s > 0 {
					si := readlen + HEAD_SIZE
					copy(buff, _tc._read_data[si:si+int(_s)])
				}
				readlen += HEAD_SIZE + int(_s)
				_call(_tc._connId, int(mid), buff, &_tc._conn)
			}
			if haveErr {
				break
			}
			if readlen > 0 {
				_tc._read_data = append(_tc._read_data[:0], _tc._read_data[readlen:]...)
			}
		}
		// 设置报错，清空chan
		_tc.clearSendChan()
		if !_tc._close {
			_tc._onClose(_tc._connId)
		}
	}()
}

func (_tc *Wsconn) Write(b []byte) {
	if _tc._have_err {
		return
	}
	_tc._sendchan <- b
}

func (_tc *Wsconn) Close() {
	_tc._close = true
	_tc._conn.Close()
	close(_tc._sendchan)
}

func (_tc *Wsconn) GetConnType() int {
	return _tc._type
}

func (_tc *Wsconn) SetUserData(ud interface{}) {
	_tc._udata = ud
}

func (_tc *Wsconn) GetUserData() interface{} {
	return _tc._udata
}

func (_tc *Wsconn) GetConnId() int {
	return _tc._connId
}
