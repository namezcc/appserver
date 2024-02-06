package module

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"goserver/LPMsg"
	"goserver/handle"
	"goserver/util"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/golang/protobuf/proto"
	"github.com/jmoiron/sqlx"
)

type ClientHttpModule struct {
	HttpModule
	_drawlogtime map[string]int64
}

func (m *ClientHttpModule) Init(mgr *moduleMgr) {
	m.HttpModule.Init(mgr)
	m._httpbase = m
	m._drawlogtime = make(map[string]int64)
	m.SetHost(":" + util.GetConfValue("clientPort"))

	handle.Handlemsg.AddMsgCall(handle.M_SAVE_BATTLE_DATA, m.onSaveBattleData)
	handle.Handlemsg.AddMsgCall(handle.M_GET_DRAW_LOG, m.onGetDrawLog)

}

func (m *ClientHttpModule) BeforRun() {
	m.HttpModule.BeforRun()
	if !m.checkLoginGroup() {
		// panic("login check group error not same")
		util.Log_error("login check group error not same")
	}
}

func (m *ClientHttpModule) initRoter(r *gin.Engine) {
	// 服务分离
	r.POST("/saveBattleData", m.saveBattleData)
	r.GET("/drawlog", m.getDrawLog)
	r.GET("/serverTime", m.onGetServerTime)
	r.POST("/saveBattleError", m.saveBattleError)
	r.GET("/battlecatch", m.getClientBattleCatch)
	r.GET("/adultCheck", m.onAdultCheck)
	r.GET("/addAdultInfo", m.onAddAdultInfo)

}

type saveBattle struct {
	stype  int
	pbdata *LPMsg.SaveBattleInfo
}

func (m *ClientHttpModule) saveBattleData(c *gin.Context) {
	d, err := c.GetRawData()
	msg := ""
	defer c.String(http.StatusOK, msg)

	if err != nil {
		log.Println(err)
		return
	}

	savedata := LPMsg.SaveBattleInfo{}
	err = proto.Unmarshal(d, &savedata)
	if err != nil {
		log.Println(err)
		return
	}

	sinfo := saveBattle{
		stype:  0,
		pbdata: &savedata,
	}
	m.sendMsg(handle.M_SAVE_BATTLE_DATA, &sinfo)
}

func (m *ClientHttpModule) saveBattleError(c *gin.Context) {
	d, err := c.GetRawData()
	msg := ""
	defer c.String(http.StatusOK, msg)

	if err != nil {
		log.Println(err)
		return
	}

	savedata := LPMsg.SaveBattleInfo{}
	err = proto.Unmarshal(d, &savedata)
	if err != nil {
		log.Println(err)
		return
	}

	savedata.Time = int32(util.GetSecond())
	sinfo := saveBattle{
		stype:  1,
		pbdata: &savedata,
	}
	m.sendMsg(handle.M_SAVE_BATTLE_DATA, &sinfo)
}

func (m *ClientHttpModule) onSaveBattleData(msg *handle.BaseMsg) {
	sinfo := msg.Data.(*saveBattle)
	savedata := sinfo.pbdata
	m.HttpLock()
	defer m.HttpUnLock()
	var sqlstr string
	if sinfo.stype == 0 {
		sqlstr = fmt.Sprintf("INSERT INTO %s(`cid`,`pbdata`,`battleinfo`,`time`,`base`,`md5`) VALUES(?,?,?,?,?,?);", "client_battle_error")
	} else {
		sqlstr = fmt.Sprintf("INSERT INTO %s(`cid`,`pbdata`,`battleinfo`,`time`,`base`,`md5`) VALUES(?,?,?,?,?,?);", "client_battle_error_catch")
	}
	_, err := m._sql.Exec(sqlstr,
		savedata.Cid, savedata.Pbdata, savedata.Battleinfo, savedata.Time, savedata.Base, savedata.Md5)
	if err != nil {
		log.Println(err)
	}
}

type dbDrawlog struct {
	Cid       int `db:"cid" json:"-"`
	Item_id   int `db:"item_id" json:"item_id"`
	Item_type int `db:"item_type" json:"item_type"`
	Num       int `db:"num" json:"num"`
	Active    int `db:"active" json:"-"`
	Time      int `db:"time" json:"time"`
}

type drawQuery struct {
	cid        int
	startindex int
	activeid   int
	clientip   string
}

func (m *ClientHttpModule) getDrawLog(c *gin.Context) {
	cid, _ := strconv.Atoi(c.Query("cid"))
	startindex, _ := strconv.Atoi(c.Query("index"))
	activeid, _ := strconv.Atoi(c.Query("active"))

	query := &drawQuery{
		cid:        cid,
		startindex: startindex,
		activeid:   activeid,
		clientip:   c.ClientIP(),
	}

	rep := m.requestMsg(&m.modulebase, handle.M_GET_DRAW_LOG, query)
	if rep == nil || rep.data == nil {
		c.String(http.StatusOK, "")
		return
	}
	logs := rep.data.([]dbDrawlog)
	c.JSON(http.StatusOK, logs)
}

func (m *ClientHttpModule) onGetDrawLog(msg *handle.BaseMsg) {
	rep := msg.Data.(*responesData)
	defer m.responesMsgData(rep)

	info := rep.data.(*drawQuery)
	rep.data = nil

	cid := info.cid
	activeid := info.activeid
	startindex := info.startindex
	cip := info.clientip

	gettime, ok := m._drawlogtime[cip]
	now := util.GetSecond()
	if ok && gettime > now {
		return
	}
	m._drawlogtime[cip] = now + 5 //5s限制

	// 获取db地址
	dbid := cid / 100000
	var dbhost dbMysqlHost

	var err error
	func() {
		m.HttpLock()
		defer m.HttpUnLock()
		err = m._sql.Get(&dbhost, "SELECT * FROM `mysql_host` WHERE id=?;", dbid)
	}()
	if err != nil {
		util.Log_error("get dbhost err:%s\n", err.Error())
		return
	}

	host := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8", dbhost.User, dbhost.Pass, dbhost.Ip, dbhost.Port, dbhost.Dbname)
	sql, sqlerr := sqlx.Open("mysql", host)
	if sqlerr != nil {
		util.Log_error("sql open error:%s\n" + sqlerr.Error())
		return
	}

	var logs []dbDrawlog
	if startindex <= 0 {
		err = sql.Select(&logs, "SELECT * FROM log_draw WHERE cid=? AND active=? ORDER BY `time` DESC LIMIT 100;", cid, activeid)
	} else {
		err = sql.Select(&logs, "SELECT * FROM log_draw WHERE cid=? AND active=? AND `time` < ? ORDER BY `time` DESC LIMIT 100;", cid, activeid, startindex)
	}
	if err != nil {
		util.Log_error("get drawlog error:%s\n", err.Error())
	}
	sql.Close()
	rep.data = logs
}

func (m *ClientHttpModule) checkLoginGroup() bool {
	var servers []dbServer
	var loginl []dbLogin

	err := m._sql.Select(&loginl, "SELECT * FROM `login_list`;")
	if err != nil {
		util.Log_error(err.Error())
		return false
	}

	err = m._sql.Select(&servers, "SELECT * FROM `server` WHERE `type`=?;", util.ST_LOGIN)
	if err != nil {
		util.Log_error(err.Error())
		return false
	}

	smap := make(map[int]dbServer)
	for _, v := range servers {
		smap[v.Id] = v
	}

	lmap := make(map[int][]dbLogin)
	for _, v := range loginl {
		vec, ok := lmap[v.Uidgroup]
		if !ok {
			vec = make([]dbLogin, 0)
		}
		vec = append(vec, v)
		lmap[v.Uidgroup] = vec
	}

	for _, vec := range lmap {
		group := 0
		for i, v := range vec {
			ser, ok := smap[v.Login]
			if !ok {
				return false
			}
			if i == 0 {
				group = ser.Group
			} else {
				if group != ser.Group {
					return false
				}
			}
		}
	}
	return true
}

func (m *ClientHttpModule) onGetServerTime(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"time": util.GetSecond()})
}

func (m *ClientHttpModule) getClientBattleCatch(c *gin.Context) {
	id := c.Query("id")
	table := c.DefaultQuery("table", "0")

	if table == "0" {
		table = "client_battle_error_catch"
	} else {
		table = "client_battle_error"
	}

	var bdata dbClientBattleError
	var err error
	func() {
		m._http_lock.Lock()
		defer m._http_lock.Unlock()
		err = m._sql.Get(&bdata, fmt.Sprintf("SELECT * FROM `%s` WHERE `id`=?;", table), id)
	}()
	if err != nil {
		util.Log_error(err.Error())
		c.String(http.StatusOK, "{}")
		return
	}

	c.JSON(http.StatusOK, bdata)
}

type adultInfo struct {
	Platform_uid string `db:"platform_uid" json:"-"`
	Adult        int    `db:"adult" json:"adult"`
	Birth        string `db:"birth" json:"birth"`
}

func (m *ClientHttpModule) onAdultCheck(c *gin.Context) {
	uid := c.Query("uid")
	platform := c.DefaultQuery("platform", "0")

	adult := adultInfo{}
	lockFunc(&m._http_lock, func() {
		err := m._sql.Get(&adult, "SELECT * FROM `adult_info` WHERE `platform_uid`=?;", fmt.Sprintf("%s_%s", platform, uid))
		if err != nil {
			util.Log_error(err.Error())
		}
	})

	haveinfo := 1
	if len(adult.Platform_uid) == 0 {
		haveinfo = 0
	}
	c.JSON(http.StatusOK, gin.H{"adult": adult.Adult, "check": haveinfo, "birth": adult.Birth})
}

const httpsecret = "aqwfasdfa"

func (m *ClientHttpModule) onAddAdultInfo(c *gin.Context) {
	uid := c.Query("uid")
	platform := c.DefaultQuery("platform", "0")
	adult := c.DefaultQuery("adult", "0")
	time := c.Query("time")
	birth := c.Query("birth")
	sign := c.Query("sign")

	itime, _ := strconv.Atoi(time)
	if int64(itime+5) < util.GetSecond() {
		c.JSON(http.StatusOK, gin.H{"code": 3, "msg": "时间过期"})
		return
	}

	cstr := uid + platform + adult + time + birth + httpsecret
	mdns := md5.Sum([]byte(cstr))
	ns := hex.EncodeToString(mdns[:])

	if sign != ns {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "校验失败"})
		return
	}

	code := 0
	msg := "成功"
	lockFunc(&m._http_lock, func() {
		_, err := m._sql.Exec("INSERT INTO `adult_info`(`platform_uid`,`adult`,`birth`) VALUES(?,?,?);", fmt.Sprintf("%s_%s", platform, uid), adult, birth)
		if err != nil {
			util.Log_error(err.Error())
			code = 2
			msg = "数据库错误"
		}
	})

	c.JSON(http.StatusOK, gin.H{"code": code, "msg": msg})
}
