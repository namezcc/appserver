package module

import (
	"context"
	"fmt"
	"goserver/handle"
	"goserver/util"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

type responesData struct {
	index int
	data  interface{}
}

type msgResponse struct {
	_c       chan *responesData
	_endtime int64
	_id      int
}

type httpBase interface {
	initRoter(*gin.Engine)
	getRedisLock() *sync.Mutex
}

type HttpModule struct {
	modulebase
	_msg_response   map[int]msgResponse
	_rsp_index      int
	_rsp_index_lock sync.Mutex
	_net_mod        *netModule
	_sql            *sqlx.DB
	_httpbase       httpBase
	_gin            *gin.Engine
	_host           string
	_http_lock      sync.Mutex
	_mysql_host     string
	_rdb            *redis.Client
	_redis_lock     sync.Mutex
}

func (m *HttpModule) Init(mgr *moduleMgr) {
	m._mod_mgr = mgr
	m._msg_response = make(map[int]msgResponse)
	m._rsp_index = 0
	m._net_mod = mgr.GetModule(MOD_NET).(*netModule)
	m._httpbase = m
	m._host = ":8999"
	m._mysql_host = "mysql"

	handle.Handlemsg.AddMsgCall(handle.N_WBE_ON_RESPONSE, m.onMsgRespones)
	handle.Handlemsg.AddMsgCall(handle.M_ON_RESPONSE, m.onMsgRespones)
}

func (m *HttpModule) HttpLock() {
	m._http_lock.Lock()
}

func (m *HttpModule) HttpUnLock() {
	m._http_lock.Unlock()
}

func (m *HttpModule) SetMysqlHots(h string) {
	m._mysql_host = h
}

func (m *HttpModule) InitSql() {
	host := util.GetConfValue(m._mysql_host)
	sql, sqlerr := sqlx.Open("mysql", host)
	m._sql = sql

	if sqlerr != nil {
		panic("sql open error:" + sqlerr.Error())
	}

	util.Log_info("conn mysql success %s\n", host)
	sql.SetMaxOpenConns(100)
	// 闲置连接数
	sql.SetMaxIdleConns(20)
	// 最大连接周期
	sql.SetConnMaxLifetime(100 * time.Second)
}

func (m *HttpModule) initRedis() {
	rhost := util.GetConfValue("redis-host")
	rpass := util.GetConfValue("redis-pass")
	rdb := util.GetConfInt("redis-db")

	m._rdb = redis.NewClient(&redis.Options{
		Addr:     rhost,
		Password: rpass,
		DB:       rdb,
	})

	doRedis(func(ctx context.Context) {
		_, err := m._rdb.Ping(ctx).Result()
		if err != nil {
			panic(err)
		}
	})
	util.Log_info("redis connect %s\n", rhost)
}

func (m *HttpModule) SetHost(host string) {
	m._host = host
}

func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("origin") //请求头部
		if len(origin) == 0 {
			origin = c.Request.Header.Get("Origin")
		}
		//接收客户端发送的origin （重要！）
		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		//允许客户端传递校验信息比如 cookie (重要)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		//服务器支持的所有跨域请求的方法
		c.Writer.Header().Set("Access-Control-Allow-Methods", "OPTIONS, GET, POST, PUT, DELETE, UPDATE")
		c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		// 设置预验请求有效期为 86400 秒
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func (m *HttpModule) initRoter(r *gin.Engine) {
	r.GET("/", m.apiIndex)
}

func (m *HttpModule) getRedisLock() *sync.Mutex {
	return &m._redis_lock
}

func (m *HttpModule) AfterInit() {
	router := gin.Default()

	router.Use(Cors())
	m._httpbase.initRoter(router)
	m._gin = router
	m.setTikerFunc(time.Second*5, m.checkRespones)
}

func (m *HttpModule) BeforRun() {
	m.InitSql()
	m.runGin()
}

func (m *HttpModule) runGin() {
	go func() {
		util.Log_info("start http server addr:%s\n", m._host)
		m._gin.Run(m._host)
	}()
}

func (m *HttpModule) requestMsg(mod *modulebase, mid int, d interface{}) *responesData {
	mrsp := m.getRespones()

	mod.sendMsg(mid, &responesData{
		index: mrsp._id,
		data:  d,
	})

	getpack := <-mrsp._c
	return getpack
}

func (m *HttpModule) responesMsgData(rep *responesData) {
	m.sendMsg(handle.M_ON_RESPONSE, rep)
}

func (m *HttpModule) getRespones() msgResponse {
	m._rsp_index_lock.Lock()
	defer m._rsp_index_lock.Unlock()
	msg := msgResponse{
		_c:       make(chan *responesData),
		_endtime: util.GetMillisecond() + 500000, //5s
		_id:      m._rsp_index,
	}
	m._msg_response[m._rsp_index] = msg
	m._rsp_index++
	return msg
}

func (m *HttpModule) checkRespones(dt int64) {
	m._rsp_index_lock.Lock()
	defer m._rsp_index_lock.Unlock()

	for k, v := range m._msg_response {
		if dt > v._endtime {
			v._c <- nil
			delete(m._msg_response, k)
		}
	}
}

func (m *HttpModule) onMsgRespones(msg *handle.BaseMsg) {
	pack := msg.Data.(*responesData)
	index := pack.index
	m._rsp_index_lock.Lock()
	rsp, ok := m._msg_response[int(index)]
	if !ok {
		m._rsp_index_lock.Unlock()
		return
	}
	delete(m._msg_response, int(index))
	m._rsp_index_lock.Unlock()

	rsp._c <- pack
}

func (m *HttpModule) apiIndex(c *gin.Context) {
	c.String(http.StatusOK, string("index"))
}

type dbServer struct {
	Type      int         `db:"type" json:"type"`
	Id        int         `db:"id" json:"id"`
	Name      string      `db:"name" json:"name"`
	Group     int         `db:"group" json:"group"`
	Port      int         `db:"port" json:"port"`
	Mysql     string      `db:"mysql" json:"mysql"`
	Redis     string      `db:"redis" json:"redis"`
	Version   string      `db:"version" json:"version"`
	Platform  string      `db:"platform" json:"platform"`
	MysqlHost dbMysqlHost `json:"mysqlhost"`
	Uidgroup  []int       `json:"uidgroup"`
	Dbidlist  []int       `json:"dbidlist"`
}

type dbMysqlHost struct {
	Id     int    `db:"id" json:"id"`
	Ip     string `db:"ip" json:"ip"`
	Port   int    `db:"port" json:"port"`
	Dbname string `db:"dbname" json:"dbname"`
	User   string `db:"user" json:"user"`
	Pass   string `db:"pass" json:"pass"`
	Type   int    `db:"type" json:"type"`
}

type dbServiceFind struct {
	Id       int    `db:"id" json:"id"`
	Ip       string `db:"ip" json:"ip"`
	Port     int    `db:"port" json:"port"`
	Httpport int    `db:"httpport"`
}

type ServiceInfo struct {
	Server  dbServer        `json:"server"`
	Service []dbServiceFind `json:"service"`
}

type dbLogin struct {
	Id       int    `db:"id"`
	Login    int    `db:"login"`
	Uidgroup int    `db:"uidgroup"`
	Showname string `db:"showname"`
}

func (m *dbLogin) encode() string {
	return fmt.Sprintf("%d,%d,%d,%s", m.Id, m.Login, m.Uidgroup, m.Showname)
}

func (m *dbLogin) decode(s string) {
	svec := strings.Split(s, ",")
	if len(svec) != 4 {
		return
	}
	m.Id, _ = strconv.Atoi(svec[0])
	m.Login, _ = strconv.Atoi(svec[1])
	m.Uidgroup, _ = strconv.Atoi(svec[2])
	m.Showname = svec[3]
}

type LoginInfo struct {
	Id       int    `json:"id"`
	Host     string `json:"host"`
	ShowName string `json:"showName"`
	Uidgroup int    `json:"uidgroup"`
}
