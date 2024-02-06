package module

import (
	"context"
	"fmt"
	"goserver/clib"
	"goserver/handle"
	"goserver/util"
	"net/http"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

type LoginHttpModule struct {
	HttpModule
	_account_sql  *sqlx.DB
	_account_lock sync.Mutex
}

func (m *LoginHttpModule) Init(mgr *moduleMgr) {
	m.HttpModule.Init(mgr)
	m._httpbase = m
	m.SetHost(":" + util.GetConfValue("loginPort"))
	m._account_sql = util.NewSqlConn(util.GetConfValue("mysql_account"))

	handle.Handlemsg.AddMsgCall(handle.M_GET_LOGIN_LIST, m.onGetLoginList)
	m.initRedis()
}

func (m *LoginHttpModule) BeforRun() {
	m.HttpModule.BeforRun()

}

func (m *LoginHttpModule) initRoter(r *gin.Engine) {
	r.GET("/loginlist", m.getLoginList)
	r.GET("/accountRole", m.getAccountRole)

}

func (m *LoginHttpModule) getLoginList(c *gin.Context) {
	platform := c.DefaultQuery("platform", "0")
	// rep := m.requestMsg(&m.modulebase, handle.M_GET_LOGIN_LIST, platform)

	var loginvec []string
	var err error
	online := make(map[string]string)
	lockFunc(&m._redis_lock, func() {
		doRedis(func(ctx context.Context) {
			mem := m._rdb.SMembers(ctx, "loginlist:"+platform)
			loginvec, err = mem.Result()
			if err != nil {
				util.Log_error(err.Error())
			}
			online, err = m._rdb.HGetAll(ctx, "loginserver").Result()
			if err != nil {
				util.Log_error("redis get loginserver error:%s\n", err.Error())
			}
		})
	})

	svers := make([]dbLogin, 0)
	for _, v := range loginvec {
		login := dbLogin{}
		login.decode(v)
		svers = append(svers, login)
	}

	logins := make([]LoginInfo, len(svers))
	for i := 0; i < len(svers); i++ {
		h, ok := online[strconv.Itoa(svers[i].Login)]
		if !ok {
			h = ""
		}
		logins[i] = LoginInfo{
			Id:       svers[i].Id,
			Host:     h,
			ShowName: svers[i].Showname,
			Uidgroup: svers[i].Uidgroup,
		}
	}
	c.JSON(http.StatusOK, logins)
}

func (m *LoginHttpModule) onGetLoginList(msg *handle.BaseMsg) {
	rep := msg.Data.(*responesData)
	defer m.responesMsgData(rep)

	var svers []dbLogin
	err := m._sql.Select(&svers, "SELECT * FROM `login_list`;")
	if err != nil {
		util.Log_error(err.Error())
		return
	}

	online := make(map[string]string)
	doRedis(func(ctx context.Context) {
		online, err = m._rdb.HGetAll(ctx, "loginserver").Result()
		if err != nil {
			util.Log_error("redis get loginserver error:%s\n", err.Error())
		}
	})

	logins := make([]LoginInfo, len(svers))
	for i := 0; i < len(svers); i++ {
		h, ok := online[strconv.Itoa(svers[i].Login)]
		if !ok {
			h = ""
		}
		logins[i] = LoginInfo{
			Id:       svers[i].Id,
			Host:     h,
			ShowName: svers[i].Showname,
			Uidgroup: svers[i].Uidgroup,
		}
	}
	rep.data = logins
}

type dbUidToHashId struct {
	Hashid int `db:"hashid"`
	Nowid  int `db:"nowid"`
	Oldid  int `db:"oldid"`
}

type dbRoleInfo struct {
	Platform_uid string `db:"platform_uid" json:"uid"`
	Loginid      int    `db:"loginid" json:"loginid"`
	Level        int    `db:"level" json:"level"`
	Logout_time  int    `db:"logout_time" json:"logout_time"`
	Cid          int    `db:"cid" json:"cid"`
}

func (m *LoginHttpModule) getAccountRole(c *gin.Context) {
	platform := c.DefaultQuery("platform", "0")
	uid := c.Query("uid")
	uid = fmt.Sprintf("%s_%s", platform, uid)
	hashval := clib.Crc32(uid)
	// nhash := crc32.ChecksumIEEE([]byte(uid))
	hidx := hashval%20 + 1

	var roles []dbRoleInfo

	func() {
		m._account_lock.Lock()
		defer m._account_lock.Unlock()

		var uidhid dbUidToHashId
		err := m._account_sql.Get(&uidhid, "SELECT * FROM `platform_uid_to_role_id` WHERE `hashid`=?;", hidx)
		if err != nil {
			util.Log_error("platform_uid_to_role_id uid:%s error:%s", uid, err.Error())
			return
		}

		err = m._account_sql.Select(&roles, fmt.Sprintf("SELECT * FROM `all_role_level_%d` WHERE `platform_uid`=?;", uidhid.Oldid), uid)
		if err != nil {
			util.Log_error("all_role_level_%d uid:%s error:%s", uidhid.Oldid, uid, err.Error())
		}
	}()

	c.JSON(http.StatusOK, roles)
}
