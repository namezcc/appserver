package module

import (
	"context"
	"encoding/json"
	"goserver/handle"
	"goserver/network"
	"goserver/util"
	"net"
	"net/http"
	"time"

	"github.com/gobwas/ws"
	"github.com/qiniu/qmgo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"gorm.io/gorm"
)

const PING_OUT_TIME = 300     //30
const MAX_TASK_TALK_NUM = 15  //保留500条聊天,>=2倍则删除一半
const MAX_USER_CHAT_NUM = 100 //保留500条聊天,>=2倍则删除一半

const (
	CHAT_TYPE_BEGIN = iota
	CHAT_TYPE_TEXT
	CHAT_TYPE_IMAGE
	CHAT_TYPE_TASK
	CHAT_TYPE_END
)

type MH map[string]interface{}

type appUserInfo struct {
	cid      int64
	Phone    string
	connid   int
	pingTime int64
	listnode *util.ListOrderNode
	user     b_user
}

type userMsg struct {
	user *appUserInfo
	msg  *network.Msgpack
}

type ClientServerModule struct {
	modulebase
	_net                  *netModule
	_host                 string
	_host_ws              string
	_user                 map[int]*appUserInfo
	_user_cid             map[int64]*appUserInfo
	_user_ping_order_list *util.ListOrder
	_task_chat            map[string]*mg_task_chat
	_task_join            map[string]*mg_task_join
	_mg_mgr               *MongoManagerModule
	_sql_mgr              *MysqlManagerModule
}

func sortUserPing(a, b interface{}) bool {
	ua := a.(*appUserInfo)
	ub := b.(*appUserInfo)
	return ua.pingTime > ub.pingTime
}

func (m *ClientServerModule) Init(mgr *moduleMgr) {
	m._mod_mgr = mgr
	m._host = util.GetConfValue("serverhost")
	m._host_ws = util.GetConfValue("serverhostws")
	m._user = make(map[int]*appUserInfo)
	m._user_cid = make(map[int64]*appUserInfo)
	m._user_ping_order_list = util.NewListOrder(sortUserPing)
	m._task_chat = make(map[string]*mg_task_chat)
	m._task_join = make(map[string]*mg_task_join)
	m._mg_mgr = mgr.GetModule(MOD_MONGO_MGR).(*MongoManagerModule)
	m._sql_mgr = mgr.GetModule(MOD_MYSQL_MGR).(*MysqlManagerModule)

	util.EventMgr.AddEventCall(util.EV_CONN_CLOSE, m, m.onClientClose)

	handle.Handlemsg.AddMsgCall(handle.M_ON_CLINET_MSG, m.onDispatchClientMsg)
	handle.Handlemsg.AddMsgCall(handle.M_ON_TASK_JOIN_UPDATE, m.onUpdateTaskJoin)
	handle.Handlemsg.AddMsgCall(handle.M_ON_TASK_NO_CHAT, m.onTaskNoChat)
	handle.Handlemsg.AddMsgCall(handle.M_ON_TASK_DELETE, m.onTaskDelete)
	handle.Handlemsg.AddMsgCall(handle.M_ON_TASK_KICK, m.onTaskBeKick)

	handle.Handlemsg.AddMsgCall(handle.N_CM_LOGIN, m.onClientLogin)
	handle.Handlemsg.AddMsgCall(handle.N_CM_PING, m.onClientPing)
	handle.Handlemsg.AddMsgCall(handle.N_CM_TASK_CHAT, m.onClientTaskChat)
	handle.Handlemsg.AddMsgCall(handle.N_CM_LOAD_TASK_CHAT, m.onGetOneTaskChat)
	handle.Handlemsg.AddMsgCall(handle.N_CM_TASK_CHAT_READ, m.onTaskChatRead)

	handle.Handlemsg.AddMsgCall(handle.N_CM_CHAT_USER, m.onClientChatUser)
	handle.Handlemsg.AddMsgCall(handle.N_CM_CHAT_USER_GET, m.onClientChatUserGet)
	handle.Handlemsg.AddMsgCall(handle.N_CM_CHAT_USER_READ, m.onClientChatUserRead)
}

func (m *ClientServerModule) AfterInit() {
	m._net = ModuleMgr.GetModule(MOD_NET).(*netModule)
	if len(m._host) > 0 {
		m.startListen()
	}

	if len(m._host_ws) > 0 {
		m.startListenWs()
	}

	m.setTikerFunc(time.Second*5, m.onCheckPingTime)
}

func (m *ClientServerModule) addUser(user *appUserInfo) {
	m._user[user.connid] = user
	m._user_cid[user.cid] = user
	user.listnode = m._user_ping_order_list.PushBack(user)
}

func (m *ClientServerModule) getUserByConnid(connid int) *appUserInfo {
	u, ok := m._user[connid]
	if ok {
		return u
	}
	return nil
}

func (m *ClientServerModule) getUserByCid(cid int64) *appUserInfo {
	u, ok := m._user_cid[cid]
	if ok {
		return u
	}
	return nil
}

func (m *ClientServerModule) removeUserByConnid(connid int) {
	u := m.getUserByConnid(connid)
	if u != nil {
		delete(m._user, u.connid)
		delete(m._user_cid, u.cid)
		if u.listnode != nil {
			u.listnode.Remove()
			u.listnode = nil
		}
	}
}

func (m *ClientServerModule) onClientClose(d interface{}) {
	cid := d.(int)
	util.Log_info("client close conn:%d", cid)
	m.removeUserByConnid(cid)
}

func (m *ClientServerModule) startListen() {
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
				m._net.AcceptConn(conn, m.onReadClientMsg, false)
			}
		}
	}()
}

func (m *ClientServerModule) startListenWs() {
	go func() {
		util.Log_info("start listen ws:%s", m._host_ws)
		err := http.ListenAndServe(m._host_ws, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, _, _, err := ws.UpgradeHTTP(r, w)
			if err != nil {
				util.Log_error(err.Error())
				return
			} else {
				util.Log_info("accept websocket conn %s", conn.RemoteAddr().String())
			}
			// conn.Close()
			m._net.AcceptConn(conn, m.onReadClientMsg, true)
		}))
		if err != nil {
			util.Log_error(err.Error())
		}
	}()
}

func (m *ClientServerModule) onReadClientMsg(cid int, mid int, buf []byte, conn *net.Conn) {
	pack := &network.Msgpack{}
	pack.Init(cid, mid, buf)
	pack.SetConn(conn)
	m.sendMsg(handle.M_ON_CLINET_MSG, pack)
}

func (m *ClientServerModule) onDispatchClientMsg(msg *handle.BaseMsg) {
	p := msg.Data.(*network.Msgpack)
	var u *appUserInfo
	if p.MsgId() == handle.N_CM_LOGIN {
		u = m.getUserByConnid(p.ConnId())
		if u != nil {
			m._net.CloseConnCallEvent(p.ConnId())
			return
		}
	} else {
		u = m.getUserByConnid(p.ConnId())
		if u == nil {
			m._net.CloseConnCallEvent(p.ConnId())
			return
		}
	}
	nmsg := handle.BaseMsg{
		Mid: p.MsgId(),
		Data: &userMsg{
			user: u,
			msg:  p,
		},
	}
	handle.Handlemsg.CallHandle(&nmsg)
}

func (m *ClientServerModule) onClientLogin(msg *handle.BaseMsg) {
	um := msg.Data.(*userMsg)
	p := um.msg
	token := p.ReadBuff()
	util.Log_info("get token %s", string(token))
	claim, err := TokenVerifyAction(string(token))
	if err != nil {
		util.Log_error("verify err:%s", err.Error())
		m._net.CloseConnCallEvent(p.ConnId())
		return
	}
	// 从数据库获取user
	var buser b_user
	m._sql_mgr.RequestFuncCallNoRes(func(d *gorm.DB) {
		res := d.Find(&buser, "cid = ?", claim.UserID)
		if res.Error != nil {
			util.Log_error("getuserinfo cid:%d %s", claim.UserID, res.Error.Error())
		}
	})
	if buser.Cid == 0 {
		m._net.CloseConnCallEvent(p.ConnId())
		return
	}

	user := appUserInfo{
		cid:      claim.UserID,
		Phone:    claim.Phone,
		connid:   p.ConnId(),
		pingTime: util.GetSecond() + PING_OUT_TIME,
		user:     buser,
	}
	m.addUser(&user)
	m.sendUserTaskChatRead(&user)
	m.sendUserTaskChatIndex(&user)
	m.sendUserChatRead(&user)
	m.loadUserChat(&user)
	util.Log_info("app user login connid:%d cid:%d phone:%s", user.connid, user.cid, user.Phone)
}

func (m *ClientServerModule) onClientPing(msg *handle.BaseMsg) {
	um := msg.Data.(*userMsg)
	p := um.msg
	u := um.user
	u.pingTime = util.GetSecond() + PING_OUT_TIME

	// 重新排序
	m._user_ping_order_list.ResetBackOrder(u.listnode)

	pack := network.NewMsgPackDef()
	m._net.SendPackMsg(p.ConnId(), handle.SM_PONG, pack)
}

func (m *ClientServerModule) onCheckPingTime(t int64) {
	if m._user_ping_order_list.Size() == 0 {
		return
	}
	head := m._user_ping_order_list.GetFirst().(*appUserInfo)
	if t >= head.pingTime*1000 {
		// util.Log_info("client ping out time")
		m._net.CloseConnCallEvent(head.connid)
	}
}

func (m *ClientServerModule) sendClientPack(connid int, msgid int, p *network.Msgpack) {
	m._net.SendPackMsg(connid, msgid, p)
}
func (m *ClientServerModule) sendClientMsgError(connid int, msgid int, code int, msg string) {
	d, err := json.Marshal(MH{"code": code, "msg": msg, "msgid": msgid})
	if err != nil {
		util.Log_error("sendclientmsgError: %s", err.Error())
		return
	}
	pack := network.NewMsgPackDef()
	pack.WriteBuff(d)
	m.sendClientPack(connid, handle.SM_ERROR_CODE, pack)
}

func (m *ClientServerModule) sendClientJson(connid int, msgid int, d interface{}) {
	if d == nil {
		util.Log_error("sendClientJson nil msgid:%d", msgid)
		return
	}
	buff, err := json.Marshal(d)
	if err != nil {
		util.Log_error("sendClientJson: %s", err.Error())
		return
	}
	pack := network.NewMsgPackDef()
	pack.WriteBuff(buff)
	m.sendClientPack(connid, msgid, pack)
}

func (m *ClientServerModule) broadClientJson(connid []int, msgid int, d interface{}) {
	buff, err := json.Marshal(d)
	if err != nil {
		util.Log_error("sendClientJson: %s", err.Error())
		return
	}
	pack := network.NewMsgPackDef()
	pack.WriteBuff(buff)
	m._net.BroadPackMsg(connid, msgid, pack)
}

type Mg_chat struct {
	Cid         int64  `json:"cid,omitempty" bson:"cid"`
	Sendername  string `json:"sendername,omitempty" bson:"sendername"`
	Sendericon  string `json:"sendericon,omitempty" bson:"sendericon"`
	SendTime    int64  `json:"send_time,omitempty" bson:"send_time"`
	Content     string `json:"content" bson:"content"`
	ContentType int    `json:"content_type" bson:"content_type"`
	Index       int    `json:"index" bson:"index"`
}

type mg_task_chat struct {
	Id     primitive.ObjectID `json:"id" bson:"_id"`
	Cid    int64              `json:"cid" bson:"cid"`
	Data   []Mg_chat          `json:"data,omitempty" bson:"data,omitempty"`
	Count  int                `json:"count" bson:"count"`
	Index  int                `json:"index" bson:"index"`
	Delete int                `json:"delete" bson:"delete"`
}

type mg_chat_user struct {
	Mg_chat `json:",inline" bson:",inline"`
	Tocid   int64 `json:"tocid" bson:"tocid"`
	Chatid  int64 `json:"chatid,omitempty" bson:"chatid"`
}

type mg_chat_user_list struct {
	Id       primitive.ObjectID `json:"id" bson:"_id"`
	Cidhei   int64              `json:"cidhei" bson:"cidhei"`
	Cidlow   int64              `json:"cidlow" bson:"cidlow"`
	Data     []Mg_chat          `json:"data,omitempty" bson:"data,omitempty"`
	Count    int                `json:"count" bson:"count"`
	Indexid  int                `json:"indexid" bson:"indexid"`
	UpdateAt int64              `json:"updateat" bson:"updateat"`
}

func (m *ClientServerModule) getTaskJoin(taskid primitive.ObjectID, cid int64, mastercid int64) *mg_task_join {
	c, ok := m._task_join[taskid.Hex()]
	if !ok {
		// 数据库查找
		var task_join mg_task_join
		m._mg_mgr.RequestFuncCallNoRes(func(ctx context.Context, qc *qmgo.QmgoClient) {
			coll := qc.Database.Collection(COLL_TASK_JOIN)
			err := coll.Find(ctx, bson.M{"_id": taskid}).One(&task_join)
			if err != nil {
				util.Log_error("get task join err:%s", err.Error())
				return
			}
		})
		if task_join.Cid == 0 && mastercid != cid {
			return nil
		}
		task_join.Cid = cid
		c = &task_join
		m._task_join[taskid.Hex()] = c
	}
	return c
}

func (m *ClientServerModule) onUpdateTaskJoin(msg *handle.BaseMsg) {
	j := msg.Data.(*mg_task_join)
	m._task_join[j.Id.Hex()] = j
}

func (m *ClientServerModule) onTaskNoChat(msg *handle.BaseMsg) {
	j := msg.Data.(*mg_task_join)
	join, ok := m._task_join[j.Id.Hex()]
	if ok {
		for i := 0; i < len(join.Data); i++ {
			v := &join.Data[i]
			if v.Cid == join.Cid {
				v.NoChat = 1
			}
		}
	}
}

func (m *ClientServerModule) onTaskDelete(msg *handle.BaseMsg) {
	id := msg.Data.(string)
	c, ok := m._task_chat[id]
	if ok {
		c.Delete = 1
	}
}

func (m *ClientServerModule) onTaskBeKick(msg *handle.BaseMsg) {
	j := msg.Data.(*mg_task_join)
	um := m.getUserByCid(j.Cid)
	if um != nil {
		m.sendClientJson(um.connid, handle.SM_TASK_BE_KICK, j)
	}
}

func (m *ClientServerModule) getTaskChat(taskid primitive.ObjectID) *mg_task_chat {
	c, ok := m._task_chat[taskid.Hex()]
	if !ok {
		// 数据库查询
		m._mg_mgr.RequestFuncCallNoRes(func(ctx context.Context, qc *qmgo.QmgoClient) {
			// 先查看有没有这个任务
			var task mg_task
			taskcoll := qc.Database.Collection(COLL_TASK)
			err := taskcoll.Find(ctx, bson.M{"_id": taskid}).Select(bson.M{"cid": 1}).One(&task)
			if err != nil {
				util.Log_error("get task chat 0 err:%s", taskid.Hex())
				return
			}
			if task.Delete > 0 {
				// 任务以删除
				util.Log_error("get task chat delete:%s", taskid.Hex())
				return
			}
			chatcoll := qc.Database.Collection(COLL_TASK_CHAT)
			var taskchat mg_task_chat
			err = chatcoll.Find(ctx, bson.M{"_id": taskid}).Select(bson.M{"index": 1, "count": 1, "cid": 1}).One(&taskchat)
			if err != nil {
				// 没有数据则插入
				c = &mg_task_chat{
					Id:    taskid,
					Cid:   task.Cid,
					Count: 0,
					Index: 0,
				}
				_, err = chatcoll.InsertOne(ctx, *c)
				if err != nil {
					util.Log_error("insert task chat err:%s", err.Error())
					c = nil
				}
			} else {
				c = &taskchat
			}
		})
		if c != nil {
			m._task_chat[taskid.Hex()] = c
		}
	}
	return c
}

func (m *ClientServerModule) insertTaskChat(c *Mg_chat, tc *mg_task_chat) bool {
	// 更新
	tc.Count++
	tc.Index++
	res := m._mg_mgr.RequestFuncCall(func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		coll := qc.Database.Collection(COLL_TASK_CHAT)
		err := coll.UpdateOne(ctx, bson.M{"_id": tc.Id}, bson.M{"$push": bson.M{"data": *c}, "$set": bson.M{"index": tc.Index, "count": tc.Count}})
		if err != nil {
			util.Log_error("insertTaskChat push:%s", err.Error())
			return false
		}

		if tc.Count >= MAX_TASK_TALK_NUM*2 {
			// 删除多余的
			var tchat mg_task_chat
			pip := getTaskChatPipline(tc.Id, -MAX_TASK_TALK_NUM, MAX_TASK_TALK_NUM)
			err = coll.Aggregate(ctx, pip).One(&tchat)
			if err == nil {
				// 在写回
				tchat.Count = len(tchat.Data)
				err = coll.UpdateOne(ctx, bson.M{"_id": tc.Id}, bson.M{"$set": bson.M{"data": tchat.Data, "count": tchat.Count}})
				if err == nil {
					tc.Count = tchat.Count
				}
			}
		}
		return true
	}).(bool)
	if res == false {
		tc.Count--
		tc.Index--
	}
	return res
}

func (m *ClientServerModule) onClientTaskChat(msg *handle.BaseMsg) {
	um := msg.Data.(*userMsg)
	// json
	var user_chat mg_task_chat
	err := json.Unmarshal(um.msg.ReadBuff(), &user_chat)
	if err != nil || len(user_chat.Data) != 1 {
		return
	}

	chat := user_chat.Data[0]
	if chat.ContentType <= CHAT_TYPE_BEGIN || chat.ContentType >= CHAT_TYPE_END || len(chat.Content) > 1000 {
		return
	}

	taskchat := m.getTaskChat(user_chat.Id)
	if taskchat == nil {
		m.sendClientMsgError(um.msg.ConnId(), handle.N_CM_TASK_CHAT, util.ERRCODE_ERROR, "taskchat nil")
		return
	}

	if taskchat.Delete > 0 {
		m.sendClientMsgError(um.msg.ConnId(), handle.N_CM_TASK_CHAT, util.ERRCODE_TASK_DELETE, "task delete")
		return
	}

	user := um.user
	// 广播
	taskjoin := m.getTaskJoin(taskchat.Id, user.cid, taskchat.Cid)
	if taskjoin == nil {
		return
	}

	chat.Cid = user.cid
	chat.SendTime = util.GetSecond()
	chat.Sendername = user.user.Name
	chat.Sendericon = user.user.Icon
	chat.Index = taskchat.Index
	taskchat.Data = []Mg_chat{chat}
	res := m.insertTaskChat(&chat, taskchat)
	if res == false {
		m.sendClientMsgError(um.msg.ConnId(), handle.N_CM_TASK_CHAT, util.ERRCODE_ERROR, "insert taskchat fail")
		return
	}

	connids := make([]int, 0)
	havemaster := false
	for _, v := range taskjoin.Data {
		if v.Cid == taskchat.Cid {
			havemaster = true
		}
		// if v.NoChat <= 0 {
		// }
		tu := m.getUserByCid(v.Cid)
		if tu != nil {
			connids = append(connids, tu.connid)
		}
	}
	if !havemaster {
		tu := m.getUserByCid(taskchat.Cid)
		if tu != nil {
			connids = append(connids, tu.connid)
		}
	}
	if len(connids) > 0 {
		// 发包
		m.broadClientJson(connids, handle.SM_CHAT_UPDATE, taskchat)
	}
}

func getTaskChatPipline(taskid primitive.ObjectID, istart int, num int) primitive.A {
	pipline := bson.A{
		bson.M{"$match": bson.M{"_id": taskid}},
		bson.M{"$project": bson.M{"index": 1, "count": 1, "data": bson.M{"$slice": bson.A{
			"$data",
			istart,
			num,
		},
		},
		}}}
	return pipline
}

// 单任务聊天数据请求
func (m *ClientServerModule) onGetOneTaskChat(msg *handle.BaseMsg) {
	um := msg.Data.(*userMsg)
	taskstrid := um.msg.ReadString()
	startindex := int(um.msg.ReadInt32())
	num := int(um.msg.ReadInt32())
	// 一次20条
	if num <= 0 || num > 20 {
		return
	}

	taskid := stringToObjectId(string(taskstrid))
	if taskid == nil {
		return
	}

	chatinfo := mg_task_chat{
		Id:    *taskid,
		Count: -1,
	}

	chat := m.getTaskChat(*taskid)
	if chat == nil || startindex >= chat.Count || (startindex < 0 && startindex+num <= -chat.Count) {
		m.sendClientJson(um.user.connid, handle.SM_CHAT_UPDATE, chatinfo)
		return
	}

	err := m._mg_mgr.RequestFuncCall(func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		coll := qc.Database.Collection(COLL_TASK_CHAT)
		pip := getTaskChatPipline(*taskid, startindex, num)
		err := coll.Aggregate(ctx, pip).One(&chatinfo)
		return err
	})
	if err != nil {
		util.Log_error("onGetOneTaskChat: %s", err.(error).Error())
	}
	m.sendClientJson(um.user.connid, handle.SM_CHAT_UPDATE, chatinfo)
}

// 获取玩家加入的和创建的聊天index
func (m *ClientServerModule) getTaskChatIndex(cid int) []mg_task_chat {
	var res []mg_task_chat
	var res2 []mg_task_chat

	pipline := bson.A{
		bson.M{"$match": bson.M{"cid": cid}},
		bson.M{"$lookup": bson.D{
			{Key: "from", Value: COLL_TASK_CHAT},
			{Key: "localField", Value: "tasklist._id"},
			{Key: "foreignField", Value: "_id"},
			{Key: "pipeline", Value: bson.A{
				bson.M{"$project": bson.M{"count": 1, "index": 1,
					"data": bson.M{"$slice": bson.A{"$data", -1, 1}},
				}},
			}},
			{Key: "as", Value: "result"},
		},
		},
		bson.M{"$unwind": "$result"},
		bson.M{"$replaceRoot": bson.M{"newRoot": "$result"}},
	}

	m._mg_mgr.RequestFuncCallNoRes(func(ctx context.Context, qc *qmgo.QmgoClient) {
		joincoll := qc.Database.Collection(COLL_USER_TASK)
		err := joincoll.Aggregate(ctx, pipline).All(&res)
		if err != nil {
			util.Log_error("getTaskChatIndex 1:%s", err.Error())
		}

		createcoll := qc.Database.Collection(COLL_USER_CREATE_TASK)
		err = createcoll.Aggregate(ctx, pipline).All(&res2)
		if err != nil {
			util.Log_error("getTaskChatIndex 2:%s", err.Error())
		}
	})
	res = append(res, res2...)
	return res
}

func (m *ClientServerModule) sendUserTaskChatIndex(u *appUserInfo) {
	vec := m.getTaskChatIndex(int(u.cid))
	if vec == nil {
		vec = make([]mg_task_chat, 0)
	}
	m.sendClientJson(u.connid, handle.SM_CHAT_INDEX, vec)
}

type b_task_chat_read struct {
	Cid    int64  `gorm:"primaryKey" json:"cid"`
	TaskId string `gorm:"primaryKey;column:taskid" json:"taskid"`
	Index  int    `json:"index"`
}

type b_user_chat_read struct {
	Cid   int64 `gorm:"primaryKey" json:"cid"`
	Tocid int64 `gorm:"primaryKey;column:tocid" json:"tocid"`
	Index int   `json:"index"`
}

func (m *ClientServerModule) getUserTaskChatRead(u *appUserInfo) []b_task_chat_read {
	var read []b_task_chat_read
	m._sql_mgr.RequestFuncCallHash(uint32(u.cid), func(d *gorm.DB) interface{} {
		res := d.Table("b_task_chat_read").Find(&read, u.cid)
		if res.Error != nil {
			util.Log_error("select task read %s", res.Error.Error())
		}
		return nil
	})
	return read
}

func (m *ClientServerModule) getUserChatRead(u *appUserInfo) []b_user_chat_read {
	var read []b_user_chat_read
	m._sql_mgr.RequestFuncCallHash(uint32(u.cid), func(d *gorm.DB) interface{} {
		res := d.Table("b_user_chat_read").Find(&read, u.cid)
		if res.Error != nil {
			util.Log_error("select task read %s", res.Error.Error())
		}
		return nil
	})
	return read
}

func (m *ClientServerModule) sendUserTaskChatRead(u *appUserInfo) {
	vec := m.getUserTaskChatRead(u)
	if vec == nil {
		vec = make([]b_task_chat_read, 0)
	}
	m.sendClientJson(u.connid, handle.SM_CHAT_READ, vec)
}

func (m *ClientServerModule) sendUserChatRead(u *appUserInfo) {
	vec := m.getUserChatRead(u)
	if vec == nil {
		vec = make([]b_user_chat_read, 0)
	}
	m.sendClientJson(u.connid, handle.SM_CHAT_USER_READ, vec)
}

func (m *ClientServerModule) onTaskChatRead(msg *handle.BaseMsg) {
	um := msg.Data.(*userMsg)
	num := int(um.msg.ReadInt32())
	var idvec [][]byte
	var indexvec []int32
	for i := 0; i < num; i++ {
		idvec = append(idvec, um.msg.ReadString())
		indexvec = append(indexvec, um.msg.ReadInt32())
	}

	m._sql_mgr.RequestFuncCallNoResHash(uint32(um.user.cid), func(d *gorm.DB) {
		for i := 0; i < len(idvec); i++ {
			res := d.Exec("INSERT INTO `b_task_chat_read` VALUES(?,?,?) ON DUPLICATE KEY UPDATE `index`=?;", um.user.cid, idvec[i], indexvec[i], indexvec[i])
			if res.Error != nil {
				util.Log_error("update task read %s", res.Error.Error())
			}
		}
	})
}

func (m *ClientServerModule) onDeleteTaskChatRead(msg *handle.BaseMsg) {
	um := msg.Data.(*userMsg)
	taskid := um.msg.ReadString()
	m._sql_mgr.RequestFuncCallNoResHash(uint32(um.user.cid), func(d *gorm.DB) {
		res := d.Table("b_task_chat_read").Delete(&b_task_chat_read{
			Cid:    um.user.cid,
			TaskId: string(taskid),
		})
		if res.Error != nil {
			util.Log_error("delete task read %s", res.Error.Error())
		}
	})
}

func (m *ClientServerModule) onClientChatUser(msg *handle.BaseMsg) {
	um := msg.Data.(*userMsg)
	// json
	var chat mg_chat_user
	err := json.Unmarshal(um.msg.ReadBuff(), &chat)
	if err != nil {
		util.Log_error("chat user err:%s", err.Error())
		return
	}

	if chat.ContentType <= CHAT_TYPE_BEGIN || chat.ContentType >= CHAT_TYPE_END || len(chat.Content) > 5000 ||
		chat.Tocid <= 0 {
		return
	}

	user := um.user
	chat.Cid = user.cid
	chat.SendTime = util.GetSecond()
	chat.Sendername = user.user.Name
	chat.Sendericon = user.user.Icon
	repid := chat.Chatid
	chat.Chatid = util.GetMillisecond()

	// 插入数据库
	// res := m._mg_mgr.RequestFuncCall(func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
	// 	// 是否黑名单
	// 	if checkInBlackList(qc, ctx, chat.Tocid, user.cid) {
	// 		return false
	// 	}

	// 	coll := qc.Database.Collection(COLL_CHAT_USER)
	// 	_, err := coll.InsertOne(ctx, chat)
	// 	if err != nil {
	// 		util.Log_error("user chat err:%s", err.Error())
	// 	}
	// 	return true
	// }).(bool)

	uchat := Mg_chat{
		Cid:         user.cid,
		Sendername:  user.user.Name,
		Sendericon:  user.user.Icon,
		SendTime:    chat.SendTime,
		Content:     chat.Content,
		ContentType: chat.ContentType,
	}

	cidhei := user.cid
	cidlow := chat.Tocid
	if chat.Tocid > user.cid {
		cidhei = chat.Tocid
		cidlow = user.cid
	}
	// user chat list
	chatlist := mg_chat_user_list{
		Cidhei:   cidhei,
		Cidlow:   cidlow,
		UpdateAt: uchat.SendTime,
	}
	res := m._mg_mgr.RequestFuncCall(func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		// 是否黑名单
		if checkInBlackList(qc, ctx, chat.Tocid, user.cid) {
			return false
		}

		coll := qc.Database.Collection(COLL_CHAT_USER_LIST)
		coll.Find(ctx, bson.M{"cidhei": cidhei, "cidlow": cidlow}).Select(bson.M{"_id": 1, "count": 1, "indexid": 1}).One(&chatlist)

		uchat.Index = chatlist.Indexid
		chatlist.Indexid++
		chatlist.Count++
		chatlist.Data = append(chatlist.Data, uchat)
		if chatlist.Id.IsZero() {
			chatlist.Id = qmgo.NewObjectID()
			// 插入
			_, err := coll.InsertOne(ctx, chatlist)
			if err != nil {
				util.Log_error("insert chat user list err:%s", err.Error())
			}
		} else {
			// 更新
			err := coll.UpdateOne(ctx, bson.M{"_id": chatlist.Id}, bson.M{"$set": bson.M{"indexid": chatlist.Indexid, "count": chatlist.Count, "updateat": chatlist.UpdateAt}, "$push": bson.M{"data": uchat}})
			if err != nil {
				util.Log_error("insert chat user list err:%s", err.Error())
			}
		}
		// 删除旧的记录
		if chatlist.Count >= MAX_USER_CHAT_NUM*2 {
			var tchat mg_chat_user_list
			pip := getTaskChatPipline(chatlist.Id, -MAX_USER_CHAT_NUM, MAX_USER_CHAT_NUM)
			err = coll.Aggregate(ctx, pip).One(&tchat)
			if err == nil {
				// 写回
				tchat.Count = len(tchat.Data)
				err = coll.UpdateOne(ctx, bson.M{"_id": tchat.Id}, bson.M{"$set": bson.M{"data": tchat.Data, "count": tchat.Count}})
				if err == nil {
					chatlist.Count = tchat.Count
				}
			}
		}
		return true
	}).(bool)

	if res {
		// 同步
		touser := m.getUserByCid(chat.Tocid)
		if touser != nil {
			m.sendClientJson(touser.connid, handle.SM_CHAT_USER, []mg_chat_user{chat})
			m.sendClientJson(touser.connid, handle.SM_CHAT_USER_LIST, chatlist)
		}
	}

	pack := network.NewMsgPackDef()
	pack.WriteInt64(chat.Tocid)
	pack.WriteInt64(um.user.cid)
	pack.WriteInt64(repid)
	pack.WriteInt64(chat.Chatid)
	m.sendClientPack(um.user.connid, handle.SM_CHAT_USER_GET, pack)

	m.sendClientJson(um.user.connid, handle.SM_CHAT_USER_LIST, chatlist)
}

func (m *ClientServerModule) loadUserChat(user *appUserInfo) {
	var chats []mg_chat_user
	m._mg_mgr.RequestFuncCall(func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		coll := qc.Database.Collection(COLL_CHAT_USER)
		err := coll.Find(ctx, bson.M{"tocid": user.cid}).All(&chats)
		if err != nil {
			util.Log_error("load user chat err:%s", err.Error())
		}
		return nil
	})
	if len(chats) > 0 {
		m.sendClientJson(user.connid, handle.SM_CHAT_USER, chats)
	}
}

func (m *ClientServerModule) onClientChatUserGet(msg *handle.BaseMsg) {
	um := msg.Data.(*userMsg)

	lowid := um.msg.ReadInt64()
	heighid := um.msg.ReadInt64()

	m._mg_mgr.RequestFuncCallNoRes(func(ctx context.Context, qc *qmgo.QmgoClient) {
		coll := qc.Database.Collection(COLL_CHAT_USER)
		coll.RemoveAll(ctx, bson.M{"tocid": um.user.cid, "chatid": bson.M{"$gte": lowid, "$lte": heighid}})
	})
}

func (m *ClientServerModule) onClientChatUserRead(msg *handle.BaseMsg) {
	um := msg.Data.(*userMsg)
	num := um.msg.ReadInt32()
	var cidvec []int64
	var indexvec []int32
	for i := 0; i < int(num); i++ {
		cidvec = append(cidvec, um.msg.ReadInt64())
		indexvec = append(indexvec, um.msg.ReadInt32())
	}

	m._sql_mgr.RequestFuncCallNoResHash(uint32(um.user.cid), func(d *gorm.DB) {
		for i := 0; i < len(cidvec); i++ {
			res := d.Exec("INSERT INTO `b_user_chat_read` VALUES(?,?,?) ON DUPLICATE KEY UPDATE `index`=?;", um.user.cid, cidvec[i], indexvec[i], indexvec[i])
			if res.Error != nil {
				util.Log_error("update user chat read err: %s", res.Error.Error())
			}
		}
	})
}
