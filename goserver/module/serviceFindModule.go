package module

import (
	"context"
	"fmt"
	"goserver/handle"
	"goserver/network"
	"goserver/util"
	"os"
	"strconv"
	"strings"

	"github.com/go-redis/redis/v8"
)

type serverInfo struct {
	stype    int32
	sid      int32
	state    int32
	connid   int
	addrkey  string
	host     string
	watchkey []string
}

var ctx = context.Background()

type ServiceFindModule struct {
	ServerModule
	servers      map[int]serverInfo
	_server_conn map[string]int
	_rdb         *redis.Client
	_http_mod    *ServiceFindHttpModule
}

func (m *ServiceFindModule) Init(mgr *moduleMgr) {
	m.ServerModule.Init(mgr)
	m.servers = make(map[int]serverInfo)
	m._server_conn = make(map[string]int)
	m._http_mod = mgr.GetModule(MOD_HTTP).(*ServiceFindHttpModule)

	util.EventMgr.AddEventCall(util.EV_CONN_CLOSE, m, m.onServerClose)

	handle.Handlemsg.AddMsgCall(handle.N_REGIST_SERVICE_FIND, m.onServerRegist)
	handle.Handlemsg.AddMsgCall(handle.N_SF_NOTICE_SERVER, m.onGetNoticeServer)

	m.initRedis()
}

func (m *ServiceFindModule) initRedis() {
	rhost := util.GetConfValue("redis-host")
	rpass := util.GetConfValue("redis-pass")
	rdb := util.GetConfInt("redis-db")

	m._rdb = redis.NewClient(&redis.Options{
		Addr:     rhost,
		Password: rpass,
		DB:       rdb,
	})

	_, err := m._rdb.Ping(ctx).Result()
	if err != nil {
		panic(err)
	}
	util.Log_info("redis connect %s\n", rhost)
}

func (m *ServiceFindModule) AfterInit() {
	m.ServerModule.AfterInit()
	sqlhost := util.GetConfValue("mysql")
	m.initSqlHost(sqlhost)

	var svec []dbServiceFind
	err := m._sql.Select(&svec, "SELECT * FROM `service_find`;")
	if err != nil {
		util.Log_error("get service db info err:%s\n", err.Error())
		os.Exit(1)
	}
	m.closeSql()

	for _, v := range svec {
		if v.Id == util.GetSelfServerId() {
			m._host = fmt.Sprintf(":%d", v.Port)
			m.startListen()
			m._http_mod.SetHost(fmt.Sprintf(":%d", v.Httpport))
		} else {
			addr := fmt.Sprintf("%s:%d", v.Ip, v.Port)
			m.addConnectServer(util.ST_SERVICE_FIND, v.Id, addr)
		}
	}
}

func (m *ServiceFindModule) onServerRegist(msg *handle.BaseMsg) {
	p := msg.Data.(*network.Msgpack)

	sid := p.ReadInt32()
	stype := p.ReadInt32()
	realip := p.ReadString()
	addrkey := p.ReadString()
	serhost := p.ReadString()
	conn := p.GetConn()
	serverip := conn.RemoteAddr().String()
	sarr := strings.Split(serverip, ":")
	host := []byte(strings.Replace(string(serhost), "ip", sarr[0], 1))
	realip = []byte(strings.Replace(string(serhost), "ip", string(realip), 1))

	// if stype == util.ST_MASTER {
	// 	host = realip
	// }

	_, ok := m._server_conn[string(addrkey)]
	if ok {
		util.Log_info("server allready exsist %s\n", addrkey)
		m._net.CloseConn(p.ConnId())
		return
	}

	var connkey, watchkey, noticekey []string
	num := p.ReadInt8()
	connkey = make([]string, 0, num)
	for i := 0; i < int(num); i++ {
		connkey = append(connkey, string(p.ReadString()))
	}
	num = p.ReadInt8()
	watchkey = make([]string, 0, num)
	for i := 0; i < int(num); i++ {
		watchkey = append(watchkey, string(p.ReadString()))
	}
	num = p.ReadInt8()
	noticekey = make([]string, 0, num)
	for i := 0; i < int(num); i++ {
		noticekey = append(noticekey, string(p.ReadString()))
	}

	_, err := m._rdb.Set(ctx, string(addrkey), host, 0).Result()
	if err != nil {
		util.Log_error(err.Error())
	}
	m._server_conn[string(addrkey)] = p.ConnId()

	connser := make([]string, 0, 10)
	for _, v := range connkey {
		adds, err := m._rdb.Keys(ctx, v).Result()
		if err != nil {
			util.Log_error(err.Error())
			continue
		}
		if len(adds) > 0 {
			res, err := m._rdb.MGet(ctx, adds...).Result()
			if err != nil {
				util.Log_error(err.Error())
				continue
			}
			for _, saddr := range res {
				connser = append(connser, saddr.(string))
			}
		}
	}

	if len(connser) > 0 {
		pack := network.NewMsgPackDef()
		pack.WriteInt32(len(connser))
		for _, v := range connser {
			pack.WriteRealString(v)
		}
		m._net.SendPackMsg(p.ConnId(), handle.N_CONN_SERVER_INFO, pack)
	}

	for _, v := range watchkey {
		_, err := m._rdb.SAdd(ctx, v, addrkey).Result()
		if err != nil {
			util.Log_error(err.Error())
		}
	}

	m.noticeServer(string(host), noticekey)
	m.sendNoticeServer(string(host), noticekey)

	if stype == util.ST_LOGIN {
		m._rdb.HSet(ctx, "loginserver", strconv.Itoa(int(sid)), realip)
	}

	info := serverInfo{
		stype:    stype,
		sid:      sid,
		state:    0,
		connid:   p.ConnId(),
		addrkey:  string(addrkey),
		host:     string(host),
		watchkey: watchkey,
	}

	m.servers[info.connid] = info
	util.Log_info("server reg %s %s\n", addrkey, host)
}

func (m *ServiceFindModule) noticeServer(host string, skey []string) {
	noticeser := make([]string, 0, 10)
	for _, v := range skey {
		res, err := m._rdb.SMembers(ctx, v).Result()
		if err != nil {
			util.Log_error(err.Error())
		} else {
			if len(res) > 0 {
				noticeser = append(noticeser, res...)
			}
		}
	}

	broadcid := make([]int, 0, 10)
	for _, v := range noticeser {
		cid, ok := m._server_conn[v]
		if ok {
			broadcid = append(broadcid, cid)
		}
	}

	if len(broadcid) > 0 {
		pack := network.NewMsgPackDef()
		pack.WriteInt32(1)
		pack.WriteRealString(string(host))
		m._net.BroadPackMsg(broadcid, handle.N_CONN_SERVER_INFO, pack)
	}
}

func (m *ServiceFindModule) sendNoticeServer(host string, skey []string) {
	pack := network.NewMsgPackDef()
	pack.WriteRealString(host)
	pack.WriteInt32(len(skey))
	for _, v := range skey {
		pack.WriteRealString(v)
	}
	m.sendAllServer(util.ST_SERVICE_FIND, handle.N_SF_NOTICE_SERVER, pack, util.GetSelfServerId())
}

func (m *ServiceFindModule) onGetNoticeServer(msg *handle.BaseMsg) {
	pack := msg.Data.(*network.Msgpack)

	host := pack.ReadString()
	num := pack.ReadInt32()
	noticekey := make([]string, 0, num)
	for i := 0; i < int(num); i++ {
		noticekey = append(noticekey, string(pack.ReadString()))
	}
	m.noticeServer(string(host), noticekey)
}

func (m *ServiceFindModule) onServerClose(d interface{}) {
	cid := d.(int)

	info, ok := m.servers[cid]

	if ok {
		util.Log_info("server close %s\n", info.addrkey)
		delete(m.servers, cid)
		delete(m._server_conn, info.addrkey)

		m._rdb.Del(ctx, info.addrkey)
		for _, v := range info.watchkey {
			m._rdb.SRem(ctx, v, info.addrkey)
		}

		if info.stype == util.ST_LOGIN {
			m._rdb.HDel(ctx, "loginserver", strconv.Itoa(int(info.sid)))
		}
	}
}
