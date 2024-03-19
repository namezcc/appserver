package module

import (
	"goserver/handle"
	"goserver/network"
	"goserver/util"
	"net"

	"github.com/golang/protobuf/proto"
)

const (
	CONN_SERVER = 0
	CONN_CLIENT = 1
)

type serverConnInfo struct {
	cid      int
	userData interface{}
}

type connServer struct {
	mod      *modulebase
	addr     string
	call     network.ReadCallBack
	userData interface{}
}

type msgConn struct {
	c    net.Conn
	call network.ReadCallBack
	isws bool
}

type msgSend struct {
	cid  int
	pack network.Msgpack
}

type netModule struct {
	modulebase
	_connMgr *network.ConnMgr
}

type closeInfo struct {
	cid    int
	active bool
}

func (m *netModule) Init(mgr *moduleMgr) {
	m._mod_mgr = mgr
	m._connMgr = network.NewconnMgr()

	handle.Handlemsg.AddMsgCall(handle.M_CONNECT_SERVER, m.onConnectServer)
	handle.Handlemsg.AddMsgCall(handle.M_ACCEPT_CONN, m.onAcceptConn)
	handle.Handlemsg.AddMsgCall(handle.M_SEND_MSG, m.onSendMsg)
	handle.Handlemsg.AddMsgCall(handle.M_CONN_CLOSE, m.onDoConnClose)
}

func (m *netModule) AfterInit() {

}

func (m *netModule) ConnectServer(addr string, mod *modulebase, readcall network.ReadCallBack, userdata interface{}) {
	d := connServer{
		addr:     addr,
		mod:      mod,
		call:     readcall,
		userData: userdata,
	}
	m.sendMsg(handle.M_CONNECT_SERVER, d)
}

func (m *netModule) AcceptConn(c net.Conn, readcall network.ReadCallBack, ws bool) {
	d := msgConn{
		c:    c,
		call: readcall,
		isws: ws,
	}
	m.sendMsg(handle.M_ACCEPT_CONN, d)
}

func (m *netModule) SendMsg(cid int, mid int, pb proto.Message) {
	buf, e := proto.Marshal(pb)
	if e != nil {
		util.Log_error(e.Error())
		return
	}
	sendbuf := make([]byte, len(buf)+network.HEAD_SIZE)
	pack := &network.Msgpack{}
	pack.Init(cid, mid, sendbuf)
	pack.EncodeMsg(mid, buf)
	m.sendMsg(handle.M_SEND_MSG, pack)
}

func (m *netModule) SendPackMsg(cid int, mid int, pack *network.Msgpack) {
	sendbuf := make([]byte, pack.Size()+network.HEAD_SIZE)
	sendpack := &network.Msgpack{}
	sendpack.Init(cid, mid, sendbuf)
	sendpack.EncodeMsg(mid, pack.GetBuff())
	m.sendMsg(handle.M_SEND_MSG, sendpack)
}

func (m *netModule) BroadPackMsg(cids []int, mid int, pack *network.Msgpack) {
	sendbuf := make([]byte, pack.Size()+network.HEAD_SIZE)
	sendpack := &network.Msgpack{}
	sendpack.Init(0, mid, sendbuf)
	sendpack.SetBroad(cids)
	sendpack.EncodeMsg(mid, pack.GetBuff())
	m.sendMsg(handle.M_SEND_MSG, sendpack)
}

func (m *netModule) onConnectServer(msg *handle.BaseMsg) {
	d := msg.Data.(connServer)

	conn := m._connMgr.ConnectTo(d.addr, d.call, m.onConnClose, CONN_SERVER)
	cid := network.CONN_STATE_DISCONNECT
	if conn != nil {
		conn.SetUserData(d.userData)
		cid = conn.GetConnId()
	}
	sinfo := serverConnInfo{
		cid:      cid,
		userData: d.userData,
	}
	d.mod.sendMsg(handle.M_SERVER_CONNECTED, sinfo)
}

func (m *netModule) onAcceptConn(msg *handle.BaseMsg) {
	d := msg.Data.(msgConn)
	if d.isws {
		m._connMgr.AddWsConn(d.c, d.call, m.onConnClose, CONN_CLIENT)
	} else {
		m._connMgr.AddConn(d.c, d.call, m.onConnClose, CONN_CLIENT)
	}
}

func (m *netModule) onSendMsg(msg *handle.BaseMsg) {
	d := msg.Data.(*network.Msgpack)
	cids := d.BroadCid()
	if len(cids) == 0 {
		m._connMgr.SendMsg(d.ConnId(), d.GetBuff())
	} else {
		for _, v := range cids {
			m._connMgr.SendMsg(v, d.GetBuff())
		}
	}
}

func (m *netModule) onConnClose(cid int) {
	m.sendMsg(handle.M_CONN_CLOSE, closeInfo{cid: cid, active: false})
}

func (m *netModule) CloseConn(cid int) {
	m.sendMsg(handle.M_CONN_CLOSE, closeInfo{cid: cid, active: true})
}

func (m *netModule) CloseConnCallEvent(cid int) {
	m.sendMsg(handle.M_CONN_CLOSE, closeInfo{cid: cid, active: false})
}

func (m *netModule) onDoConnClose(msg *handle.BaseMsg) {
	info := msg.Data.(closeInfo)
	conn := m._connMgr.CloseConn(info.cid)
	if conn != nil && !info.active {
		if conn.GetConnType() == CONN_CLIENT {
			util.EventMgr.CallEvent(util.EV_CONN_CLOSE, info.cid)
		} else {
			sinfo := serverConnInfo{
				cid:      info.cid,
				userData: conn.GetUserData(),
			}
			util.EventMgr.CallEvent(util.EV_SERVER_CLOSE, sinfo)
		}
	}
}
