package module

import (
	"goserver/LPMsg"
	"goserver/handle"
	"goserver/network"
	"goserver/util"
	"os"

	"github.com/golang/protobuf/proto"
)

type serverStateInfo struct {
	link map[int]int
}

type MasterModule struct {
	ServerModule
	_http_mod     *MasterHttpModule
	_server_state map[int]*serverStateInfo
}

func (m *MasterModule) Init(mgr *moduleMgr) {
	m.ServerModule.Init(mgr)

	m._server_state = make(map[int]*serverStateInfo)
	m._http_mod = mgr.GetModule(MOD_HTTP).(*MasterHttpModule)

	handle.Handlemsg.AddMsgCall(handle.M_GET_SERVER_STATE, m.onGetServerState)
	handle.Handlemsg.AddMsgCall(handle.M_HOT_LOAD, m.onRoomHotLoad)
	handle.Handlemsg.AddMsgCall(handle.M_NOTICE_DB_NUM_INFO_CHANGE, m.onNoticeDbNumChange)
	handle.Handlemsg.AddMsgCall(handle.M_ADD_SERVER_MAIL, m.onAddServerMail)
	handle.Handlemsg.AddMsgCall(handle.M_ADD_PLAYER_MAIL, m.onAddPlayerMail)

	handle.Handlemsg.AddMsgCall(handle.N_SERVER_LINK_INFO, m.onServerLinkInfo)
	handle.Handlemsg.AddMsgCall(int(LPMsg.IM_MSG_ID_IM_MASTER_GET_SERVER_GROUP_INFO), m.onGetServerGroupInfo)
	handle.Handlemsg.AddMsgCall(handle.N_SEND_BAN_INFO, m.onSendBanInfo)

}

func (m *MasterModule) AfterInit() {
	m.ServerModule.AfterInit()
	if !m.getConfigFromHttp(util.ST_MASTER, 1) {
		os.Exit(1)
	}
}

func (m *MasterModule) onGetServerState(msg *handle.BaseMsg) {
	rep := msg.Data.(*responesData)

	serstate := make(map[int]serverState)
	for _, mv := range m._net_server {
		for _, v := range mv {
			key := (v.servertype << 16) | v.id
			info := m.getServerState(v.servertype, v.id)

			serstate[key] = serverState{
				state: 0,
				addr:  v.addr,
				link:  info.link,
			}
		}
	}

	rep.data = serstate
	m._http_mod.responesMsgData(rep)
}

type hotInfo struct {
	Lua    string `json:"lua"`
	Server []int  `json:"server"`
	Except []int  `json:"except"`
}

func (m *MasterModule) onRoomHotLoad(msg *handle.BaseMsg) {
	smap := m.getServerList(util.ST_ROOM)
	if smap == nil || len(smap) == 0 {
		return
	}

	info := msg.Data.(hotInfo)

	if len(info.Server) > 0 {
		for _, v := range info.Server {
			ser, ok := smap[v]
			if ok {
				pack := network.NewMsgPack(len(info.Lua))
				pack.WriteString([]byte(info.Lua))
				m._net.SendPackMsg(ser.cid, int(LPMsg.IM_MSG_ID_IM_ROOM_LUA_HOT_LOAD), pack)
			}
		}
	} else {
		check := make(map[int]int)
		for _, v := range info.Except {
			check[v] = 1
		}

		for _, v := range smap {
			_, ok := check[v.id]
			if ok {
				continue
			}

			pack := network.NewMsgPack(len(info.Lua))
			pack.WriteString([]byte(info.Lua))
			m._net.SendPackMsg(v.cid, int(LPMsg.IM_MSG_ID_IM_ROOM_LUA_HOT_LOAD), pack)
		}
	}
}

func (m *MasterModule) onServerLinkInfo(msg *handle.BaseMsg) {
	p := msg.Data.(*network.Msgpack)

	stype := p.ReadInt32()
	sid := p.ReadInt32()

	// log
	// util.Log_info("get link info type:%d serid:%d", stype, sid)

	num := p.ReadInt32()

	info := m.getServerState(int(stype), int(sid))

	for i := 0; i < int(num); i++ {
		lt := p.ReadInt32()
		info.link[int(lt)] = int(p.ReadInt32())
	}
}

func (m *MasterModule) getServerState(stype int, sid int) *serverStateInfo {
	key := (stype << 16) | sid
	info, ok := m._server_state[key]
	if !ok {
		info = &serverStateInfo{
			link: make(map[int]int),
		}
		m._server_state[key] = info
	}
	return info
}

func (m *MasterModule) onGetServerGroupInfo(msg *handle.BaseMsg) {
	p := msg.Data.(*network.Msgpack)
	var svec []dbServer
	m._sql.Select(&svec, "SELECT * FROM `server` WHERE `type`=? OR `type`=?;", util.ST_MYSQL, util.ST_ROOM_MANAGER)

	pack := network.NewMsgPackDef()
	pack.WriteInt32(len(svec))
	for _, v := range svec {
		pack.WriteInt32(v.Type)
		pack.WriteInt32(v.Id)
		pack.WriteInt32(v.Group)
		pack.WriteRealString(v.Mysql)
	}
	m._net.SendPackMsg(p.ConnId(), int(LPMsg.IM_MSG_ID_IM_RMGR_SERVER_GROUP_INFO), pack)
}

func (m *MasterModule) onSendBanInfo(msg *handle.BaseMsg) {
	p := msg.Data.(*network.Msgpack)
	ser := m.getServer(util.ST_LOGIN_LOCK, 0)
	if ser == nil {
		return
	}
	p.SetWriteIndex()
	p.WriteInt32(ser.id)
	m.sendAllServer(util.ST_LOGIN_LOCK, int(LPMsg.IM_MSG_ID_IM_LOCK_BAN_INFO), p)
}

func (m *MasterModule) onNoticeDbNumChange(msg *handle.BaseMsg) {
	m.sendServer(util.ST_ADMIN_MGR, 0, int(LPMsg.IM_MSG_ID_IM_ADM_LOAD_PLAYER_DBNUM), network.NewMsgPackDef())
}

func (m *MasterModule) onAddServerMail(msg *handle.BaseMsg) {
	mail := msg.Data.(*ServerMail)
	pbmail := &LPMsg.DBServerMail{
		Sender:  "管理员",
		Title:   mail.Title,
		Content: mail.Content,
		Reward:  mail.Reward,
		Time:    int32(util.GetSecond()),
	}

	data, err := proto.Marshal(pbmail)
	if err != nil {
		util.Log_error(err.Error())
		return
	}
	pack := network.NewMsgPackDef()
	pack.WriteBuff(data)
	m.sendServer(util.ST_PUBLICK, 0, int(LPMsg.IM_MSG_ID_IM_PUB_ADD_SERVER_MAIL), pack)
}

func (m *MasterModule) onAddPlayerMail(msg *handle.BaseMsg) {
	mail := msg.Data.(*PlayerMail)
	pbmail := &LPMsg.DBChrMail{
		Cid:     int32(mail.Cid),
		Sender:  "管理员",
		Title:   mail.Title,
		Content: mail.Content,
		Reward:  mail.Reward,
		Time:    int32(util.GetSecond()),
	}

	data, err := proto.Marshal(pbmail)
	if err != nil {
		util.Log_error(err.Error())
		return
	}
	pack := network.NewMsgPackDef()
	pack.WriteBuff(data)

	path := newServerPath(
		PathNode{util.ST_ROOM_MANAGER, int16(mail.RoomMgrId)},
		PathNode{util.ST_ROOM, 0})

	m.sendServerPath(path, int(LPMsg.IM_MSG_ID_IM_ROOM_ADD_PLAYER_MAIL), pack)
}
