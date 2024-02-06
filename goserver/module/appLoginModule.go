package module

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"goserver/handle"
	"goserver/util"
	"hash/crc32"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/qiniu/qmgo"
	qmoption "github.com/qiniu/qmgo/options"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gorm.io/gorm"
)

const (
	COLL_TASK_JOIN        = "task_join"
	COLL_TASK             = "task"
	COLL_USER_TASK        = "user_task" //加入的任务
	COLL_TASK_CHAT        = "task_chat"
	COLL_USER_CREATE_TASK = "user_create_task" //创建的任务
)

type jsonBase struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

type AppJWTClaims struct { // token里面添加用户信息，验证token后可能会用到用户信息
	jwt.StandardClaims
	UserID int64  `json:"user_id"`
	Phone  string `json:"phone"`
	// Username    string   `json:"username"`
	// FullName    string   `json:"full_name"`
	// Permissions []string `json:"permissions"`
}

var (
	AppSecret         = "LiuBei"      // 加盐
	AppExpireTime     = 3600 * 24 * 5 // token有效期
	RedisPhoneCodeKey = "phoneCode"
)

func AppVerify(c *gin.Context) (result bool, userPhone string, userId int64) {
	strToken := c.Request.Header.Get("Authorization")
	if strToken == "" {
		result = false
		return result, "", 0
	}
	claim, err := TokenVerifyAction(strToken)
	if err != nil {
		result = false
		util.Log_error("verify err:%s", err.Error())
		return result, "", 0
	}
	result = true
	userPhone = claim.Phone
	userId = claim.UserID
	return result, userPhone, userId
}

// 刷新token
func TokenRefresh(c *gin.Context) string {
	strToken := c.Request.Header.Get("Authorization")
	claims, err := TokenVerifyAction(strToken)
	if err != nil {
		util.Log_error("token refresh %s", err.Error())
		return ""
	}
	claims.ExpiresAt = time.Now().Unix() + (claims.ExpiresAt - claims.IssuedAt)
	signedToken, err := AppGetToken(claims)
	if err != nil {
		util.Log_error("token refresh %s", err.Error())
		return ""
	}
	return signedToken
}

// 验证token是否存在，存在则获取信息
func TokenVerifyAction(strToken string) (claims *AppJWTClaims, err error) {
	token, err := jwt.ParseWithClaims(strToken, &AppJWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(AppSecret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*AppJWTClaims)
	if !ok {
		return nil, err
	}
	if err := token.Claims.Valid(); err != nil {
		return nil, err
	}
	return claims, nil
}

// 生成token
func AppGetToken(claims *AppJWTClaims) (signedToken string, err error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err = token.SignedString([]byte(AppSecret))
	if err != nil {
		return "", err
	}
	return signedToken, nil
}

type AppLoginModule struct {
	HttpModule
	_safe_req   map[string]bool
	_redis_mgr  *RedisManagerModule
	_sql_mgr    *MysqlManagerModule
	_mg_mgr     *MongoManagerModule
	_server_mod *ClientServerModule

	_create_userid_num  int
	_create_userid_time int64
	_userid_lock        sync.Mutex
}

func (m *AppLoginModule) Init(mgr *moduleMgr) {
	m.HttpModule.Init(mgr)
	m._httpbase = m
	m.SetHost(":" + util.GetConfValue("hostport"))
	m._safe_req = make(map[string]bool)
	m._redis_mgr = mgr.GetModule(MOD_REDIS_MGR).(*RedisManagerModule)
	m._sql_mgr = mgr.GetModule(MOD_MYSQL_MGR).(*MysqlManagerModule)
	m._mg_mgr = mgr.GetModule(MOD_MONGO_MGR).(*MongoManagerModule)
	m._server_mod = mgr.GetModule(MOD_CLIENT_SERVER).(*ClientServerModule)
}

func (m *AppLoginModule) BeforRun() {
	m.HttpModule.BeforRun()

}

func (m *AppLoginModule) getMiddlew() gin.HandlerFunc {
	return func(c *gin.Context) {
		p := c.FullPath()
		_, ok := m._safe_req[p]
		if ok {
			c.Next()
			return
		}

		checkUser, userPhone, userId := AppVerify(c) //第二个值是用户名，这里没有使用
		if checkUser == false {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 0, "msg": "身份认证失败"})
			c.Abort()
			return
		}

		// hashval := util.StringHash(userPhone)
		hashval := uint32(userId)
		c.Set("phone", userPhone)
		c.Set("userId", userId)
		c.Set("phoneHash", hashval)
		c.Next()
	}
}

func (m *AppLoginModule) safeGet(r *gin.Engine, p string, h gin.HandlerFunc) {
	m._safe_req[p] = true
	r.GET(p, h)
}

func (m *AppLoginModule) safePost(r *gin.Engine, p string, h gin.HandlerFunc) {
	m._safe_req[p] = true
	r.POST(p, h)
}

type AppModuleHandle func(*gin.Context, *AppLoginModule)

func bindApiHandle(m *AppLoginModule, h AppModuleHandle) gin.HandlerFunc {
	return func(c *gin.Context) {
		h(c, m)
	}
}

func (m *AppLoginModule) initRoter(r *gin.Engine) {
	path, err := os.Getwd()
	if err != nil {
		// log.Fatal(err)
		panic(err.Error())
	}
	// absPath, _ := filepath.Abs(path) // 转换为绝对路径
	r.Static("/static", path+"/upload")

	r.Use(m.getMiddlew())

	m.safeGet(r, "/phoneCode", m.phoneCode)
	m.safePost(r, "/userlogin", m.userLogin)

	r.GET("/getUserInfo", m.apiGetUserInfo)
	r.GET("/userRefreshToken", m.userRefreshToken)
	r.POST("/apiCreateTask", m.apiCreateTask)
	r.POST("/apiUpdateTask", m.apiUpdateTask)

	m.safePost(r, "/apiGetTaskInfo", m.apiGetTaskInfo)
	m.safeGet(r, "/apiGetOneTaskInfo", m.apiGetOneTaskInfo)

	r.GET("/apiLoadMyTaskInfo", m.apiLoadMyTaskInfo)
	r.GET("/apiLoadMyJoinTaskInfo", m.apiLoadMyJoinTaskInfo)
	r.GET("/apiDeleteMyTaskInfo", m.apiDeleteMyTaskInfo)

	r.POST("/uploadTaskImage", bindApiHandle(m, apiUploadTaskImage))
	m.safePost(r, "/apiUploadOssImage", bindApiHandle(m, apiUploadOssImage))

	r.GET("/apiJoinTask", m.apiJoinTask)
	r.GET("/apiQuitTask", m.apiQuitTask)
	r.GET("/apiKickTask", m.apiKickTask)
	r.GET("/apiDeleteUserJoin", m.apiDeleteUserJoin)

	r.POST("/apiFinishTask", m.apiFinishTask)
	r.GET("/apiGetTaskReward", m.apiGetTaskReward)
	r.GET("/apiPayTaskCost", m.apiPayTaskCost)
	r.GET("/apiGetTaskCost", m.apiGetTaskCost)

	r.GET("/apiEditName", m.apiEditName)
	r.GET("/apiEditSex", m.apiEditSex)
}

const UID_START_TIME = 1675612800

func (m *AppLoginModule) genUserId() int64 {
	m._userid_lock.Lock()
	defer m._userid_lock.Unlock()

	now := util.GetSecond()
	if now > m._create_userid_time {
		m._create_userid_time = now
		m._create_userid_num = 0
	}

	uid := ((now - UID_START_TIME) << 12) | int64(m._create_userid_num)

	m._create_userid_num++
	if m._create_userid_num >= 0xFFF {
		m._create_userid_num = 0
		m._create_userid_time++
	}
	return uid
}

func (m *AppLoginModule) ResponesJsonData(c *gin.Context, d interface{}) {
	jd := jsonBase{
		Data: d,
	}
	c.JSON(http.StatusOK, jd)
}

func (m *AppLoginModule) ResponesJsonBase(c *gin.Context, d jsonBase) {
	c.JSON(http.StatusOK, d)
}

func (m *AppLoginModule) ResponesError(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, gin.H{"code": code, "msg": msg})
}

type phoneCodeInfo struct {
	Code string `json:"code"`
	Time int64  `json:"time"`
}

// 生成验证码
func (m *AppLoginModule) phoneCode(c *gin.Context) {
	pnum := c.Query("phoneNumber")
	mobregex := `^1[3-9]\d{9}$`
	reg := regexp.MustCompile(mobregex)
	ok := reg.MatchString(pnum)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "number error"})
		return
	}

	hashval := crc32.ChecksumIEEE([]byte(pnum))

	codestr := m._redis_mgr.RequestRedisFuncCallHash(hashval, func(ctx context.Context, _rdb *redis.Client) interface{} {
		res, err := _rdb.HGet(ctx, RedisPhoneCodeKey, pnum).Result()
		if err != nil {
			util.Log_error("phonecode %s", err.Error())
		}
		return res
	}).(string)

	codeinfo := phoneCodeInfo{}
	if codestr != "" {
		err := json.Unmarshal([]byte(codestr), &codeinfo)
		if err != nil {
			util.Log_error("phonecode %s", err.Error())
		}
	}

	if codeinfo.Time > util.GetSecond() {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "验证码生成太频繁"})
		return
	}

	code := rand.Intn(899999) + 100000
	codeinfo.Code = strconv.Itoa(code)
	// 60s 过期
	codeinfo.Time = util.GetSecond() + 60
	infostr, err := json.Marshal(codeinfo)
	if err != nil {
		util.Log_error("phonecode json %s", err.Error())
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "json encode error"})
		return
	}

	// 闭包数据可能会被释放 ???
	m._redis_mgr.RedisWorkFuncCallNoBlockHash(hashval, func(ctx context.Context, _rdb *redis.Client) {
		_rdb.HSet(ctx, RedisPhoneCodeKey, pnum, infostr)
	})

	util.Log_info("phonecode phone: %s code:%s", pnum, codeinfo.Code)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
}

type userLoginData struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

func (m *AppLoginModule) userLogin(c *gin.Context) {
	var info userLoginData
	err := c.ShouldBindJSON(&info)

	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": err.Error()})
		return
	}

	codestr := ""
	hashval := crc32.ChecksumIEEE([]byte(info.Phone))
	m._redis_mgr.RequestRedisFuncCallNoResHash(hashval, func(ctx context.Context, _rdb *redis.Client) {
		rs := _rdb.HGet(ctx, RedisPhoneCodeKey, info.Phone)
		codestr, err = rs.Result()
		if err != nil {
			util.Log_error(err.Error())
		}
	})

	if codestr == "" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "not gen code"})
		return
	}

	codeinfo := phoneCodeInfo{}
	err = json.Unmarshal([]byte(codestr), &codeinfo)
	if err != nil {
		util.Log_error("userLogin %s", err.Error())
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "server error"})
		return
	}

	if codeinfo.Time+4*60 < util.GetSecond() {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "code out time"})
		return
	}

	if codeinfo.Code == info.Code {
		// 查询账号
		userId := int64(0)
		m._sql_mgr.RequestFuncCallNoResHash(hashval, func(d *gorm.DB) {
			var buser b_user
			sqlres := d.Select("cid", "phone").Find(&buser, "phone = ?", info.Phone)
			// 创建账号
			if sqlres.Error != nil {
				util.Log_error("select user error %s", sqlres.Error.Error())
				return
			}
			if buser.Cid == 0 {
				buser.Cid = m.genUserId()
				buser.Phone = info.Phone
				buser.Name = info.Phone
				sqlres := d.Create(&buser)
				if sqlres.Error != nil {
					util.Log_error("create user error %s", sqlres.Error.Error())
					return
				}
			}
			userId = buser.Cid
		})

		if userId == 0 {
			c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "account error"})
			return
		}

		claims := &AppJWTClaims{
			UserID: userId,
			Phone:  info.Phone,
		}
		claims.IssuedAt = time.Now().Unix()
		claims.ExpiresAt = time.Now().Add(time.Second * time.Duration(ExpireTime)).Unix()
		signedToken, err := AppGetToken(claims)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 1, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": signedToken})
	} else {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "验证码错误"})
	}
}

func (m *AppLoginModule) userRefreshToken(c *gin.Context) {
	newtoken := TokenRefresh(c)
	if newtoken == "" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "token error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": newtoken})
}

type b_user struct {
	Cid   int64  `gorm:"primaryKey" json:"cid"`
	Phone string `json:"phone"`
	Name  string `json:"name"`
	Sex   int    `json:"sex"`
	Icon  string `json:"icon"`
	Birth string `json:"birth"`
	Money int    `json:"money"`
}

func (m *AppLoginModule) getUserInfo(cid int64, hashval uint32) *b_user {
	var buser b_user
	m._sql_mgr.RequestFuncCallNoResHash(hashval, func(d *gorm.DB) {
		res := d.Find(&buser, "cid = ?", cid)
		if res.Error != nil {
			util.Log_error("getuserinfo cid:%d %s", cid, res.Error.Error())
		}
	})
	if buser.Cid > 0 {
		return &buser
	} else {
		return nil
	}
}

func (m *AppLoginModule) apiGetUserInfo(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	hashval := c.MustGet("phoneHash").(uint32)
	buser := m.getUserInfo(cid, hashval)
	if buser == nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "没有用户信息"})
		return
	}
	m.ResponesJsonData(c, *buser)
}

type mg_location struct {
	Type        string    `json:"type" bson:"type"`
	Coordinates []float64 `json:"coordinates" bson:"coordinates"`
}

// 经度,纬度
func NewLocation(longitude float64, latitude float64) mg_location {
	return mg_location{
		Type:        "Point",
		Coordinates: []float64{longitude, latitude},
	}
}

type addressInfo struct {
	Address   string  `json:"address" bson:"address"`
	Pname     string  `json:"pname,omitempty" bson:"pname,omitempty"`
	Cityname  string  `json:"cityname,omitempty" bson:"cityname,omitempty"`
	Adname    string  `json:"adname,omitempty" bson:"adname,omitempty"`
	Name      string  `json:"name" bson:"name"`
	Location  string  `json:"location" bson:"location"`
	Longitude float64 `json:"longitude" bson:"longitude"`
	Latitude  float64 `json:"latitude" bson:"latitude"`
}

func (m *addressInfo) equal(a addressInfo) bool {
	if m.Address != a.Address || m.Pname != a.Pname || m.Cityname != a.Cityname || m.Adname != a.Adname ||
		m.Name != a.Name || m.Location != a.Location {
		return false
	}
	return true
}

type mg_task struct {
	Id          primitive.ObjectID `json:"id,omitempty" bson:"_id"`
	CreateAt    string             `json:"createAt,omitempty" bson:"createAt"`
	UpdateAt    string             `json:"updateAt,omitempty" bson:"updateAt"`
	Cid         int64              `json:"cid,omitempty" bson:"cid"`
	CreatorName string             `json:"creator_name" bson:"creator_name"`
	CreatorIcon string             `json:"creator_icon,omitempty" bson:"creator_icon,omitempty"`
	Title       string             `json:"title" bson:"title"`
	Content     string             `json:"content" bson:"content"`
	Images      []string           `json:"images,omitempty" bson:"images,omitempty"`
	MoneyType   int                `json:"money_type" bson:"money_type"`
	Money       int                `json:"money" bson:"money"`
	WomanMoney  int                `json:"womanMoney" bson:"womanMoney"`
	PeopleNum   int                `json:"people_num" bson:"people_num"`
	ManNum      int                `json:"man_num" bson:"man_num"`
	EndTime     int64              `json:"end_time" bson:"end_time"`
	Address     *addressInfo       `json:"address,omitempty" bson:"address,omitempty"`
	Delete      int                `json:"delete,omitempty" bson:"delete,omitempty"`
	Join        *mg_task_join      `json:"join,omitempty" bson:"join,omitempty"`
	State       int                `json:"state,omitempty" bson:"state,omitempty"`
}

// 获取空闲人数
func (m *mg_task) getUsableNum(sex int) int {
	if m.ManNum < 0 {
		// 性别无关
		joinnum := 0
		if m.Join != nil {
			joinnum = len(m.Join.Data)
		}
		return m.PeopleNum - joinnum
	} else {
		joinnum := 0
		if m.Join != nil {
			for _, v := range m.Join.Data {
				if v.Sex == sex {
					joinnum++
				}
			}
		}
		if sex == util.SEX_WOMAN {
			return m.PeopleNum - m.ManNum - joinnum
		} else {
			return m.ManNum - joinnum
		}
	}
}

type mg_task_location struct {
	Id       primitive.ObjectID `bson:"_id"`
	Location mg_location        `bson:"location"`
	UpdateAt time.Time          `bson:"updateAt"`
}

type mg_task_globel struct {
	Id       primitive.ObjectID `bson:"_id"`
	UpdateAt time.Time          `bson:"updateAt"`
}

type mg_task_join_info struct {
	Cid    int64  `json:"cid" bson:"cid"`
	Name   string `json:"name" bson:"name"`
	Sex    int    `json:"sex" bson:"sex"`
	Icon   string `json:"icon" bson:"icon"`
	State  int    `json:"state" bson:"state"`
	Money  int    `json:"money" bson:"moneye"`
	NoChat int    `json:"nochat" bson:"nochat"`
}

type mg_task_join struct {
	Id   primitive.ObjectID  `json:"_id" bson:"_id"`
	Cid  int64               `json:"cid" bson:"cid"`
	Data []mg_task_join_info `json:"data" bson:"data"`
}

type My_join_task struct {
	Id    primitive.ObjectID `json:"_id" bson:"_id"`
	Time  int64              `json:"time" bson:"time"`
	State int                `json:"state" bson:"state"`
}

type mg_task_my_join struct {
	Id       primitive.ObjectID `json:"_id" bson:"_id"`
	Cid      int                `json:"cid" bson:"cid"`
	TaskList []My_join_task     `json:"tasklist" bson:"tasklist"`
}

type UserJoinTask My_join_task
type UserTask mg_task_my_join
type UserCreateTask mg_task_my_join

func checkTaskData(task *mg_task) bool {
	// 检查数据
	nowsec := util.GetSecond()
	if task.EndTime <= nowsec {
		util.Log_error("task endtime :%d", task.EndTime)
		return false
	}
	if task.PeopleNum <= 0 {
		util.Log_error("task poeple num :%d", task.PeopleNum)
		return false
	}
	titlenum := util.StringCharLen(task.Title)
	contentnum := util.StringCharLen(task.Content)
	if titlenum <= 0 || titlenum > 20 || contentnum > 500 {
		util.Log_error("task title:%d content:%d", titlenum, contentnum)
		return false
	}
	return true
}

func (m *AppLoginModule) apiCreateTask(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	hashval := c.MustGet("phoneHash").(uint32)
	var task mg_task
	err := c.ShouldBindJSON(&task)
	if err != nil {
		util.Log_error("get json error:%s", err.Error())
		m.ResponesError(c, 1, "输入数据错误")
		return
	}
	// 检查数据
	nowsec := util.GetSecond()
	if !checkTaskData(&task) {
		m.ResponesError(c, 1, "输入数据错误")
		return
	}
	// ...

	task.Cid = cid
	task.Id = qmgo.NewObjectID()
	nowtimestr := util.NowTime()
	task.CreateAt = nowtimestr
	task.UpdateAt = nowtimestr

	// 过期时间
	expireTime := task.EndTime - nowsec

	// 插入数据库
	ires := m._mg_mgr.RequestFuncCallHash(hashval, func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		// 事务
		_, err := qc.DoTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
			if task.Address != nil {
				// task.Location = &loc
				taskloc := mg_task_location{
					Id:       task.Id,
					Location: NewLocation(task.Address.Longitude, task.Address.Latitude),
					UpdateAt: time.Now().Add(time.Second * time.Duration(expireTime)),
				}

				coll := qc.Database.Collection("task_location")
				_, err := coll.InsertOne(sessCtx, taskloc)
				if err != nil {
					util.Log_error("insert task_location err:%s", err.Error())
					return nil, err
				}
			} else {
				taskglo := mg_task_globel{
					Id:       task.Id,
					UpdateAt: time.Now().Add(time.Second * time.Duration(expireTime)),
				}
				coll := qc.Database.Collection("task_globel")
				_, err := coll.InsertOne(sessCtx, taskglo)
				if err != nil {
					util.Log_error("insert task_globel err:%s", err.Error())
					return nil, err
				}
			}

			coll := qc.Database.Collection("task")
			res, err := coll.InsertOne(sessCtx, task)
			if err != nil {
				util.Log_error("insert task err:%s", err.Error())
				return nil, err
			}
			// user_create_task
			userColl := qc.Database.Collection(COLL_USER_CREATE_TASK)
			upopts := options.Update().SetUpsert(true)
			qmopt := qmoption.UpdateOptions{UpdateHook: nil, UpdateOptions: upopts}
			usercreate := My_join_task{
				Id:   task.Id,
				Time: nowsec,
			}
			err = userColl.UpdateOne(sessCtx, bson.M{"cid": cid}, bson.M{"$push": bson.M{"tasklist": usercreate}}, qmopt)
			if err != nil {
				util.Log_error("user create task err:%s", err.Error())
				return nil, err
			}
			util.Log_info("insert task success:%s", res.InsertedID)
			return nil, nil
		})
		return err == nil
	}).(bool)

	if ires {
		m.ResponesJsonData(c, task)
	} else {
		m.ResponesError(c, 1, "创建task错误")
	}
}

func taskIdToObjectId(id string) *primitive.ObjectID {
	objid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		util.Log_error("taskid err: %s", err.Error())
		return nil
	}
	return &objid
}

func (m *AppLoginModule) GetTaskInfo(taskid string) *mg_task {
	objid := taskIdToObjectId(taskid)
	if objid == nil {
		return nil
	}
	return m.getTaskInfoObjId(*objid, util.StringHash(taskid))
}

func getTaskInfoByQmgo(qc *qmgo.QmgoClient, ctx context.Context, objid primitive.ObjectID) *mg_task {
	var task mg_task
	coll := qc.Database.Collection("task")
	err := coll.Find(ctx, bson.M{"_id": objid}).One(&task)
	if err != nil {
		util.Log_error("gettask err: %s", err.Error())
		return nil
	}
	return &task
}

func getTaskByPipline(qc *qmgo.QmgoClient, ctx context.Context, pipline interface{}) (*mg_task, error) {
	var task mg_task
	coll := qc.Database.Collection("task")
	err := coll.Aggregate(ctx, pipline).One(&task)
	if err != nil {
		util.Log_error("gettask pipline err: %s", err.Error())
		return nil, err
	}
	return &task, nil
}

func (m *AppLoginModule) getTaskInfoObjId(objid primitive.ObjectID, hashval uint32) *mg_task {
	tres := m._mg_mgr.RequestFuncCallHash(hashval, func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		return getTaskInfoByQmgo(qc, ctx, objid)
	}).(*mg_task)
	return tres
}

// 修改任务
func (m *AppLoginModule) apiUpdateTask(c *gin.Context) {
	var task mg_task
	err := c.ShouldBindJSON(&task)
	if err != nil {
		util.Log_error("get json error:%s", err.Error())
		m.ResponesError(c, 1, "输入数据错误")
		return
	}
	// 检查数据
	if !checkTaskData(&task) {
		m.ResponesError(c, 1, "输入数据错误")
		return
	}
	// ...
	task.UpdateAt = util.NowTime()
	// 过期时间
	expireTime := task.EndTime - util.GetSecond()
	exptime := time.Now().Add(time.Second * time.Duration(expireTime))
	taskhash := util.StringHash(task.Id.String())
	// 更新
	upopts := options.Update().SetUpsert(true)
	qmopt := qmoption.UpdateOptions{UpdateHook: nil, UpdateOptions: upopts}
	tres := m._mg_mgr.RequestFuncCallHash(taskhash, func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		oldtask := getTaskInfoByQmgo(qc, ctx, task.Id)
		if oldtask == nil || oldtask.State != 0 {
			// 已完成不能更新
			return false
		}
		task.Delete = oldtask.Delete
		_, err := qc.DoTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
			loc_coll := qc.Database.Collection("task_location")
			glo_coll := qc.Database.Collection("task_globel")
			if oldtask.Address != nil {
				if task.Address == nil {
					// 删除 local 报错不管
					loc_coll.RemoveId(sessCtx, task.Id)
					// 插入 globel
					taskglo := mg_task_globel{
						Id:       task.Id,
						UpdateAt: exptime,
					}
					_, err = glo_coll.InsertOne(sessCtx, taskglo)
					if err != nil {
						util.Log_error("update insert task_globel err:%s", err.Error())
						return nil, err
					}

				} else {
					// 更新 local
					loc := NewLocation(task.Address.Longitude, task.Address.Latitude)
					// taskloc := mg_task_location{
					// 	Id:       task.Id,
					// 	Location: loc,
					// 	UpdateAt: exptime,
					// }
					// err := loc_coll.ReplaceOne(sessCtx, bson.M{"_id": task.Id}, taskloc)
					err := loc_coll.UpdateOne(sessCtx, bson.M{"_id": task.Id}, bson.M{"$set": bson.M{"location": loc, "updateAt": exptime}}, qmopt)
					if err != nil {
						// 更新失败改插入
						// _, err = loc_coll.InsertOne(sessCtx, taskloc)
						// if err != nil {
						util.Log_error("update task 1 err:%s", err.Error())
						// 	return nil, err
						// }
						return nil, err
					}
				}
			} else if task.Address != nil {
				// 插入 local
				taskloc := mg_task_location{
					Id:       task.Id,
					Location: NewLocation(task.Address.Longitude, task.Address.Latitude),
					UpdateAt: exptime,
				}
				_, err := loc_coll.InsertOne(sessCtx, taskloc)
				if err != nil {
					util.Log_error("update task 3 err:%s", err.Error())
					return nil, err
				}
				// 删除globel 删除失败不管
				glo_coll.RemoveId(sessCtx, task.Id)
			} else {
				// globel
				// taskglo := mg_task_globel{
				// 	Id:       task.Id,
				// 	UpdateAt: exptime,
				// }
				err = glo_coll.UpdateOne(sessCtx, bson.M{"_id": task.Id}, bson.M{"$set": bson.M{"updateAt": exptime}}, qmopt)
				if err != nil {
					util.Log_error("replace task_globel err:%s", err.Error())
					return nil, err
				}
			}

			// 更新 task
			coll := qc.Database.Collection("task")
			err := coll.ReplaceOne(sessCtx, bson.M{"_id": task.Id}, task)
			if err != nil {
				util.Log_error("update task 5 err:%s", err.Error())
				return nil, err
			}
			return nil, nil
		})
		return err == nil
	}).(bool)
	if tres {
		m.ResponesJsonData(c, task)
	} else {
		m.ResponesError(c, 1, "更新失败")
	}
}

type taskGetConfig struct {
	GlobelLimit int     `json:"globel_limit"`
	Longitude   float64 `json:"longitude"`
	Latitude    float64 `json:"latitude"`
	MinDistance int     `json:"min_distance"`
	Loc_limit   int     `json:"loc_limit"`
	GlobelMax   int     `json:"globelMax"`
	LocMax      int     `json:"locMax"`
}

type taskResult struct {
	Config *taskGetConfig `json:"config"`
	Data   []mg_task      `json:"data"`
}

func (m *AppLoginModule) apiGetTaskInfo(c *gin.Context) {
	var taskconf taskGetConfig
	err := c.ShouldBindJSON(&taskconf)
	if err != nil {
		util.Log_error(err.Error())
		m.ResponesError(c, 1, "数据错误")
		return
	}

	taskResult := taskResult{
		Config: &taskconf,
		Data:   []mg_task{},
	}
	if taskconf.LocMax > 0 && taskconf.GlobelMax > 0 {
		m.ResponesJsonData(c, taskResult)
		return
	}

	neednum := 20
	// 区域查找
	if taskconf.LocMax == 0 {
		location := NewLocation(taskconf.Longitude, taskconf.Latitude)
		pipline := bson.A{
			bson.D{
				{Key: "$geoNear",
					Value: bson.D{
						{Key: "near", Value: location},
						{Key: "distanceField", Value: "distance"},
						{Key: "minDistance", Value: taskconf.MinDistance},
					},
				},
			},
			bson.D{{Key: "$skip", Value: taskconf.Loc_limit}},
			bson.D{{Key: "$limit", Value: neednum}},
			bson.D{
				{Key: "$lookup",
					Value: bson.D{
						{Key: "from", Value: "task"},
						{Key: "localField", Value: "_id"},
						{Key: "foreignField", Value: "_id"},
						{Key: "as", Value: "result"},
					},
				},
			},
			bson.D{{Key: "$match", Value: bson.D{{Key: "result", Value: bson.D{{Key: "$ne", Value: bson.A{}}}}}}},
			bson.D{
				{Key: "$replaceRoot",
					Value: bson.D{
						{Key: "newRoot",
							Value: bson.D{
								{Key: "$arrayElemAt",
									Value: bson.A{
										"$result",
										0,
									},
								},
							},
						},
					},
				},
			},
			bson.M{"$lookup": bson.D{
				{Key: "from", Value: "task_join"},
				{Key: "localField", Value: "_id"},
				{Key: "foreignField", Value: "_id"},
				{Key: "as", Value: "join"},
			},
			},
			bson.M{"$addFields": bson.M{"join": bson.M{"$arrayElemAt": bson.A{"$join", 0}}}},
		}
		res := m._mg_mgr.RequestFuncCall(func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
			coll := qc.Database.Collection("task_location")
			err := coll.Aggregate(ctx, pipline).All(&taskResult.Data)
			if err != nil {
				util.Log_error("serch task err:%s", err.Error())
				return false
			}
			return true
		}).(bool)
		if res == false {
			m.ResponesError(c, 1, "查找失败task")
			return
		}
		getlen := len(taskResult.Data)
		taskconf.Loc_limit += getlen
		if getlen < neednum {
			taskconf.LocMax = 1
		}
	}

	if taskconf.GlobelMax == 0 {
		pipline2 := bson.A{
			bson.M{"$sort": bson.M{"_id": -1}},
			bson.M{"$skip": taskconf.GlobelLimit},
			bson.M{"$limit": neednum},
			bson.M{"$lookup": bson.D{
				{Key: "from", Value: "task"},
				{Key: "localField", Value: "_id"},
				{Key: "foreignField", Value: "_id"},
				{Key: "as", Value: "result"},
			},
			},
			bson.M{"$match": bson.M{"result": bson.D{{Key: "$ne", Value: bson.A{}}}}},
			bson.M{"$replaceRoot": bson.M{"newRoot": bson.M{"$arrayElemAt": bson.A{
				"$result",
				0,
			},
			},
			},
			},
			bson.M{"$lookup": bson.D{
				{Key: "from", Value: "task_join"},
				{Key: "localField", Value: "_id"},
				{Key: "foreignField", Value: "_id"},
				{Key: "as", Value: "join"},
			},
			},
			bson.M{"$addFields": bson.M{"join": bson.M{"$arrayElemAt": bson.A{"$join", 0}}}},
		}

		var globeldata []mg_task
		res := m._mg_mgr.RequestFuncCall(func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
			coll := qc.Database.Collection("task_globel")
			// err := coll.Find(ctx, bson.M{}).Sort("-_id").Skip(int64(taskconf.GlobelLimit)).Limit(20).All(&taskResult.Data)
			err := coll.Aggregate(ctx, pipline2).All(&globeldata)
			if err != nil {
				util.Log_error("serch task_globel err:%s", err.Error())
				return false
			}
			return true
		}).(bool)

		if res == false {
			m.ResponesError(c, 1, "查找失败task_globel")
			return
		}

		getlen := len(globeldata)
		if getlen > 0 {
			taskResult.Data = append(taskResult.Data, globeldata...)
			taskconf.GlobelLimit += getlen
		}
		if getlen < neednum {
			taskconf.GlobelMax = 1
		}
	}
	m.ResponesJsonData(c, taskResult)
}

func (m *AppLoginModule) apiGetOneTaskInfo(c *gin.Context) {
	taskid := c.DefaultQuery("taskid", "")
	if taskid == "" {
		m.ResponesError(c, 1, "数据错误")
		return
	}
	objid := taskIdToObjectId(taskid)
	if objid == nil {
		m.ResponesError(c, 1, "数据错误")
		return
	}
	pipline := getTaskWithJoinPipline(*objid)
	hash := util.StringHash(taskid)
	res := m._mg_mgr.RequestFuncCallHash(hash, func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		task, err := getTaskByPipline(qc, ctx, pipline)
		if err != nil {
			return nil
		}
		return task
	})
	if res == nil {
		m.ResponesError(c, 1, "数据错误")
	} else {
		m.ResponesJsonData(c, res.(*mg_task))
	}
}

func (m *AppLoginModule) apiLoadMyTaskInfo(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	skip := util.StringToInt(c.DefaultQuery("skip", "0"))
	neednum := 20
	pipline := bson.A{
		bson.M{"$match": bson.M{"cid": cid}},
		bson.M{"$project": bson.M{"tasklist": bson.M{"$slice": bson.A{
			"$tasklist",
			-skip - neednum,
			neednum,
		},
		},
		},
		},
		bson.M{"$lookup": bson.D{
			{Key: "from", Value: "task"},
			{Key: "localField", Value: "tasklist._id"},
			{Key: "foreignField", Value: "_id"},
			{Key: "as", Value: "result"},
		},
		},
		bson.M{"$unwind": "$result"},
		bson.M{"$replaceRoot": bson.M{"newRoot": "$result"}},
		bson.M{"$lookup": bson.D{
			{Key: "from", Value: "task_join"},
			{Key: "localField", Value: "_id"},
			{Key: "foreignField", Value: "_id"},
			{Key: "as", Value: "join"},
		},
		},
		bson.M{"$addFields": bson.M{"join": bson.M{"$arrayElemAt": bson.A{"$join", 0}}}},
	}

	var tasks []mg_task

	m._mg_mgr.RequestFuncCall(func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		coll := qc.Database.Collection(COLL_USER_CREATE_TASK)
		err := coll.Aggregate(ctx, pipline).All(&tasks)
		if err != nil {
			util.Log_error("load task err:%s", err.Error())
			return false
		}
		return true
	})
	m.ResponesJsonData(c, tasks)
}

func (m *AppLoginModule) apiLoadMyJoinTaskInfo(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	skip := util.StringToInt(c.DefaultQuery("skip", "0"))
	neednum := 20
	pipline := bson.A{
		bson.M{"$match": bson.M{"cid": cid}},
		bson.M{"$project": bson.M{"tasklist": bson.M{"$slice": bson.A{
			"$tasklist",
			-skip - neednum,
			neednum,
		},
		},
		},
		},
		bson.M{"$lookup": bson.D{
			{Key: "from", Value: "task"},
			{Key: "localField", Value: "tasklist._id"},
			{Key: "foreignField", Value: "_id"},
			{Key: "as", Value: "result"},
		},
		},
		bson.M{"$unwind": "$result"},
		bson.M{"$replaceRoot": bson.M{"newRoot": "$result"}},
		bson.M{"$lookup": bson.D{
			{Key: "from", Value: "task_join"},
			{Key: "localField", Value: "_id"},
			{Key: "foreignField", Value: "_id"},
			{Key: "as", Value: "join"},
		},
		},
		bson.M{"$addFields": bson.M{"join": bson.M{"$arrayElemAt": bson.A{"$join", 0}}}},
	}

	var tasklist []mg_task
	m._mg_mgr.RequestFuncCall(func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		coll := qc.Database.Collection(COLL_USER_TASK)
		err := coll.Aggregate(ctx, pipline).All(&tasklist)
		if err != nil {
			util.Log_error("load user join task err:%s", err.Error())
			return false
		}
		return true
	})

	m.ResponesJsonData(c, tasklist)
}

func (m *AppLoginModule) apiDeleteMyTaskInfo(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	taskid := c.DefaultQuery("taskid", "")
	if taskid == "" {
		m.ResponesError(c, 1, "数据错误")
		return
	}

	objid := taskIdToObjectId(taskid)
	if objid == nil {
		m.ResponesError(c, 1, "任务id错误")
		return
	}
	taskhash := util.StringHash(taskid)
	res := m._mg_mgr.RequestFuncCallHash(taskhash, func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		_, err := qc.DoTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
			coll := qc.Database.Collection("task")
			err := coll.UpdateOne(sessCtx, bson.M{"cid": cid, "_id": objid}, bson.M{"$set": bson.M{"delete": 1}})
			if err != nil {
				util.Log_error("delete task err:%s", err.Error())
				return nil, err
			}
			loc_coll := qc.Database.Collection("task_location")
			glo_coll := qc.Database.Collection("task_globel")
			// 删除 location
			loc_coll.RemoveId(sessCtx, objid)
			// 删除 globel
			glo_coll.RemoveId(sessCtx, objid)
			return nil, nil
		})
		if err == nil {
			// 删除 user create
			createColl := qc.Database.Collection(COLL_USER_CREATE_TASK)
			createColl.UpdateOne(ctx, bson.M{"cid": cid}, bson.M{"$pull": bson.M{"tasklist": bson.M{"_id": objid}}})
			// user joinn
			userjoin := qc.Database.Collection(COLL_USER_TASK)
			userjoin.UpdateOne(ctx, bson.M{"cid": cid}, bson.M{"$pull": bson.M{"tasklist": bson.M{"_id": objid}}})
			// task chat
			chatcoll := qc.Database.Collection(COLL_TASK_CHAT)
			chatcoll.UpdateOne(ctx, bson.M{"_id": objid}, bson.M{"$set": bson.M{"delete": 1}})
		}
		return err == nil
	}).(bool)
	if res {
		m.ResponesJsonData(c, nil)
		// 通知client server
		m._server_mod.sendMsg(handle.M_ON_TASK_DELETE, taskid)
	} else {
		m.ResponesError(c, 1, "删除失败")
	}
}

// 报名任务
func (m *AppLoginModule) apiJoinTask(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	hashval := c.MustGet("phoneHash").(uint32)
	taskid := c.DefaultQuery("taskid", "")
	if taskid == "" {
		m.ResponesError(c, 1, "数据错误")
		return
	}
	user := m.getUserInfo(cid, hashval)
	if user == nil {
		m.ResponesError(c, 1, "数据错误")
		util.Log_waring("join task user nil cid:%d", cid)
		return
	}

	taskhash := util.StringHash(taskid)
	objid := taskIdToObjectId(taskid)
	if objid == nil {
		m.ResponesError(c, 1, "数据错误")
		return
	}

	pipline := bson.A{
		bson.M{"$match": bson.M{"_id": objid, "delete": bson.M{"$ne": 1}}},
		bson.M{"$lookup": bson.D{
			{Key: "from", Value: "task_join"},
			{Key: "localField", Value: "_id"},
			{Key: "foreignField", Value: "_id"},
			{Key: "as", Value: "join"},
		},
		},
		bson.M{"$addFields": bson.M{"join": bson.M{"$arrayElemAt": bson.A{"$join", 0}}}},
	}

	var sendres gin.H
	var join *mg_task_join
	// 写入
	errcode := m._mg_mgr.RequestFuncCallHash(taskhash, func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		task, _ := getTaskByPipline(qc, ctx, pipline)
		if task == nil {
			util.Log_waring("join task nil taskid:%s", taskid)
			return util.ERRCODE_ERROR
		}
		if task.State != 0 {
			// 已完成不能加入
			util.Log_waring("join task finished taskid:%s", taskid)
			return util.ERRCODE_ERROR
		}
		// 是否已报名
		if task.Join != nil {
			findex := util.VectorFind[mg_task_join_info](task.Join.Data, func(m *mg_task_join_info) bool {
				return m.Cid == cid
			})
			if findex >= 0 {
				return util.ERRCODE_TASK_HAVE_JOIN
			}
		}
		// 任务以取消
		// if task.Delete > 0 {
		// 	return util.ERRCODE_TASK_DELETE
		// }

		nowsec := util.GetSecond()
		// 报名时间已过
		if task.EndTime != 0 && nowsec > task.EndTime {
			return util.ERRCODE_TASK_OVER_ENDTIME
		}

		// 检查人数
		if task.getUsableNum(user.Sex) <= 0 {
			return util.ERRCODE_PEOPLE_FULL
		}
		_, err := qc.DoTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
			// task_join
			joincoll := qc.Database.Collection(COLL_TASK_JOIN)
			joininfo := mg_task_join_info{
				Cid:  user.Cid,
				Name: user.Name,
				Sex:  user.Sex,
				Icon: user.Icon,
			}
			upopts := options.Update().SetUpsert(true)
			qmopt := qmoption.UpdateOptions{UpdateHook: nil, UpdateOptions: upopts}
			err := joincoll.UpdateOne(sessCtx, bson.M{"_id": task.Id}, bson.M{"$push": bson.M{"data": joininfo}, "$setOnInsert": bson.M{"cid": task.Cid}}, qmopt)
			if err != nil {
				util.Log_error("task join err:%s", err.Error())
				return nil, err
			}
			// user task
			usercoll := qc.Database.Collection(COLL_USER_TASK)
			userjoin := My_join_task{
				Id:    task.Id,
				Time:  nowsec,
				State: 0,
			}
			err = usercoll.UpdateOne(sessCtx, bson.M{"cid": cid}, bson.M{"$push": bson.M{"tasklist": userjoin}}, qmopt)
			if err != nil {
				util.Log_error("user join err:%s", err.Error())
				return nil, err
			}
			if task.Join == nil {
				task.Join = &mg_task_join{
					Data: []mg_task_join_info{joininfo},
					Cid:  task.Cid,
				}
			} else {
				task.Join.Data = append(task.Join.Data, joininfo)
			}
			return nil, nil
		})
		if err == nil {
			join = task.Join
			sendres = gin.H{"join": task.Join}
			return util.ERRCODE_SUCCESS
		} else {
			return util.ERRCODE_ERROR
		}
	}).(int)
	if errcode == util.ERRCODE_SUCCESS {
		m.ResponesJsonData(c, sendres)
		m.updateServerTaskJoin(join)
	} else {
		m.ResponesError(c, 1, "报名失败")
	}
}

// 退出任务
func (m *AppLoginModule) apiQuitTask(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	hashval := c.MustGet("phoneHash").(uint32)
	taskid := c.DefaultQuery("taskid", "")
	if taskid == "" {
		m.ResponesError(c, 1, "数据错误")
		return
	}
	user := m.getUserInfo(cid, hashval)
	if user == nil {
		m.ResponesError(c, 1, "数据错误")
		util.Log_waring("join task user nil cid:%d", cid)
		return
	}

	objid := taskIdToObjectId(taskid)
	if objid == nil {
		m.ResponesError(c, 1, "数据错误")
		return
	}

	pipline := bson.A{
		bson.M{"$match": bson.M{"_id": objid}},
		bson.M{"$lookup": bson.D{
			{Key: "from", Value: "task_join"},
			{Key: "localField", Value: "_id"},
			{Key: "foreignField", Value: "_id"},
			{Key: "as", Value: "join"},
		},
		},
		bson.M{"$addFields": bson.M{"join": bson.M{"$arrayElemAt": bson.A{"$join", 0}}}},
		bson.M{"$project": bson.D{
			{Key: "join", Value: 1},
		},
		},
	}
	taskhash := util.StringHash(taskid)
	var sendres gin.H
	var join *mg_task_join
	res := m._mg_mgr.RequestFuncCallHash(taskhash, func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		_, err := qc.DoTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
			task, err := getTaskByPipline(qc, sessCtx, pipline)
			if task == nil {
				return nil, err
			}
			// 有没有
			if task.Join == nil {
				return nil, errors.New("no join")
			}
			oldlen := len(task.Join.Data)
			// 已完成不能退出
			task.Join.Data = util.VectorRemoveNoSort[mg_task_join_info](task.Join.Data, func(m *mg_task_join_info) bool {
				return m.Cid == cid && m.State == FINISH_NONE
			})
			if oldlen == len(task.Join.Data) {
				return nil, errors.New("no join")
			}
			// task_join
			joincoll := qc.Database.Collection(COLL_TASK_JOIN)
			err = joincoll.UpdateOne(sessCtx, bson.M{"_id": objid}, bson.M{"$pull": bson.M{"data": bson.M{"cid": cid, "state": FINISH_NONE}}})
			if err != nil {
				util.Log_error("quit task join err:%s", err.Error())
				return nil, err
			}
			// user task
			usercoll := qc.Database.Collection(COLL_USER_TASK)
			err = usercoll.UpdateOne(sessCtx, bson.M{"cid": cid}, bson.M{"$pull": bson.M{"tasklist": bson.M{"_id": objid}}})
			if err != nil {
				util.Log_error("quit user task err:%s", err.Error())
				return nil, err
			}
			join = task.Join
			sendres = gin.H{"join": task.Join}
			return nil, nil
		})
		return err == nil
	}).(bool)

	if res {
		m.ResponesJsonData(c, sendres)
		m.updateServerTaskJoin(join)
	} else {
		m.ResponesError(c, util.ERRCODE_TASK_QUIT_ERROR, "退出失败")
	}
}

// 踢人
func (m *AppLoginModule) apiKickTask(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	taskid := c.DefaultQuery("taskid", "")
	kickcid := util.StringToInt64(c.DefaultQuery("kickcid", "0"))
	if taskid == "" {
		m.ResponesError(c, 1, "数据错误")
		return
	}
	objid := taskIdToObjectId(taskid)
	if objid == nil {
		m.ResponesError(c, 1, "数据错误")
		return
	}

	pipline := bson.A{
		bson.M{"$match": bson.M{"_id": objid, "cid": cid, "delete": bson.M{"$ne": 1}}},
		bson.M{"$lookup": bson.D{
			{Key: "from", Value: "task_join"},
			{Key: "localField", Value: "_id"},
			{Key: "foreignField", Value: "_id"},
			{Key: "as", Value: "join"},
		},
		},
		bson.M{"$addFields": bson.M{"join": bson.M{"$arrayElemAt": bson.A{"$join", 0}}}},
		bson.M{"$project": bson.D{
			{Key: "join", Value: 1},
		},
		},
	}
	taskhash := util.StringHash(taskid)
	var sendres gin.H
	var join *mg_task_join
	res := m._mg_mgr.RequestFuncCallHash(taskhash, func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		_, err := qc.DoTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
			task, err := getTaskByPipline(qc, sessCtx, pipline)
			if task == nil {
				return nil, err
			}
			// 有没有
			if task.Join == nil {
				return nil, errors.New("no join")
			}
			oldlen := len(task.Join.Data)
			// 已完成不能踢出
			task.Join.Data = util.VectorRemoveNoSort[mg_task_join_info](task.Join.Data, func(m *mg_task_join_info) bool {
				return m.Cid == kickcid && m.State == FINISH_NONE
			})
			if oldlen == len(task.Join.Data) {
				return nil, errors.New("no join")
			}
			// task_join
			joincoll := qc.Database.Collection(COLL_TASK_JOIN)
			err = joincoll.UpdateOne(sessCtx, bson.M{"_id": objid}, bson.M{"$pull": bson.M{"data": bson.M{"cid": kickcid}}})
			if err != nil {
				util.Log_error("kick task join err:%s", err.Error())
				return nil, err
			}
			// user task join
			// 被踢
			usercoll := qc.Database.Collection(COLL_USER_TASK)
			err = usercoll.UpdateOne(sessCtx, bson.M{"cid": kickcid}, bson.M{"$pull": bson.M{"tasklist": bson.M{"_id": objid}}})
			if err != nil {
				util.Log_error("quit user task err:%s", err.Error())
				return nil, err
			}
			join = task.Join
			sendres = gin.H{"join": task.Join}
			return nil, nil
		})
		return err == nil
	}).(bool)
	if res {
		m.ResponesJsonData(c, sendres)
		m.updateServerTaskJoin(join)
	} else {
		m.ResponesError(c, util.ERRCODE_ERROR, "踢人失败")
	}
}

func (m *AppLoginModule) updateServerTaskJoin(join *mg_task_join) {
	if join == nil {
		return
	}
	m._server_mod.sendMsg(handle.M_ON_TASK_JOIN_UPDATE, join)
}

// 删除并不在接收消息
func (m *AppLoginModule) apiDeleteUserJoin(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	taskid := c.DefaultQuery("taskid", "")
	if taskid == "" {
		m.ResponesError(c, 1, "数据错误")
		return
	}
	objid := taskIdToObjectId(taskid)
	if objid == nil {
		m.ResponesError(c, 1, "数据错误")
		return
	}
	taskhash := util.StringHash(taskid)
	m._mg_mgr.RequestFuncCallHash(taskhash, func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		// user task join
		coll := qc.Database.Collection(COLL_USER_TASK)
		err := coll.UpdateOne(ctx, bson.M{"cid": cid}, bson.M{"$pull": bson.M{"tasklist": bson.M{"_id": objid}}})
		if err != nil {
			util.Log_error("apiDeleteUserTaskJoin 1 %s", err.Error())
		}
		//task join
		joincoll := qc.Database.Collection(COLL_TASK_JOIN)
		err = joincoll.UpdateOne(ctx, bson.M{"_id": objid, "data.cid": cid}, bson.M{"$set": bson.M{"data.$.nochat": 1}})
		if err != nil {
			util.Log_error("apiDeleteUserTaskJoin 2 %s", err.Error())
		}
		return nil
	})
	// 通知 client server
	m._server_mod.sendMsg(handle.M_ON_TASK_NO_CHAT, &mg_task_join{
		Id:  *objid,
		Cid: cid,
	})
	m.ResponesJsonData(c, nil)
}

type TaskFinish struct {
	Id  primitive.ObjectID `json:"id"`
	Pos []int              `json:"pos"`
	Cid []int64            `json:"cid"`
}

func getTaskWithJoinPipline(objid primitive.ObjectID) bson.A {
	pipline := bson.A{
		bson.M{"$match": bson.M{"_id": objid}},
		bson.M{"$lookup": bson.D{
			{Key: "from", Value: "task_join"},
			{Key: "localField", Value: "_id"},
			{Key: "foreignField", Value: "_id"},
			{Key: "as", Value: "join"},
		},
		},
		bson.M{"$addFields": bson.M{"join": bson.M{"$arrayElemAt": bson.A{"$join", 0}}}},
	}
	return pipline
}

const (
	FINISH_NONE       = 0
	FINISH_HAVE_MONEY = 1
	FINISH_GET_MONEY  = 2
	FINISH_DONE       = 3
)

// 完成任务
func (m *AppLoginModule) apiFinishTask(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	var finish TaskFinish
	err := c.ShouldBindJSON(&finish)
	if err != nil {
		util.Log_error(err.Error())
		m.ResponesError(c, 1, "数据错误")
		return
	}
	lencid := len(finish.Cid)
	if lencid == 0 ||
		len(finish.Pos) != lencid ||
		util.VectorFind[int64](finish.Cid, func(i *int64) bool { return cid == *i }) >= 0 {
		m.ResponesError(c, 1, "数据错误")
		return
	}

	hash := util.StringHash(finish.Id.Hex())
	pipline := getTaskWithJoinPipline(finish.Id)
	var tjoin *mg_task_join
	money := -1
	errcode := m._mg_mgr.RequestFuncCallHash(hash, func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		task, err := getTaskByPipline(qc, ctx, pipline)
		if err != nil {
			return util.ERRCODE_ERROR
		}
		if task.Cid != cid {
			util.Log_error("finish task not master m:%d %d", task.Cid, cid)
			return util.ERRCODE_ERROR
		}
		// 检查数据
		lenjoin := len(task.Join.Data)
		if lencid > lenjoin {
			return util.ERRCODE_ERROR
		}
		paymoney := 0
		for i := 0; i < lencid; i++ {
			jidx := finish.Pos[i]
			if jidx < 0 || jidx >= lenjoin {
				return util.ERRCODE_ERROR
			}
			join := &task.Join.Data[i]
			if join.State != FINISH_NONE {
				return util.ERRCODE_ERROR
			}
			if task.Money > 0 && task.MoneyType == util.MONEY_REWARD {
				paymoney += task.Money
				join.Money = task.Money
				join.State = FINISH_HAVE_MONEY
			} else {
				join.State = FINISH_DONE
			}
		}
		// 更新
		if task.State == 0 {
			coll := qc.Database.Collection(COLL_TASK)
			err := coll.UpdateOne(ctx, bson.M{"_id": task.Id}, bson.M{"$set": bson.M{"state": 1}})
			if err != nil {
				util.Log_error("update task state %s", err.Error())
				return util.ERRCODE_ERROR
			}
		}
		_, err = qc.DoTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
			coll := qc.Database.Collection(COLL_TASK_JOIN)
			err := coll.UpdateOne(sessCtx, bson.M{"_id": task.Id}, bson.M{"$set": bson.M{"data": task.Join.Data}})
			if err != nil {
				return nil, err
			}

			if paymoney > 0 {
				// 扣钱
				res := m._sql_mgr.RequestFuncCallHash(uint32(cid), func(d *gorm.DB) interface{} {
					var buser b_user
					r := d.Select("cid", "money").Find(&buser)
					if r.Error != nil {
						return r.Error
					}
					if buser.Money < paymoney {
						return errors.New("money not enouth")
					}
					money = buser.Money - paymoney
					// 更新
					r = d.Model(&buser).Update("money", money)
					return r.Error
				})
				if res != nil {
					return nil, res.(error)
				}
			}
			return nil, nil
		})
		if err != nil {
			util.Log_error("finish task err %s", err.Error())
			return util.ERRCODE_ERROR
		}
		tjoin = task.Join
		return util.ERRCODE_SUCCESS
	}).(int)
	if tjoin != nil {
		m.ResponesJsonData(c, gin.H{"join": tjoin, "money": money})
	} else {
		m.ResponesError(c, errcode, "")
	}
}

// 玩家领取任务奖励
func (m *AppLoginModule) apiGetTaskReward(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	taskid := c.DefaultQuery("taskid", "")
	if taskid == "" {
		m.ResponesError(c, 1, "数据错误")
		return
	}
	objid := taskIdToObjectId(taskid)
	if objid == nil {
		m.ResponesError(c, 1, "数据错误")
		return
	}
	pipline := getTaskWithJoinPipline(*objid)
	hash := util.StringHash(taskid)
	money := -1
	getmoney := 0
	errcode := m._mg_mgr.RequestFuncCallHash(hash, func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		task, err := getTaskByPipline(qc, ctx, pipline)
		if err != nil {
			return util.ERRCODE_ERROR
		}
		if task.MoneyType != util.MONEY_REWARD {
			return util.ERRCODE_ERROR
		}
		pos := 0
		for i := 0; i < len(task.Join.Data); i++ {
			jinfo := &task.Join.Data[i]
			if jinfo.Cid == cid {
				if jinfo.Money == 0 || jinfo.State != FINISH_HAVE_MONEY {
					return util.ERRCODE_ERROR
				}
				pos = i
				getmoney = jinfo.Money
				break
			}
		}
		if getmoney == 0 {
			return util.ERRCODE_ERROR
		}
		_, err = qc.DoTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
			coll := qc.Database.Collection(COLL_TASK_JOIN)
			field := fmt.Sprintf("data.%d.state", pos)
			err := coll.UpdateOne(sessCtx, bson.M{"_id": objid}, bson.M{"$set": bson.M{field: FINISH_GET_MONEY}})
			if err != nil {
				return nil, err
			}
			// 更新money
			res := m._sql_mgr.RequestFuncCallHash(uint32(cid), func(d *gorm.DB) interface{} {
				var buser b_user
				r := d.Select("cid", "money").Find(&buser, "cid=?", cid)
				if r.Error != nil {
					return r.Error
				}
				money = getmoney + buser.Money
				r = d.Model(&buser).Update("money", money)
				return r.Error
			})
			if res != nil {
				return nil, res.(error)
			}
			return nil, nil
		})
		if err != nil {
			util.Log_error("get task reward err %s", err.Error())
			return util.ERRCODE_ERROR
		}
		return util.ERRCODE_SUCCESS
	}).(int)
	if errcode == util.ERRCODE_SUCCESS {
		m.ResponesJsonData(c, gin.H{"money": money, "getmoney": getmoney})
	} else {
		m.ResponesError(c, errcode, "")
	}
}

// 玩家支付任务费用
func (m *AppLoginModule) apiPayTaskCost(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	taskid := c.DefaultQuery("taskid", "")
	if taskid == "" {
		m.ResponesError(c, 1, "数据错误")
		return
	}
	objid := taskIdToObjectId(taskid)
	if objid == nil {
		m.ResponesError(c, 1, "数据错误")
		return
	}
	pipline := getTaskWithJoinPipline(*objid)
	hash := util.StringHash(taskid)
	user := m.getUserInfo(cid, uint32(cid))
	money := -1
	errcode := m._mg_mgr.RequestFuncCallHash(hash, func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		task, err := getTaskByPipline(qc, ctx, pipline)
		if err != nil {
			return util.ERRCODE_ERROR
		}
		if task.MoneyType != util.MONEY_COST || task.Cid == cid {
			return util.ERRCODE_ERROR
		}
		// 金额
		paymoney := 0
		if user.Sex == util.SEX_MAN {
			paymoney = task.Money
		} else {
			paymoney = task.WomanMoney
		}
		if paymoney <= 0 || paymoney > user.Money {
			return util.ERRCODE_ERROR
		}
		pos := -1
		for i := 0; i < len(task.Join.Data); i++ {
			jinfo := &task.Join.Data[i]
			if jinfo.Cid == cid {
				if jinfo.State != FINISH_NONE {
					return util.ERRCODE_ERROR
				}
				pos = i
				jinfo.State = FINISH_HAVE_MONEY
				jinfo.Money = paymoney
				break
			}
		}
		if pos < 0 {
			return util.ERRCODE_ERROR
		}
		_, err = qc.DoTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
			coll := qc.Database.Collection(COLL_TASK_JOIN)
			field := fmt.Sprintf("data.%d.state", pos)
			moneyField := fmt.Sprintf("data.%d.money", pos)
			err := coll.UpdateOne(sessCtx, bson.M{"_id": objid}, bson.M{"$set": bson.M{field: FINISH_HAVE_MONEY, moneyField: paymoney}})
			if err != nil {
				return nil, err
			}
			// 更新money
			res := m._sql_mgr.RequestFuncCallHash(uint32(cid), func(d *gorm.DB) interface{} {
				r := d.Select("cid", "money").Find(&user, "cid=?", cid)
				if r.Error != nil {
					return r.Error
				}
				// 检查money
				if user.Money < paymoney {
					return errors.New("money not enough")
				}
				money = user.Money - paymoney
				r = d.Model(&user).Update("money", money)
				return r.Error
			})
			if res != nil {
				return nil, res.(error)
			}
			return nil, nil
		})
		if err != nil {
			util.Log_error("pay task cost err %s", err.Error())
			return util.ERRCODE_ERROR
		}
		return util.ERRCODE_SUCCESS
	}).(int)
	if errcode == util.ERRCODE_SUCCESS {
		m.ResponesJsonData(c, gin.H{"money": money})
	} else {
		m.ResponesError(c, errcode, "")
	}
}

// 发布者领取任务费用
func (m *AppLoginModule) apiGetTaskCost(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	taskid := c.DefaultQuery("taskid", "")
	if taskid == "" {
		m.ResponesError(c, 1, "数据错误")
		return
	}
	objid := taskIdToObjectId(taskid)
	if objid == nil {
		m.ResponesError(c, 1, "数据错误")
		return
	}
	pipline := getTaskWithJoinPipline(*objid)
	hash := util.StringHash(taskid)
	money := -1
	getmoney := 0
	var tjoin *mg_task_join
	errcode := m._mg_mgr.RequestFuncCallHash(hash, func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		task, err := getTaskByPipline(qc, ctx, pipline)
		if err != nil {
			return util.ERRCODE_ERROR
		}
		if task.MoneyType != util.MONEY_COST || task.Cid != cid {
			return util.ERRCODE_ERROR
		}
		for i := 0; i < len(task.Join.Data); i++ {
			jinfo := &task.Join.Data[i]
			if jinfo.Money > 0 && jinfo.State == FINISH_HAVE_MONEY {
				getmoney += jinfo.Money
				jinfo.State = FINISH_GET_MONEY
			}
		}
		if getmoney <= 0 {
			return util.ERRCODE_ERROR
		}
		tjoin = task.Join
		_, err = qc.DoTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
			coll := qc.Database.Collection(COLL_TASK_JOIN)
			err := coll.UpdateOne(sessCtx, bson.M{"_id": objid}, bson.M{"$set": bson.M{"data": task.Join.Data}})
			if err != nil {
				return nil, err
			}
			// 更新money
			res := m._sql_mgr.RequestFuncCallHash(uint32(cid), func(d *gorm.DB) interface{} {
				var buser b_user
				r := d.Select("cid", "money").Find(&buser, "cid=?", cid)
				if r.Error != nil {
					return r.Error
				}
				money = getmoney + buser.Money
				r = d.Model(&buser).Update("money", money)
				return r.Error
			})
			if res != nil {
				return nil, res.(error)
			}
			return nil, nil
		})
		return util.ERRCODE_SUCCESS
	}).(int)
	if errcode == util.ERRCODE_SUCCESS {
		m.ResponesJsonData(c, gin.H{"money": money, "join": tjoin, "getmoney": getmoney})
	} else {
		m.ResponesError(c, errcode, "")
	}
}

func (m *AppLoginModule) apiEditName(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	name := c.DefaultQuery("name", "")
	namelen := util.StringCharLen(name)
	if name == "" || namelen < 2 || namelen > 15 {
		m.ResponesError(c, util.ERRCODE_ERROR, "昵称不符合规范")
		return
	}

	res := m._sql_mgr.RequestFuncCallHash(uint32(cid), func(d *gorm.DB) interface{} {
		r := d.Model(&b_user{Cid: cid}).Update("name", name)
		if r.Error != nil {
			util.Log_error("apiEditName %s", r.Error.Error())
			return false
		}
		return true
	}).(bool)
	if res {
		m.ResponesJsonData(c, nil)
	} else {
		m.ResponesError(c, util.ERRCODE_ERROR, "服务器错位")
	}
}

func (m *AppLoginModule) apiEditSex(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	sex := c.DefaultQuery("sex", "")
	if sex != "0" && sex != "1" {
		m.ResponesError(c, util.ERRCODE_ERROR, "数据错误")
		return
	}

	sexval := util.StringToInt(sex)
	res := m._sql_mgr.RequestFuncCallHash(uint32(cid), func(d *gorm.DB) interface{} {
		r := d.Model(&b_user{Cid: cid}).Update("sex", sexval)
		if r.Error != nil {
			util.Log_error("apiEditSex %s", r.Error.Error())
			return false
		}
		return true
	}).(bool)
	if res {
		m.ResponesJsonData(c, nil)
	} else {
		m.ResponesError(c, util.ERRCODE_ERROR, "服务器错位")
	}
}