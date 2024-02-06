package module

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"goserver/handle"
	"goserver/util"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

type JWTClaims struct { // token里面添加用户信息，验证token后可能会用到用户信息
	jwt.StandardClaims
	UserID      int      `json:"user_id"`
	Password    string   `json:"password"`
	Username    string   `json:"username"`
	FullName    string   `json:"full_name"`
	Permissions []string `json:"permissions"`
}

var (
	Secret     = "LiuBei"  // 加盐
	ExpireTime = 3600 * 24 // token有效期
)

func Verify(c *gin.Context) (result bool, userName string, err error) {
	strToken := c.Request.Header.Get("Authorization")
	if strToken == "" {
		result = false
		return result, "", nil
	}
	claim, err := verifyAction(strToken)
	if err != nil {
		result = false
		util.Log_error("verify err:%s", err.Error())
		return result, "", nil
	}
	result = true
	userName = claim.Username
	return result, userName, nil
}

// 刷新token
func Refresh(c *gin.Context) {
	strToken := c.Request.Header.Get("Authorization")
	claims, err := verifyAction(strToken)
	if err != nil {
		c.JSON(400, gin.H{"err": err.Error()})
		return
	}
	claims.ExpiresAt = time.Now().Unix() + (claims.ExpiresAt - claims.IssuedAt)
	signedToken, err := getToken(claims)
	if err != nil {
		c.JSON(400, gin.H{"err": err.Error()})
		return
	}
	c.JSON(200, gin.H{"data": signedToken})
}

// 验证token是否存在，存在则获取信息
func verifyAction(strToken string) (claims *JWTClaims, err error) {
	token, err := jwt.ParseWithClaims(strToken, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(Secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		return nil, err
	}
	if err := token.Claims.Valid(); err != nil {
		return nil, err
	}
	return claims, nil
}

// 生成token
func getToken(claims *JWTClaims) (signedToken string, err error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err = token.SignedString([]byte(Secret))
	if err != nil {
		return "", err
	}
	return signedToken, nil
}

type MasterHttpModule struct {
	HttpModule
	_master_mod   *MasterModule
	_safe_req     map[string]bool
	_account_sql  *sqlx.DB
	_account_lock sync.Mutex
}

func (m *MasterHttpModule) Init(mgr *moduleMgr) {
	m.HttpModule.Init(mgr)
	m._httpbase = m
	m.SetHost(":" + util.GetConfValue("masterport"))
	m._safe_req = make(map[string]bool)
	m._master_mod = mgr.GetModule(MOD_MASTER).(*MasterModule)
	m.initRedis()
	m._account_sql = util.NewSqlConn(util.GetConfValue("mysql_account"))
}

func (m *MasterHttpModule) BeforRun() {
	m.HttpModule.BeforRun()
	m.loadLoginListToRedis()
}

func (m *MasterHttpModule) getMiddlew() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, ok := m._safe_req[c.FullPath()]
		if ok {
			c.Next()
			return
		}

		checkUser, _, err := Verify(c) //第二个值是用户名，这里没有使用
		if err != nil {
			c.JSON(400, gin.H{"err": err.Error()})
			c.Abort()
		}
		if checkUser == false {
			c.JSON(400, gin.H{"err": "身份认证失败"})
			c.Abort()
		}
		c.Next()
	}
}

func (m *MasterHttpModule) safeGet(r *gin.Engine, p string, h gin.HandlerFunc) {
	m._safe_req[p] = true
	r.GET(p, h)
}

func (m *MasterHttpModule) safePost(r *gin.Engine, p string, h gin.HandlerFunc) {
	m._safe_req[p] = true
	r.POST(p, h)
}

func (m *MasterHttpModule) initRoter(r *gin.Engine) {
	r.Use(m.getMiddlew())

	m.safeGet(r, "/battleData", m.getBattleData)
	m.safeGet(r, "/clientBattleData", m.getClientBattleData)
	m.safePost(r, "/userlogin", m.userLogin)

	// 需要鉴权 >>>
	r.POST("/hotload", m.reqLoginHotLoad)
	r.GET("/serverInfo", m.getServerState)
	r.GET("/yamlsfind", m.onMakeyamlSfind)
	r.GET("/yamlserver", m.onMakeyamlServer)
	r.GET("/refreshLoginList", m.onRefreshLoginList)
	r.POST("/addMysqlHost", m.onAddMysqlHost)
	r.POST("/addServer", m.onAddServer)
	r.POST("/updateServer", m.onUpdateServer)
	r.GET("/deleteServer", m.onDeleteServer)
	r.GET("/serverTable", m.onGetServerTable)
	r.POST("/addServerMail", m.onAddServerMail)
	r.POST("/addPlayerMail", m.onAddPlayerMail)

}

type dbBattleData struct {
	Id       int    `db:"id" json:"id"`
	Serverid int    `db:"serverid" json:"serverid"`
	Time     int    `db:"time" json:"time"`
	Compress int    `db:"compress" json:"compress"`
	Orglen   int    `db:"orglen" json:"orglen"`
	Data     []byte `db:"data" json:"data"`
}

func (m *MasterHttpModule) getBattleData(c *gin.Context) {
	id := c.Query("id")
	var bdata dbBattleData
	var err error
	func() {
		m._http_lock.Lock()
		defer m._http_lock.Unlock()
		err = m._sql.Get(&bdata, "SELECT * FROM `battledata` WHERE `id`=?;", id)
	}()
	if err != nil {
		util.Log_error(err.Error())
		c.String(http.StatusOK, "{}")
		return
	}

	c.JSON(http.StatusOK, bdata)
}

type dbClientBattleError struct {
	Id         int    `db:"id" json:"id"`
	Cid        int    `db:"cid" json:"cid"`
	Pbdata     []byte `db:"pbdata" json:"pbdata"`
	Battleinfo string `db:"battleinfo" json:"battleinfo"`
	Time       int    `db:"time" json:"time"`
	Base       int    `db:"base" json:"base"`
	Md5        string `db:"md5" json:"md5"`
}

func (m *MasterHttpModule) getClientBattleData(c *gin.Context) {
	id := c.Query("id")
	var bdata dbClientBattleError
	var err error
	func() {
		m._http_lock.Lock()
		defer m._http_lock.Unlock()
		err = m._sql.Get(&bdata, "SELECT * FROM `client_battle_error` WHERE `id`=?;", id)
	}()
	if err != nil {
		util.Log_error(err.Error())
		c.String(http.StatusOK, "{}")
		return
	}

	c.JSON(http.StatusOK, bdata)
}

func (m *MasterHttpModule) reqLoginHotLoad(c *gin.Context) {
	buf, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.String(http.StatusOK, err.Error())
		return
	}

	var info hotInfo

	err = json.Unmarshal(buf, &info)
	if err != nil {
		c.String(http.StatusOK, err.Error())
		return
	}

	// println(info.Lua)
	c.String(http.StatusOK, "ok")
	m._master_mod.sendMsg(handle.M_HOT_LOAD, info)
}

type userInfo struct {
	Name string `json:"name"`
	Pass string `json:"pass"`
}

type dbAdmin struct {
	Name string `db:"name" json:"name"`
	Pass string `db:"pass" json:"pass"`
}

func (m *MasterHttpModule) userLogin(c *gin.Context) {
	var info userInfo
	err := c.ShouldBindJSON(&info)

	if err != nil {
		c.JSON(http.StatusOK, gin.H{"err": err.Error()})
		return
	}

	var dbuser dbAdmin
	func() {
		m._http_lock.Lock()
		defer m._http_lock.Unlock()
		err = m._sql.QueryRow("SELECT * FROM `master_user` WHERE `name`=?;", info.Name).Scan(&dbuser.Name, &dbuser.Pass)
	}()
	if err != nil {
		util.Log_error(err.Error())
		c.JSON(http.StatusOK, gin.H{"err": err.Error()})
		return
	}

	if info.Pass == dbuser.Pass {
		claims := &JWTClaims{
			UserID:      1,
			Username:    info.Name,
			Password:    info.Pass,
			FullName:    info.Name,
			Permissions: []string{},
		}
		claims.IssuedAt = time.Now().Unix()
		claims.ExpiresAt = time.Now().Add(time.Second * time.Duration(ExpireTime)).Unix()
		signedToken, err := getToken(claims)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"err": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"token": signedToken})
	} else {
		c.JSON(http.StatusOK, gin.H{"err": "密码错误"})
	}
}

type servernode struct {
	Type  int         `json:"type"`
	Id    int         `json:"id"`
	Name  string      `json:"name"`
	Ip    string      `json:"ip"`
	Open  int         `json:"open"`
	Error int         `json:"error"`
	Info  dbServer    `json:"info"`
	Link  map[int]int `json:"link"`
}

type serverState struct {
	state int
	addr  string
	link  map[int]int
}

func (m *MasterHttpModule) getServerState(c *gin.Context) {

	pack := m.requestMsg(&m._master_mod.modulebase, handle.M_GET_SERVER_STATE, nil)
	if pack == nil {
		c.String(http.StatusOK, "[]")
		return
	}

	var serverInfo []dbServer
	var err error
	func() {
		m._http_lock.Lock()
		defer m._http_lock.Unlock()
		err = m._sql.Select(&serverInfo, "SELECT * FROM `server`;")
	}()

	if err != nil {
		util.Log_error(err.Error())
		c.String(http.StatusOK, "[]")
		return
	}

	serstate := pack.data.(map[int]serverState)

	snode := make([]servernode, 0, len(serverInfo))

	for _, v := range serverInfo {
		key := (v.Type << 16) | v.Id
		sv, ok := serstate[key]
		open := 0
		if ok {
			open = 1
		}
		if v.Type == util.ST_MASTER {
			open = 1
		}

		snode = append(snode, servernode{
			Type:  v.Type,
			Id:    v.Id,
			Name:  v.Name,
			Ip:    sv.addr,
			Open:  open,
			Error: sv.state,
			Info:  v,
			Link:  sv.link,
		})
	}

	c.JSON(http.StatusOK, snode)
}

type yamlSfind struct {
	Version string          `json:"version"`
	Server  []dbServiceFind `json:"server"`
}

func (m *MasterHttpModule) onMakeyamlSfind(c *gin.Context) {
	version := c.Query("version")
	if version == "" {
		c.String(http.StatusOK, "version error")
		return
	}

	var err error
	defer func() {
		if err != nil {
			c.String(http.StatusOK, err.Error())
		}
	}()

	var svec []dbServiceFind
	func() {
		m._http_lock.Lock()
		defer m._http_lock.Unlock()
		err := m._sql.Select(&svec, "SELECT * FROM `service_find`;")
		if err != nil {
			return
		}
	}()

	ysfind := yamlSfind{
		Version: version,
		Server:  svec,
	}

	url := util.GetConfValue("k8serrvice") + "/makeyamlSvFind"

	pdata, err := json.Marshal(ysfind)
	if err != nil {
		return
	}
	req, _ := http.NewRequest("POST", url, bytes.NewReader(pdata))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	} else {
		msg, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return
		}
		c.String(http.StatusOK, string(msg))
	}
}

func (m *MasterHttpModule) onMakeyamlServer(c *gin.Context) {
	stype := c.Query("type")
	sid := c.Query("id")
	sgroup := c.Query("group")

	wcon := ""
	if stype != "" {
		if len(wcon) > 0 {
			wcon += " AND `type`=" + stype
		} else {
			wcon += " `type`=" + stype
		}
	}
	if sid != "" {
		if len(wcon) > 0 {
			wcon += " AND `id`=" + sid
		} else {
			wcon += " `id`=" + sid
		}
	}
	if sgroup != "" {
		if len(wcon) > 0 {
			wcon += " AND `group`=" + sgroup
		} else {
			wcon += " `group`=" + sgroup
		}
	}

	var err error
	defer func() {
		if err != nil {
			c.String(http.StatusOK, err.Error())
		}
	}()
	var svec []dbServer
	func() {
		m._http_lock.Lock()
		defer m._http_lock.Unlock()
		if wcon == "" {
			err = m._sql.Select(&svec, "SELECT * FROM `server`;")
		} else {
			err = m._sql.Select(&svec, fmt.Sprintf("SELECT * FROM `server` WHERE %s;", wcon))
		}
		if err != nil {
			return
		}
	}()

	url := util.GetConfValue("k8serrvice") + "/makeyaml"

	pdata, err := json.Marshal(svec)
	if err != nil {
		return
	}
	req, _ := http.NewRequest("POST", url, bytes.NewReader(pdata))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	} else {
		msg, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return
		}
		c.String(http.StatusOK, string(msg))
	}
}

func (m *MasterHttpModule) loadLoginListToRedis() {
	var svec []dbServer
	lockFunc(&m._http_lock, func() {
		err := m._sql.Select(&svec, "SELECT * FROM `server` WHERE `type`=?", util.ST_LOGIN)
		if err != nil {
			util.Log_error(err.Error())
		}
	})

	if len(svec) == 0 {
		return
	}

	var logvec []dbLogin
	lockFunc(&m._http_lock, func() {
		err := m._sql.Select(&logvec, "SELECT * FROM `login_list`;")
		if err != nil {
			util.Log_error(err.Error())
		}
	})

	logmap := make(map[int][]dbLogin)
	for _, v := range logvec {
		lm, ok := logmap[v.Login]
		if !ok {
			lm = make([]dbLogin, 0)
		}
		lm = append(lm, v)
		logmap[v.Login] = lm
	}

	platmap := make(map[string]map[int]bool)
	for _, v := range svec {
		platvec := strings.Split(v.Platform, ",")
		for _, ps := range platvec {
			lm, ok := platmap[ps]
			if !ok {
				lm = make(map[int]bool)
				platmap[ps] = lm
			}
			lm[v.Id] = true
		}
	}

	for platid, v := range platmap {
		vec := make([]string, 0)
		for sid := range v {
			lvec, ok := logmap[sid]
			if ok {
				for _, login := range lvec {
					vec = append(vec, login.encode())
				}
			}
		}

		key := "loginlist:" + platid
		lockFunc(&m._redis_lock, func() {
			doRedis(func(ctx context.Context) {
				m._rdb.Del(ctx, key)
				m._rdb.SAdd(ctx, key, vec)
			})
		})
	}
}

func (m *MasterHttpModule) onRefreshLoginList(c *gin.Context) {
	m.loadLoginListToRedis()
	c.String(http.StatusOK, "")
}

type jsGameMysqlHost struct {
	Id           int    `json:"id"`
	Ip           string `json:"ip"`
	Port         int    `json:"port"`
	Dbname       string `json:"dbname"`
	Maxnum       int    `json:"maxnum"`
	AccountTable int    `json:"account_table"`
}

type dbPlayerNumInfo struct {
	Dbid         int `db:"dbid" json:"dbid"`
	Num          int `db:"num" json:"num"`
	Maxnum       int `db:"maxnum" json:"maxnum"`
	AccountTable int `db:"account_table" json:"account_table"`
}

const dbuser string = "root"

func (m *MasterHttpModule) onAddMysqlHost(c *gin.Context) {
	c.Header("content-type", "text/plain; charset=utf-8")
	var hostvec []jsGameMysqlHost
	err := c.ShouldBindJSON(&hostvec)
	if err != nil {
		c.String(http.StatusOK, err.Error())
		return
	}
	for _, gamehost := range hostvec {
		var dbmysql dbMysqlHost
		lockFunc(&m._http_lock, func() {
			m._sql.Get(&dbmysql, "SELECT * FROM `mysql_host` WHERE `id`=?;", gamehost.Id)
		})

		if dbmysql.Id > 0 {
			if dbmysql.Ip != gamehost.Ip || dbmysql.Dbname != gamehost.Dbname {
				c.String(http.StatusOK, "allready have diff host")
				return
			}
		} else {
			lockFunc(&m._http_lock, func() {
				_, err = m._sql.Exec("INSERT INTO `mysql_host` (`id`,`ip`,`port`,`dbname`,`user`,`pass`,`type`) VALUES(?,?,?,?,?,?,?);",
					gamehost.Id, gamehost.Ip, gamehost.Port, gamehost.Dbname, dbuser, util.GetConfValue("gamedbpass"), 0)
			})
			if err != nil {
				c.String(http.StatusOK, err.Error())
				return
			}
		}

		var dbnum dbPlayerNumInfo
		lockFunc(&m._account_lock, func() {
			m._account_sql.Get(&dbnum, "SELECT * FROM `db_player_num_info` WHERE `dbid`=?;", gamehost.Id)
		})
		if dbnum.Dbid > 0 {
			c.String(http.StatusOK, "dbplayernuminfo allready have data:%d", gamehost.Id)
			return
		}

		lockFunc(&m._account_lock, func() {
			_, err = m._account_sql.Exec("INSERT INTO `db_player_num_info` (`dbid`,`num`,`maxnum`,`account_table`) VALUES(?,?,?,?);",
				gamehost.Id, 0, gamehost.Maxnum, gamehost.AccountTable)
		})
		if err != nil {
			c.String(http.StatusOK, err.Error())
			return
		}
	}

	c.String(http.StatusOK, "ok")
	// 通知server
	m._master_mod.sendMsg(handle.M_NOTICE_DB_NUM_INFO_CHANGE, nil)
}

type jsDbServer struct {
	Type     int    `json:"type"`
	Id       int    `json:"id"`
	Name     string `json:"name"`
	Group    int    `json:"group"`
	Port     int    `json:"port"`
	Mysql    string `json:"mysql"`
	Redis    string `json:"redis"`
	Version  string `json:"version"`
	Platform string `json:"platform"`
}

var server_mysql_type = map[int]int{
	util.ST_MYSQL:        util.MYSQL_TYPE_GAME,
	util.ST_LOGIN_LOCK:   util.MYSQL_TYPE_ACCOUNT,
	util.ST_LOGSERVER:    util.MYSQL_TYPE_GAME,
	util.ST_PUBLICK:      util.MYSQL_TYPE_MASTER,
	util.ST_ADMIN_MGR:    util.MYSQL_TYPE_ACCOUNT,
	util.ST_ACCOUNT_ROLE: util.MYSQL_TYPE_ACCOUNT,
	util.ST_MONITOR:      util.MYSQL_TYPE_MASTER,
	util.ST_MASTER:       util.MYSQL_TYPE_MASTER,
	util.ST_OPERATE:      util.MYSQL_TYPE_MASTER,
}

func (m *MasterHttpModule) onAddServer(c *gin.Context) {
	c.Header("content-type", "text/plain; charset=utf-8")
	var jsser []jsDbServer
	err := c.ShouldBindJSON(&jsser)
	defer func() {
		if err != nil {
			c.String(http.StatusOK, err.Error())
		}
	}()

	if err != nil {
		return
	}

	for _, v := range jsser {
		var dbser dbServer
		lockFunc(&m._http_lock, func() {
			m._sql.Get(&dbser, "SELECT * FROM `server` WHERE `type`=? AND `id`=?;", v.Type, v.Id)
		})

		if dbser.Id > 0 {
			c.Writer.WriteString(fmt.Sprintf("all ready have server type:%d id:%d", v.Type, v.Id))
		} else {
			var dberr bool
			dberr, err = m.checkMysql(&v, c)
			if err != nil {
				return
			}

			if dberr {
				continue
			}

			lockFunc(&m._http_lock, func() {
				_, err = m._sql.Exec("INSERT INTO `server` (`type`,`id`,`name`,`group`,`port`,`mysql`,`redis`,`version`,`platform`) VALUES(?,?,?,?,?,?,?,?,?);",
					v.Type, v.Id, v.Name, v.Group, v.Port, v.Mysql, v.Redis, v.Version, v.Platform)
			})
			if err != nil {
				return
			}
		}
	}
	c.String(http.StatusOK, "ok")
}

func (m *MasterHttpModule) checkMysql(v *jsDbServer, c *gin.Context) (bool, error) {
	//检查 mysql
	var dbhost []dbMysqlHost
	var err error
	res := false

	lockFunc(&m._http_lock, func() {
		err = m._sql.Select(&dbhost, "SELECT * FROM `mysql_host` WHERE `id` in(?);", v.Mysql)
	})
	if err != nil {
		return res, err
	}

	sqltype, ok := server_mysql_type[v.Type]
	if !ok {
		if len(dbhost) > 0 {
			c.Writer.WriteString(fmt.Sprintf("server not need mysql type:%d id:%d", v.Type, v.Id))
		}
	} else {
		if len(dbhost) == 0 {
			c.Writer.WriteString(fmt.Sprintf("server need mysql type:%d id:%d", v.Type, v.Id))
		} else {
			dberr := false
			mh := dbhost[0]
			for i := 0; i < len(dbhost); i++ {
				tmp := dbhost[i]
				if i > 0 {
					if mh.Ip != tmp.Ip || mh.Dbname != tmp.Dbname {
						dberr = true
						break
					}
				}
				if tmp.Type != sqltype {
					dberr = true
					break
				}
			}
			if dberr {
				c.Writer.WriteString(fmt.Sprintf("server mysql error type:%d id:%d", v.Type, v.Id))
			} else {
				res = true
			}
		}
	}
	return res, nil
}

func (m *MasterHttpModule) onUpdateServer(c *gin.Context) {
	c.Header("content-type", "text/plain; charset=utf-8")
	var jsser jsDbServer
	err := c.ShouldBindJSON(&jsser)
	defer func() {
		if err != nil {
			c.String(http.StatusOK, err.Error())
		}
	}()
	if err != nil {
		return
	}

	var dberr bool
	dberr, err = m.checkMysql(&jsser, c)
	if dberr {
		return
	}

	lockFunc(&m._http_lock, func() {
		_, err = m._sql.Exec("UPDATE `server` SET `name`=?,`group`=?,`port`=?,`mysql`=?,`redis`=?,`version`=?,`platform`=? WHERE `type`=? AND `id`=?;",
			jsser.Name, jsser.Group, jsser.Port, jsser.Mysql, jsser.Redis, jsser.Version, jsser.Platform, jsser.Type, jsser.Id)
	})
	if err == nil {
		c.String(http.StatusOK, "ok")
	}
}

func (m *MasterHttpModule) onDeleteServer(c *gin.Context) {
	st := c.Query("type")
	sid := c.Query("id")

	lockFunc(&m._http_lock, func() {
		_, err := m._sql.Exec("DELETE FROM `server` WHERE `type`=? AND `id`=?;", st, sid)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"data": err.Error()})
		} else {
			c.JSON(http.StatusOK, gin.H{"data": "ok"})
		}
	})
}

type serverTable struct {
	Server    []dbServer        `json:"server"`
	Mysql     []dbMysqlHost     `json:"mysql"`
	PlayerNum []dbPlayerNumInfo `json:"playernum"`
}

func (m *MasterHttpModule) onGetServerTable(c *gin.Context) {
	sertab := serverTable{}
	lockFunc(&m._account_lock, func() {
		err := m._account_sql.Select(&sertab.PlayerNum, "SELECT * FROM `db_player_num_info`;")
		if err != nil {
			util.Log_error(err.Error())
		}
	})
	lockFunc(&m._http_lock, func() {
		err := m._sql.Select(&sertab.Mysql, "SELECT * FROM `mysql_host`;")
		if err != nil {
			util.Log_error(err.Error())
		}
		err = m._sql.Select(&sertab.Server, "SELECT * FROM `server`;")
		if err != nil {
			util.Log_error(err.Error())
		}
	})
	c.JSON(http.StatusOK, sertab)
}

type ServerMail struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Reward  string `json:"reward"`
}

func (m *MasterHttpModule) onAddServerMail(c *gin.Context) {
	var smail ServerMail
	err := c.ShouldBindJSON(&smail)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"err": err.Error()})
		return
	}

	m._master_mod.sendMsg(handle.M_ADD_SERVER_MAIL, &smail)
	c.JSON(http.StatusOK, gin.H{"msg": "ok"})
}

type PlayerMail struct {
	Cid       int    `json:"cid"`
	RoomMgrId int    `json:"roommgrid"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Reward    string `json:"reward"`
}

func (m *MasterHttpModule) onAddPlayerMail(c *gin.Context) {
	var smail PlayerMail
	err := c.ShouldBindJSON(&smail)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"err": err.Error()})
		return
	}

	m._master_mod.sendMsg(handle.M_ADD_PLAYER_MAIL, &smail)
	c.JSON(http.StatusOK, gin.H{"msg": "ok"})
}
