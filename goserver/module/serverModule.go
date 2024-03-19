package module

import (
	"encoding/json"
	"fmt"
	"goserver/handle"
	"goserver/network"
	"goserver/util"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

const (
	SCT_NONE = 0 + iota
	SCT_ALL
	SCT_AVG
	SCT_ID
)

type netServer struct {
	servertype int
	id         int
	addr       string
	data       interface{}
	cid        int
	reconnect  bool
}

type connRule struct {
	group          bool
	server_type    int
	to_server_type int
	conn_type      int
}

type jsServer struct {
	Name string `json:"name"`
	Type int    `json:"type"`
}

type jsConnRule struct {
	Group          bool   `json:"group"`
	Server_type    string `json:"server"`
	To_server_type string `json:"to_server"`
	Conn_type      int    `json:"type"`
}

type PathNode struct {
	stype int8
	serid int16
}

func newServerPath(ps ...PathNode) []PathNode {
	np := make([]PathNode, 0, 2)
	np = append(np, PathNode{
		stype: util.ST_MASTER,
		serid: 1,
	})
	np = append(np, ps...)
	return np
}

func pathToPack(path []PathNode, mid int, pack *network.Msgpack) *network.Msgpack {
	np := network.NewMsgPackDef()
	np.WriteInt8(int8(len(path)))
	np.WriteInt8(1)

	for _, v := range path {
		np.WriteInt8(v.stype)
		np.WriteInt16(v.serid)
	}
	np.WriteInt32(mid)
	np.WriteBuff(pack.GetBuff())
	return np
}

type ServerModule struct {
	modulebase
	_net           *netModule
	_host          string
	_sql           *sqlx.DB
	_serviceFind   []dbServiceFind
	_find_index    int
	_serverInfo    dbServer
	_conn_rule     []connRule
	_server_name   map[int]string
	_net_server    map[int]map[int]*netServer
	_client_server map[int]*netServer
}

func (m *ServerModule) Init(mgr *moduleMgr) {
	m._mod_mgr = mgr
	m._find_index = 0
	m._server_name = make(map[int]string)
	m._net_server = make(map[int]map[int]*netServer)
	m._client_server = make(map[int]*netServer)

	util.EventMgr.AddEventCall(util.EV_SERVER_CLOSE, m, m.onServerClose)
	util.EventMgr.AddEventCall(util.EV_CONN_CLOSE, m, m.onClientClose)

	handle.Handlemsg.AddMsgCall(handle.M_SERVER_CONNECTED, m.onServerConnect)
	handle.Handlemsg.AddMsgCall(handle.N_CONN_SERVER_INFO, m.onGetConnInfo)
	handle.Handlemsg.AddMsgCall(handle.N_REGISTE_SERVER, m.onRegistServer)
	handle.Handlemsg.AddMsgCall(handle.N_SERVER_PING, m.onServerPing)
}

func (m *ServerModule) AfterInit() {
	m._net = ModuleMgr.GetModule(MOD_NET).(*netModule)
	if len(m._host) > 0 {
		m.startListen()
	}
	m.parseConnRule()
}

func (m *ServerModule) startListen() {
	go func() {
		host := m._host

		ln, err := net.Listen("tcp", host)

		if err != nil {
			util.Log_error(err.Error())
			return
		} else {
			util.Log_info("start listen:%s", host)
		}

		for {
			conn, err := ln.Accept()
			if err != nil {
				util.Log_error(err.Error())
			} else {
				util.Log_info("accept conn %s", conn.RemoteAddr().String())
				m._net.AcceptConn(conn, m.onReadConnBuff, false)
			}
		}
	}()
}

func getRealIp(url string) string {
	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		util.Log_error("http get err:%s\n", err.Error())
		return ""
	}
	defer resp.Body.Close()

	str, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		util.Log_error("http get err:%s\n", err.Error())
		return ""
	}
	return string(str)
}

func (m *ServerModule) getRealIpFromHttp() string {
	url := util.GetConfValue("realipservice")
	strvec := strings.Split(url, "|")

	for _, v := range strvec {
		rip := getRealIp(v)
		if len(rip) > 0 {
			return rip
		}
	}
	return ""
}

func (m *ServerModule) getConfigFromHttp(stype int, sid int) bool {
	url := util.GetConfValue("confighost") + "/serverInfo?id=%d&type=%d"
	url = fmt.Sprintf(url, sid, stype)

	// req,err := http.NewRequest("GET",url,nil)
	// if err != nil {
	// 	util.Log_error("http req err %s \n",err.Error())
	// 	return false
	// }

	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		util.Log_error("http get err:%s\n", err.Error())
		return false
	}
	defer resp.Body.Close()

	var serinfo ServiceInfo
	err = json.NewDecoder(resp.Body).Decode(&serinfo)
	if err != nil {
		util.Log_error("json decode err:%s\n", err.Error())
		return false
	}

	m._serverInfo = serinfo.Server
	m._serverInfo.Name = m._server_name[serinfo.Server.Type]

	if serinfo.Server.Port > 0 {
		m._host = fmt.Sprintf(":%d", serinfo.Server.Port)
		m.startListen()
	}

	if serinfo.Server.MysqlHost.Id > 0 {
		d := serinfo.Server.MysqlHost
		m.initSql(d.Ip, d.Port, d.User, d.Pass, d.Dbname)
	}

	if len(serinfo.Service) > 0 {
		m._serviceFind = append(m._serviceFind, serinfo.Service...)
		rand.Seed(util.GetMillisecond())
		m._find_index = rand.Int() % len(m._serviceFind)
		m.connnetFindService()
	}
	return true
}

func (m *ServerModule) initSql(ip string, port int, user string, pass string, db string) {
	host := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8", user, pass, ip, port, db)
	m.initSqlHost(host)
}

func (m *ServerModule) initSqlHost(host string) {
	sql, sqlerr := sqlx.Open("mysql", host)
	m._sql = sql
	if sqlerr != nil {
		panic("sql open error:" + sqlerr.Error())
	}
	sql.SetMaxOpenConns(5)
	// 闲置连接数
	sql.SetMaxIdleConns(3)
	// 最大连接周期
	sql.SetConnMaxLifetime(100 * time.Second)
}

func (m *ServerModule) closeSql() {
	if m._sql != nil {
		m._sql.Close()
		m._sql = nil
	}
}

func (m *ServerModule) connnetFindService() {
	if m._find_index >= len(m._serviceFind) {
		m._find_index = 0
	}

	sfind := m._serviceFind[m._find_index]
	addr := fmt.Sprintf("%s:%d", sfind.Ip, sfind.Port)
	m.connectServer(addr, util.ST_SERVICE_FIND, sfind.Id, sfind, false)
}

func (m *ServerModule) connectServer(addr string, stype int, sid int, udata interface{}, reconnect bool) {
	netser := &netServer{
		addr:       addr,
		servertype: stype,
		id:         sid,
		data:       udata,
		reconnect:  reconnect,
	}
	m._net.ConnectServer(addr, &m.modulebase, m.onReadConnBuff, netser)
}

func (m *ServerModule) connectNetServer(ser *netServer) {
	m._net.ConnectServer(ser.addr, &m.modulebase, m.onReadConnBuff, ser)
}

func (m *ServerModule) onServerConnect(msg *handle.BaseMsg) {
	sinfo := msg.Data.(serverConnInfo)
	ser := sinfo.userData.(*netServer)
	if sinfo.cid == network.CONN_STATE_DISCONNECT {
		util.Log_info("conn server error type:%d id:%d addr:%s\n", ser.servertype, ser.id, ser.addr)
		if !ser.reconnect && ser.servertype == util.ST_SERVICE_FIND {
			// 重连 10s
			m.setAfterFunc(time.Second*10, func(i int64) {
				m.connnetFindService()
			})
		} else {
			// 重连 10s
			m.setAfterFunc(time.Second*10, func(i int64) {
				newser := m.getServer(ser.servertype, ser.id)
				if newser != nil {
					m.connectNetServer(newser)
				}
			})
		}
	} else {
		util.Log_info("conn server success type:%d id:%d addr:%s\n", ser.servertype, ser.id, ser.addr)
		ser.cid = sinfo.cid
		if ser.servertype == util.ST_SERVICE_FIND {
			if !ser.reconnect {
				m.addNetServer(ser)
				m.registServiceFind()
			}
		} else {
			// 发注册消息
			if ser.reconnect {
				pack := network.NewMsgPackDef()
				mysid, mystype := util.GetSelfServer()
				pack.WriteInt32(mysid)
				pack.WriteInt32(mystype)
				pack.WriteRealString("")
				pack.WriteInt32(m._serverInfo.Port)
				m._net.SendPackMsg(ser.cid, handle.N_REGISTE_SERVER, pack)
			}
		}
	}
}

func (m *ServerModule) onServerClose(d interface{}) {
	sinfo := d.(serverConnInfo)
	ser := sinfo.userData.(*netServer)
	util.Log_info("service conn disconnect type:%d serid:%d addr:%s\n", ser.servertype, ser.id, ser.addr)
	newser := m.getServer(ser.servertype, ser.id)
	if newser == ser {
		if ser.reconnect {
			m.setAfterFunc(time.Second*10, func(i int64) {
				newser := m.getServer(ser.servertype, ser.id)
				if newser != nil {
					m.connectNetServer(newser)
				}
			})
		} else {
			m.removeNetServer(ser.servertype, ser.id)
			if ser.servertype == util.ST_SERVICE_FIND {
				// 重连 10s
				m.setAfterFunc(time.Second*10, func(i int64) {
					m.connnetFindService()
				})
			}
		}
	}
}

func (m *ServerModule) registServiceFind() {
	pack := network.NewMsgPackDef()
	pack.WriteInt32(m._serverInfo.Id)
	pack.WriteInt32(m._serverInfo.Type)
	pack.WriteRealString(m.getRealIpFromHttp())
	pack.WriteRealString(m.getServerAddrKey())
	pack.WriteRealString(m.getServerAddrInfo())

	ck, wk := m.getConnectKey()
	nk := m.getNoticeKey()

	pack.WriteInt8(int8(len(ck)))
	for _, v := range ck {
		pack.WriteRealString(v)
	}

	pack.WriteInt8(int8(len(wk)))
	for _, v := range wk {
		pack.WriteRealString(v)
	}

	pack.WriteInt8(int8(len(nk)))
	for v := range nk {
		pack.WriteRealString(v)
	}
	m.sendAllServer(util.ST_SERVICE_FIND, handle.N_REGIST_SERVICE_FIND, pack)
}

func (m *ServerModule) parseConnRule() {
	serverfile := util.GetConfValue("jsonPath") + "Server.json"
	d1, err := ioutil.ReadFile(serverfile)
	if err != nil {
		util.Log_error("read json:%s err:%s\n", serverfile, err.Error())
		return
	}
	var serinfo []jsServer
	err = json.Unmarshal(d1, &serinfo)
	if err != nil {
		util.Log_error("json err:%s\n", err.Error())
		return
	}

	rulefile := util.GetConfValue("jsonPath") + "connect_rule.json"
	d1, err = ioutil.ReadFile(rulefile)
	if err != nil {
		util.Log_error("read json:%s err:%s\n", serverfile, err.Error())
		return
	}

	var ruleinfo []jsConnRule
	err = json.Unmarshal(d1, &ruleinfo)
	if err != nil {
		util.Log_error("json err:%s\n", err.Error())
		return
	}

	snameType := make(map[string]int, len(serinfo))
	for _, v := range serinfo {
		snameType[v.Name] = v.Type
		m._server_name[v.Type] = v.Name
	}

	for _, v := range ruleinfo {
		if len(v.Server_type) == 0 {
			continue
		}
		st, _ := snameType[v.Server_type]
		tost, _ := snameType[v.To_server_type]
		m._conn_rule = append(m._conn_rule, connRule{
			group:          v.Group,
			server_type:    st,
			to_server_type: tost,
			conn_type:      v.Conn_type,
		})
	}
}

func (m *ServerModule) getServerAddrKey() string {
	s := m._serverInfo
	return fmt.Sprintf("serveraddr:g%d:%s-%d", s.Group, s.Name, s.Id)
}

func (m *ServerModule) getServerAddrInfo() string {
	s := m._serverInfo
	return fmt.Sprintf("%s:%d:%d:ip:%d", s.Name, s.Type, s.Id, s.Port)
}

func (m *ServerModule) getConnectKey() ([]string, []string) {
	connkey := make([]string, 0)
	watchkey := make([]string, 0)

	for _, v := range m._conn_rule {
		if v.server_type != m._serverInfo.Type {
			continue
		}

		ckey := ":g"
		if v.group {
			ckey += strconv.Itoa(m._serverInfo.Group)
		} else {
			ckey += "*"
		}

		ckey += ":" + m._server_name[v.to_server_type]

		if v.conn_type == SCT_ALL {
			ckey += "-*"
		} else if v.conn_type == SCT_ID {
			ckey += strconv.Itoa(m._serverInfo.Id)
		}

		connkey = append(connkey, "serveraddr"+ckey)
		watchkey = append(watchkey, "watch"+ckey)
	}
	return connkey, watchkey
}

func (m *ServerModule) getNoticeKey() map[string]bool {
	res := make(map[string]bool)

	for _, v := range m._conn_rule {
		if v.to_server_type != m._serverInfo.Type {
			continue
		}

		k := "watch:g"
		if v.group {
			k += strconv.Itoa(m._serverInfo.Group)
		} else {
			k += "*"
		}

		k += ":" + m._serverInfo.Name

		if v.conn_type == SCT_ALL {
			k += "-*"
		} else if v.conn_type == SCT_ID {
			k += strconv.Itoa(m._serverInfo.Id)
		}

		res[k] = true
	}
	return res
}

func (m *ServerModule) onGetConnInfo(msg *handle.BaseMsg) {
	pack := msg.Data.(*network.Msgpack)
	num := pack.ReadInt32()

	var server []*netServer

	for i := 0; i < int(num); i++ {
		str := string(pack.ReadString())
		sarr := strings.Split(str, ":")
		if len(sarr) != 5 {
			util.Log_error("conn info error %s\n", str)
			continue
		}

		stype, _ := strconv.Atoi(sarr[1])
		sid, _ := strconv.Atoi(sarr[2])
		addr := fmt.Sprintf("%s:%s", sarr[3], sarr[4])

		old := m.getServer(stype, sid)
		if old != nil && old.addr == addr {
			continue
		}

		server = append(server, &netServer{
			servertype: stype,
			id:         sid,
			addr:       addr,
			cid:        network.CONN_STATE_DISCONNECT,
			reconnect:  true,
		})
	}

	for _, v := range server {
		m.addNetServer(v)
		m.connectNetServer(v)
	}
}

func (m *ServerModule) addConnectServer(stype int, sid int, addr string) {
	ser := &netServer{
		servertype: stype,
		id:         sid,
		addr:       addr,
		cid:        network.CONN_STATE_DISCONNECT,
		reconnect:  true,
	}
	m.addNetServer(ser)
	m.connectNetServer(ser)
}

func getRandServer(m map[int]*netServer) *netServer {
	r := rand.Intn(len(m))
	for _, v := range m {
		if r == 0 {
			return v
		}
		r--
	}
	return nil
}

func (m *ServerModule) getServer(stype int, sid int) *netServer {
	smap, ok := m._net_server[stype]
	if ok {
		var ser *netServer
		if sid == 0 {
			ser = getRandServer(smap)
			return ser
		} else {
			ser, ok = smap[sid]
			if ok {
				return ser
			}
		}
	}
	return nil
}

func (m *ServerModule) addNetServer(ser *netServer) {
	smap, ok := m._net_server[ser.servertype]
	if !ok {
		smap = make(map[int]*netServer)
		m._net_server[ser.servertype] = smap
	}
	smap[ser.id] = ser
}

func (m *ServerModule) removeNetServer(stype int, sid int) {
	smap, ok := m._net_server[stype]
	if ok {
		delete(smap, sid)
	}
}

func (m *ServerModule) sendServer(stype int, sid int, mid int, pack *network.Msgpack) {
	ser := m.getServer(stype, sid)
	if ser == nil || ser.cid == network.CONN_STATE_DISCONNECT {
		return
	}
	m._net.SendPackMsg(ser.cid, mid, pack)
}

func (m *ServerModule) sendServerPath(path []PathNode, mid int, pack *network.Msgpack) {
	if len(path) < 2 {
		return
	}

	pack = pathToPack(path, mid, pack)
	nd := path[1]
	if nd.serid == -1 {
		m.sendAllServer(int(nd.stype), handle.N_TRANS_SERVER_MSG, pack)
	} else {
		m.sendServer(int(nd.stype), int(nd.serid), handle.N_TRANS_SERVER_MSG, pack)
	}
}

func (m *ServerModule) sendAllServer(stype int, mid int, pack *network.Msgpack, nosid ...int) {
	smap, ok := m._net_server[stype]
	if !ok {
		return
	}

	var broadid []int
	for _, v := range smap {
		send := true
		for _, sid := range nosid {
			if sid == v.id {
				send = false
			}
		}
		if send && v.cid != network.CONN_STATE_DISCONNECT {
			broadid = append(broadid, v.cid)
		}
	}
	if len(broadid) > 0 {
		m._net.BroadPackMsg(broadid, mid, pack)
	}
}

func (m *ServerModule) onRegistServer(msg *handle.BaseMsg) {
	p := msg.Data.(*network.Msgpack)

	sid := p.ReadInt32()
	stype := p.ReadInt32()
	ip := p.ReadString()
	port := p.ReadInt32()
	addr := fmt.Sprintf("%s:%d", ip, port)

	addr = p.GetConn().RemoteAddr().String()

	ser := &netServer{
		servertype: int(stype),
		id:         int(sid),
		addr:       addr,
		cid:        p.ConnId(),
		reconnect:  false,
	}

	util.Log_info("regist server type:%d id:%d addr:%s \n", stype, sid, addr)
	m.addNetServer(ser)
	m._client_server[p.ConnId()] = ser
}

func (m *ServerModule) onClientClose(d interface{}) {
	cid := d.(int)
	ser, ok := m._client_server[cid]
	if ok {
		util.Log_info("client server close addr:%s type:%d id:%d\n", ser.addr, ser.servertype, ser.id)
		if ser == m.getServer(ser.servertype, ser.id) {
			m.removeNetServer(ser.servertype, ser.id)
		}
		delete(m._client_server, cid)
	}
}

func (m *ServerModule) onServerPing(msg *handle.BaseMsg) {
	p := msg.Data.(*network.Msgpack)
	pack := network.NewMsgPackDef()
	m._net.SendPackMsg(p.ConnId(), handle.N_SERVER_PONG, pack)
}

func (m *ServerModule) getServerList(stype int) map[int]*netServer {
	smap, ok := m._net_server[stype]
	if !ok {
		return nil
	}
	return smap
}
