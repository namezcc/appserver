package module

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"goserver/handle"
	"goserver/util"
	"hash/crc32"
	"io"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/go-ego/gse"
	"github.com/go-redis/redis/v8"
	"github.com/qiniu/qmgo"
	qmoption "github.com/qiniu/qmgo/options"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gorm.io/gorm"
)

const (
	COLL_TASK_JOIN        = "task_join"
	COLL_TASK             = "task"
	COLL_USER_TASK        = "user_task" //加入的任务
	COLL_TASK_CHAT        = "task_chat"
	COLL_USER_CREATE_TASK = "user_create_task" //创建的任务
	COLL_REPORT_TASK      = "report_task"      //举报任务
	COLL_REPORT_USER      = "report_user"      //举报用户
	COLL_BLACK_LIST       = "black_list"       //黑名单
	COLL_CHAT_USER        = "chat_user"        //私聊
	COLL_USER_INTEREST    = "user_interest"    //收藏
	COLL_APP_CRASH        = "app_crash"        //崩溃信息
	COLL_TASK_CHECK       = "task_check"       //审核中
	COLL_TASK_GLOBEL      = "task_globel"      //全服任务
	COLL_TASK_LOCATION    = "task_location"    //本地任务
	COLL_CHAT_USER_LIST   = "chat_user_list"   //聊天缓存
	COLL_SUGGEST          = "user_suggest"     //建议
)

const (
	MAX_USER_INTEREST = 100
	CREDIT_SCORE_BASE = 100
)

type JsonStringList struct {
	Data []string `json:"data"`
}

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

func createOfficialMongoClient() *mongo.Client {
	host := util.GetConfValue("mongo-host")
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(host))
	if err != nil {
		panic(err)
	}
	return client
}

func creatCollIndex(cli *mongo.Client, collname string, models []mongo.IndexModel) {
	coll := cli.Database("test").Collection(collname)
	_, err := coll.Indexes().CreateMany(context.Background(), models)
	if err != nil {
		util.Log_error("createIndex coll:%s err:%s", collname, err.Error())
	}
}

// 初始化索引
func initCollIndex() {
	cli := createOfficialMongoClient()
	// black_list
	indexs := []mongo.IndexModel{
		{Keys: bson.D{{Key: "cid", Value: 1}}},
	}
	creatCollIndex(cli, COLL_BLACK_LIST, indexs)
	// chat_user
	indexs = []mongo.IndexModel{
		{Keys: bson.D{{Key: "tocid", Value: 1}}},
	}
	creatCollIndex(cli, COLL_CHAT_USER, indexs)
	// chat_user_list
	indexs = []mongo.IndexModel{
		{Keys: bson.D{{Key: "cidhei", Value: 1}}},
		{Keys: bson.D{{Key: "cidlow", Value: 1}}},
	}
	creatCollIndex(cli, COLL_CHAT_USER_LIST, indexs)
	// task_globel
	indexs = []mongo.IndexModel{
		{Keys: bson.D{{Key: "title", Value: "text"}}},
		{Keys: bson.D{{Key: "updateAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
	}
	creatCollIndex(cli, COLL_TASK_GLOBEL, indexs)
	// task_location
	indexs = []mongo.IndexModel{
		{Keys: bson.D{{Key: "location", Value: "2dsphere"}}},
		{Keys: bson.D{{Key: "title", Value: "text"}}},
		{Keys: bson.D{{Key: "updateAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
	}
	creatCollIndex(cli, COLL_TASK_LOCATION, indexs)
	// user_create_task
	indexs = []mongo.IndexModel{
		{Keys: bson.D{{Key: "cid", Value: 1}}},
	}
	creatCollIndex(cli, COLL_USER_CREATE_TASK, indexs)
	// user_interest
	indexs = []mongo.IndexModel{
		{Keys: bson.D{{Key: "cid", Value: 1}}},
	}
	creatCollIndex(cli, COLL_USER_INTEREST, indexs)
	// user_task
	indexs = []mongo.IndexModel{
		{Keys: bson.D{{Key: "cid", Value: 1}}},
	}
	creatCollIndex(cli, COLL_USER_TASK, indexs)
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

var cutseg gse.Segmenter

func (m *AppLoginModule) Init(mgr *moduleMgr) {
	m.HttpModule.Init(mgr)
	m._httpbase = m
	m.SetHost(":" + util.GetConfValue("hostport"))
	m.SetMysqlHots("")
	m._safe_req = make(map[string]bool)
	// m._redis_mgr = mgr.GetModule(MOD_REDIS_MGR).(*RedisManagerModule)
	m._sql_mgr = mgr.GetModule(MOD_MYSQL_MGR).(*MysqlManagerModule)
	m._mg_mgr = mgr.GetModule(MOD_MONGO_MGR).(*MongoManagerModule)
	m._server_mod = mgr.GetModule(MOD_CLIENT_SERVER).(*ClientServerModule)

	cutseg.LoadDict("dict/t_1.txt, dict/s_1.txt")
	initCollIndex()
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
			c.JSON(http.StatusUnauthorized, gin.H{"code": util.ERRCODE_UNAUTHORIZED, "msg": "身份认证失败"})
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
	// r.StaticFS("/web", http.Dir("./view/"))
	// r.GET("/web/*filepath", func(c *gin.Context) {
	// 	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	// 	c.File(c.FullPath())
	// })

	wg := r.Group("/web")
	wg.Use(func(ctx *gin.Context) {
		ctx.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		ctx.Next()
	})
	wg.StaticFS("/", http.Dir("./view/"))

	r.Use(m.getMiddlew())

	m.safeGet(r, "/phoneCode", m.phoneCode)
	m.safePost(r, "/userlogin", m.userLogin)
	m.safePost(r, "/userloginWxCode", m.userloginWxCode)

	m.safeGet(r, "/getUserInfo", m.apiGetUserInfo)
	r.GET("/getSelfUserInfo", m.getSelfUserInfo)
	m.safePost(r, "/apiGetUserList", m.apiGetUserList)
	// r.POST("/apiGetUserList", m.apiGetUserList)

	r.GET("/userRefreshToken", m.userRefreshToken)
	r.POST("/apiCreateTask", m.apiCreateTask)
	r.POST("/apiUpdateTask", m.apiUpdateTask)

	m.safePost(r, "/apiGetTaskInfo", m.apiGetTaskInfo)
	m.safeGet(r, "/apiGetOneTaskInfo", m.apiGetOneTaskInfo)

	r.GET("/apiLoadMyTaskInfo", m.apiLoadMyTaskInfo)
	r.GET("/apiLoadOtherTaskInfo", m.apiLoadOtherTaskInfo)
	r.GET("/apiLoadMyJoinTaskInfo", m.apiLoadMyJoinTaskInfo)
	r.GET("/apiDeleteMyTaskInfo", m.apiDeleteMyTaskInfo)

	r.POST("/uploadTaskImage", bindApiHandle(m, apiUploadTaskImage))
	m.safePost(r, "/apiUploadOssImage", bindApiHandle(m, apiUploadOssImage))

	r.GET("/apiJoinTask", m.apiJoinTask)
	r.GET("/apiQuitTask", m.apiQuitTask)
	r.GET("/apiKickTask", m.apiKickTask)
	r.GET("/apiDeleteUserJoin", m.apiDeleteUserJoin)

	r.POST("/apiFinishTask", m.apiFinishTask)
	r.GET("/apiSetFinishTask", m.apiSetFinishTask)
	r.GET("/apiGetTaskReward", m.apiGetTaskReward)
	r.GET("/apiPayTaskCost", m.apiPayTaskCost)
	r.GET("/apiGetTaskCost", m.apiGetTaskCost)

	r.GET("/apiEditName", m.apiEditName)
	// r.GET("/apiEditSex", m.apiEditSex)
	r.POST("/apiSetUserIcon", m.apiSetUserIcon)

	r.POST("/apiReportTask", m.apiReportTask)
	r.POST("/apiReportUser", m.apiReportUser)
	m.safePost(r, "/apiUserSuggest", m.apiUserSuggest)

	r.GET("/apiPushBlackList", m.apiPushBlackList)
	r.GET("/apiPullBlackList", m.apiPullBlackList)
	r.GET("/apiGetBlackList", m.apiGetBlackList)

	// m.safePost(r, "/apiSearchTask", m.apiSearchTask)
	r.POST("/apiSearchTask", m.apiSearchTask)

	r.GET("/apiTaskPushInterest", m.apiTaskPushInterest)
	r.GET("/apiTaskPullInterest", m.apiTaskPullInterest)
	r.GET("/apiLoadInterest", m.apiLoadInterest)
	r.GET("/apiLoadInterestTask", m.apiLoadInterestTask)

	m.safePost(r, "/apiAppCrash", m.apiAppCrash)
	m.safePost(r, "/apiAppError", m.apiAppError)
	m.safePost(r, "/apiTaskCheck", m.apiTaskCheck)
	m.safeGet(r, "/apiTaskCheckAll", m.apiTaskCheckAll)

	r.GET("/apiLoadTaskChat", m.apiLoadTaskChat)
	r.GET("/apiLoadUserChatList", m.apiLoadUserChatList)
	r.GET("/apiLoadUserChatData", m.apiLoadUserChatData)
	r.GET("/apiLoadOneUserChatData", m.apiLoadOneUserChatData)
	r.GET("/apiDeleteUserChatData", m.apiDeleteUserChatData)

	r.GET("/apiGetCreditToUserType", m.apiGetCreditToUserType)
	r.GET("/apiSetCreditToUserType", m.apiSetCreditToUserType)
	r.GET("/apiCheckIdCard", m.apiCheckIdCard)
	r.GET("/apiGetOssCredential", m.apiGetOssCredential)
	m.safeGet(r, "/apiGetPostPolicy", m.apiGetPostPolicy)
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
		Code: util.ERRCODE_SUCCESS,
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
	codeinfo.Time = util.GetSecond() + 30
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
	// c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
	m.ResponesJsonData(c, codeinfo.Code)
}

type userLoginData struct {
	Phone string `json:"phone,omitempty"`
	Code  string `json:"code,omitempty"`
}

// 生成随机8个英文字符的字符串
func generateRandomString() string {
	// 定义包含所有可能字符的字符串
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	// 生成8个随机字符
	result := make([]byte, 8)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}

func (m *AppLoginModule) userLogin(c *gin.Context) {
	var info userLoginData
	err := c.ShouldBindJSON(&info)
	if err != nil {
		util.Log_error("userLogin json err:%s", err.Error())
		ResponesCommonError(c, "数据错误")
		return
	}

	if len(info.Phone) == 0 {
		ResponesCommonError(c, "数据错误")
		return
	}
	// 检查是否测试账号

	m.userRealLogin(c, info.Phone)
}

type UserPhoneRes struct {
	Errcode     int    `json:"errcode,omitempty"`
	Errmsg      string `json:"errmsg,omitempty"`
	PhoneNumber string `json:"phoneNumber,omitempty"`
}

func (m *AppLoginModule) userloginWxCode(c *gin.Context) {
	var info userLoginData
	err := c.ShouldBindJSON(&info)
	if err != nil {
		util.Log_error("userWxLogin json err:%s", err.Error())
		ResponesCommonError(c, "数据错误")
		return
	}

	if len(info.Code) == 0 {
		ResponesCommonError(c, "数据错误")
		return
	}

	url := util.GetConfValue("nodeServerHost") + "getPhoneNumber"
	pdata, err := json.Marshal(gin.H{
		"code": info.Code,
	})
	if err != nil {
		util.Log_error("userWxLogin step 1 err:%s", err.Error())
		ResponesCommonError(c, "服务器错误")
		return
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(pdata))
	if err != nil {
		util.Log_error("userWxLogin step 2 err:%s", err.Error())
		ResponesCommonError(c, "服务器错误")
		return
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		util.Log_error("userWxLogin step 3 err:%s", err.Error())
		ResponesCommonError(c, "服务器错误")
		return
	}

	msg, err := io.ReadAll(resp.Body)
	if err != nil {
		util.Log_error("userWxLogin step 4 err:%s", err.Error())
		ResponesCommonError(c, "服务器错误")
		return
	}
	var userPhone UserPhoneRes
	err = json.Unmarshal(msg, &userPhone)
	if err != nil {
		util.Log_error("userWxLogin step 5 err:%s", err.Error())
		ResponesCommonError(c, "服务器错误")
	} else {
		if len(userPhone.PhoneNumber) == 0 {
			util.Log_error("userWxLogin step 6 code:%d err:%s", userPhone.Errcode, userPhone.Errmsg)
			ResponesCommonError(c, "手机号获取失败")
		} else {
			// ResponesJsonData(c, userPhone)
			m.userRealLogin(c, userPhone.PhoneNumber)
		}
	}
}

func (m *AppLoginModule) userRealLogin(c *gin.Context, phoneNumber string) {
	hashval := crc32.ChecksumIEEE([]byte(phoneNumber))
	// 查询账号
	userId := int64(0)
	m._sql_mgr.RequestFuncCallNoResHash(hashval, func(d *gorm.DB) {
		var buser b_user
		sqlres := d.Select("cid", "phone").Find(&buser, "phone = ?", phoneNumber)
		// 创建账号
		if sqlres.Error != nil {
			util.Log_error("select user error %s", sqlres.Error.Error())
			return
		}
		if buser.Cid == 0 {
			buser.Cid = m.genUserId()
			buser.Phone = phoneNumber
			buser.Name = generateRandomString()
			sqlres := d.Create(&buser)
			if sqlres.Error != nil {
				util.Log_error("create user error %s", sqlres.Error.Error())
				return
			}
		}
		userId = buser.Cid
	})

	if userId == 0 {
		ResponesError(c, util.USER_HTTP_ERR_ERROR, "account error")
		return
	}

	claims := &AppJWTClaims{
		UserID: userId,
		Phone:  phoneNumber,
	}
	claims.IssuedAt = time.Now().Unix()
	claims.ExpiresAt = time.Now().Add(time.Second * time.Duration(AppExpireTime)).Unix()
	signedToken, err := AppGetToken(claims)
	if err != nil {
		ResponesError(c, util.USER_HTTP_ERR_ERROR, err.Error())
		return
	}
	ResponesJsonData(c, gin.H{"token": signedToken})
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
	Cid         int64  `gorm:"primaryKey" json:"cid"`
	Phone       string `json:"phone"`
	Name        string `json:"name"`
	Sex         int    `json:"sex"`
	Icon        string `json:"icon"`
	Birth       string `json:"birth"`
	Money       int    `json:"money"`
	CreditScore int    `json:"credit_score"`
	Age         int    `json:"age"`
	Weixin      string `json:"weixin"`
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
	strcid := c.DefaultQuery("cid", "")
	cid := int64(0)
	if len(strcid) == 0 {
		m.ResponesError(c, util.ERRCODE_ERROR, "数据错误")
		return
	} else {
		cid = util.StringToInt64(strcid)
	}
	hashval := uint32(cid)
	buser := m.getUserInfo(cid, hashval)
	if buser == nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "没有用户信息"})
		return
	}
	m.ResponesJsonData(c, *buser)
}

func (m *AppLoginModule) getSelfUserInfo(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	hashval := uint32(cid)
	buser := m.getUserInfo(cid, hashval)
	if buser == nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "msg": "没有用户信息"})
		return
	}
	m.ResponesJsonData(c, *buser)
}

type UserCidList struct {
	Cids []int64 `json:"cids"`
}

func (m *AppLoginModule) apiGetUserList(c *gin.Context) {
	var list UserCidList
	err := c.ShouldBindJSON(&list)
	if err != nil || len(list.Cids) == 0 {
		util.Log_error("apiGetUserList 1 err:%s", err.Error())
		m.ResponesError(c, util.ERRCODE_ERROR, "数据错误")
		return
	}

	var users []b_user
	m._sql_mgr.RequestFuncCall(func(d *gorm.DB) interface{} {
		res := d.Select("cid", "name").Find(&users, list.Cids)
		if res.Error != nil {
			util.Log_error("apiGetUserList 2 err:%s", res.Error.Error())
		}
		return nil
	})
	m.ResponesJsonData(c, users)
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
	Id            primitive.ObjectID `json:"id,omitempty" bson:"_id"`
	CreateAt      string             `json:"createAt,omitempty" bson:"createAt"`
	UpdateAt      string             `json:"updateAt,omitempty" bson:"updateAt"`
	Cid           int64              `json:"cid,omitempty" bson:"cid"`
	CreatorName   string             `json:"creator_name,omitempty" bson:"creator_name"`
	CreatorIcon   string             `json:"creator_icon,omitempty" bson:"creator_icon,omitempty"`
	Title         string             `json:"title" bson:"title"`
	Content       string             `json:"content" bson:"content"`
	Images        []string           `json:"images,omitempty" bson:"images,omitempty"`
	MoneyType     int                `json:"money_type" bson:"money_type"`
	Money         int                `json:"money" bson:"money"`
	WomanMoney    int                `json:"womanMoney" bson:"womanMoney"`
	PeopleNum     int                `json:"people_num" bson:"people_num"`
	ManNum        int                `json:"man_num" bson:"man_num"`
	ContactWay    string             `json:"contact_way,omitempty" bson:"contact_way,omitempty"`
	EndTime       int64              `json:"end_time" bson:"end_time"`
	TaskStartTime int64              `json:"task_start_time,omitempty" bson:"task_start_time,omitempty"`
	TaskEndTime   int64              `json:"task_end_time,omitempty" bson:"task_end_time,omitempty"`
	CreditScore   int                `json:"credit_score,omitempty" bson:"credit_score,omitempty"`
	NonPublic     int                `json:"nonpublic,omitempty" bson:"nonpublic,omitempty"`
	Address       *addressInfo       `json:"address,omitempty" bson:"address,omitempty"`
	Delete        int                `json:"delete,omitempty" bson:"delete,omitempty"`
	Join          *mg_task_join      `json:"join,omitempty" bson:"join,omitempty"`
	State         int                `json:"state,omitempty" bson:"state,omitempty"`
}

func (m *mg_task) isPublic() bool {
	return m.NonPublic <= 0
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
	Title    string             `json:"title" bson:"title"`
}

type mg_task_globel struct {
	Id       primitive.ObjectID `bson:"_id"`
	UpdateAt time.Time          `bson:"updateAt"`
	Title    string             `json:"title" bson:"title"`
}

type mg_task_join_info struct {
	Cid    int64  `json:"cid" bson:"cid"`
	Name   string `json:"name" bson:"name"`
	Sex    int    `json:"sex" bson:"sex"`
	Icon   string `json:"icon" bson:"icon"`
	State  int    `json:"state" bson:"state"`
	Money  int    `json:"money" bson:"moneye"`
	NoChat int    `json:"nochat" bson:"nochat"`
	Time   int64  `json:"time" bson:"time"`
}

type mg_task_join struct {
	Id   primitive.ObjectID  `json:"id" bson:"_id"`
	Cid  int64               `json:"cid" bson:"cid"`
	Data []mg_task_join_info `json:"data" bson:"data"`
}

type My_join_task struct {
	Id    primitive.ObjectID `json:"id" bson:"_id"`
	Time  int64              `json:"time" bson:"time"`
	State int                `json:"state" bson:"state"`
}

type mg_task_my_join struct {
	Id       primitive.ObjectID `json:"id" bson:"_id"`
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
	if titlenum <= 0 || titlenum > 30 || contentnum > 500 {
		util.Log_error("task title:%d content:%d", titlenum, contentnum)
		return false
	}

	if util.StringCharLen(task.ContactWay) > 30 {
		util.Log_error("task maxlen contactway:%d", util.StringCharLen(task.ContactWay))
		return false
	}
	return true
}

func checkInBlackList(qc *qmgo.QmgoClient, ctx context.Context, cid int64, blackcid int64) bool {
	coll := qc.Database.Collection(COLL_BLACK_LIST)
	n, err := coll.Find(ctx, bson.M{"cid": cid, "black": bson.M{"$in": bson.A{blackcid}}}).Count()
	if err != nil {
		util.Log_error("check in black err:%s", err.Error())
	}
	return n > 0
}

type mg_task_check struct {
	Id   primitive.ObjectID `json:"id,omitempty" bson:"_id"`
	Time int                `json:"time,omitempty" bson:"time"`
}

func addTaskCheck(qc *qmgo.QmgoClient, ctx context.Context, taskid primitive.ObjectID) error {
	coll := qc.Database.Collection(COLL_TASK_CHECK)
	upopts := options.Update().SetUpsert(true)
	qmopt := qmoption.UpdateOptions{UpdateHook: nil, UpdateOptions: upopts}
	return coll.UpdateOne(ctx, bson.M{"_id": taskid}, bson.M{"$set": bson.M{"time": util.GetSecond()}}, qmopt)
}

func setTaskChecked(qc *qmgo.QmgoClient, ctx context.Context, taskid primitive.ObjectID) error {
	coll := qc.Database.Collection(COLL_TASK)
	var task mg_task
	err := coll.Find(ctx, bson.M{"_id": taskid}).One(&task)
	if err != nil {
		return err
	}

	if task.State != util.TASK_STATE_IN_CHECK {
		return nil
	}
	// 删除审核
	collcheck := qc.Database.Collection(COLL_TASK_CHECK)
	collcheck.RemoveId(ctx, taskid)

	// 设置状态
	err = coll.UpdateOne(ctx, bson.M{"_id": taskid}, bson.M{"$set": bson.M{"state": util.TASK_STATE_OPEN}})
	if err != nil {
		return err
	}
	// 加入global/location
	// 过期时间
	expireTime := task.EndTime - util.GetSecond()
	worlds := cutseg.CutSearch(task.Title)
	title := strings.Join(worlds, " ")
	if task.Address != nil {
		taskloc := mg_task_location{
			Id:       task.Id,
			Location: NewLocation(task.Address.Longitude, task.Address.Latitude),
			UpdateAt: time.Now().Add(time.Second * time.Duration(expireTime)),
			Title:    title,
		}
		coll := qc.Database.Collection(COLL_TASK_LOCATION)
		_, err := coll.InsertOne(ctx, taskloc)
		if err != nil {
			return err
		}
	} else {
		taskglo := mg_task_globel{
			Id:       task.Id,
			UpdateAt: time.Now().Add(time.Second * time.Duration(expireTime)),
			Title:    title,
		}
		coll := qc.Database.Collection(COLL_TASK_GLOBEL)
		_, err := coll.InsertOne(ctx, taskglo)
		if err != nil {
			return err
		}
	}
	return nil
}

func addToPublic(qc *qmgo.QmgoClient, ctx context.Context, task mg_task) error {
	// 加入global/location
	// 过期时间
	expireTime := task.EndTime - util.GetSecond()
	worlds := cutseg.CutSearch(task.Title)
	title := strings.Join(worlds, " ")
	if task.Address != nil {
		taskloc := mg_task_location{
			Id:       task.Id,
			Location: NewLocation(task.Address.Longitude, task.Address.Latitude),
			UpdateAt: time.Now().Add(time.Second * time.Duration(expireTime)),
			Title:    title,
		}
		coll := qc.Database.Collection(COLL_TASK_LOCATION)
		_, err := coll.InsertOne(ctx, taskloc)
		if err != nil {
			return err
		}
	} else {
		taskglo := mg_task_globel{
			Id:       task.Id,
			UpdateAt: time.Now().Add(time.Second * time.Duration(expireTime)),
			Title:    title,
		}
		coll := qc.Database.Collection(COLL_TASK_GLOBEL)
		_, err := coll.InsertOne(ctx, taskglo)
		if err != nil {
			return err
		}
	}
	return nil
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

	user := m.getUserInfo(cid, hashval)
	if user == nil {
		m.ResponesError(c, 1, "用户数据错误")
		return
	}

	task.Cid = cid
	task.Id = qmgo.NewObjectID()
	nowtimestr := util.NowTime()
	task.CreateAt = nowtimestr
	task.UpdateAt = nowtimestr
	task.CreatorName = user.Name
	task.CreatorIcon = user.Icon
	task.State = util.TASK_STATE_IN_CHECK

	// 插入数据库
	ires := m._mg_mgr.RequestFuncCallHash(hashval, func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		// 事务
		_, err := qc.DoTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
			// 加入审核列表
			// err := addTaskCheck(qc, sessCtx, task.Id)
			// if err != nil {
			// 	util.Log_error("insert task check err:%s", err.Error())
			// 	return nil, err
			// }
			task.State = util.TASK_STATE_OPEN
			if task.isPublic() {
				err := addToPublic(qc, sessCtx, task)
				if err != nil {
					util.Log_error("insert task to public err:%s", err.Error())
					return nil, err
				}
			}

			coll := qc.Database.Collection(COLL_TASK)
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
		// 检查违规
		m.checkTask(&task)
	} else {
		m.ResponesError(c, 1, "创建task错误")
	}
}

func stringToObjectId(id string) *primitive.ObjectID {
	objid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		util.Log_error("taskid err: %s", err.Error())
		return nil
	}
	return &objid
}

func (m *AppLoginModule) GetTaskInfo(taskid string) *mg_task {
	objid := stringToObjectId(taskid)
	if objid == nil {
		return nil
	}
	return m.getTaskInfoObjId(*objid, util.StringHash(taskid))
}

func getTaskInfoByQmgo(qc *qmgo.QmgoClient, ctx context.Context, objid primitive.ObjectID) (*mg_task, error) {
	var task mg_task
	coll := qc.Database.Collection(COLL_TASK)
	err := coll.Find(ctx, bson.M{"_id": objid}).One(&task)
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func getTaskByPipline(qc *qmgo.QmgoClient, ctx context.Context, pipline interface{}) (*mg_task, error) {
	var task mg_task
	coll := qc.Database.Collection(COLL_TASK)
	err := coll.Aggregate(ctx, pipline).One(&task)
	if err != nil {
		util.Log_error("gettask pipline err: %s", err.Error())
		return nil, err
	}
	return &task, nil
}

func (m *AppLoginModule) getTaskInfoObjId(objid primitive.ObjectID, hashval uint32) *mg_task {
	tres := m._mg_mgr.RequestFuncCallHash(hashval, func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		task, err := getTaskInfoByQmgo(qc, ctx, objid)
		if err != nil {
			util.Log_error("get taskid:%s err:%s", objid.Hex(), err.Error())
		}
		return task
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
	cid := c.MustGet("userId").(int64)
	hashval := c.MustGet("phoneHash").(uint32)
	user := m.getUserInfo(cid, hashval)
	if user == nil {
		m.ResponesError(c, 1, "用户数据错误")
		return
	}

	task.UpdateAt = util.NowTime()
	task.CreatorName = user.Name
	task.CreatorIcon = user.Icon
	task.State = util.TASK_STATE_IN_CHECK

	taskhash := util.StringHash(task.Id.String())
	tres := m._mg_mgr.RequestFuncCallHash(taskhash, func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		oldtask, err := getTaskInfoByQmgo(qc, ctx, task.Id)
		if err != nil {
			util.Log_error("update task get taskid:%s err:%s", task.Id.Hex(), err.Error())
			return false
		}
		if oldtask == nil || oldtask.State == util.TASK_STATE_FINISH || oldtask.State == util.TASK_STATE_ILLEGAL {
			// 已完成不能更新
			return false
		}

		task.Cid = cid
		task.Id = oldtask.Id
		task.CreateAt = oldtask.CreateAt
		task.UpdateAt = oldtask.UpdateAt
		task.Delete = oldtask.Delete
		task.State = oldtask.State
		// if oldtask.State == util.TASK_STATE_FINISH {
		// 	task.State = util.TASK_STATE_FINISH
		// }

		_, err = qc.DoTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
			// 更新审核
			// err := addTaskCheck(qc, sessCtx, task.Id)
			if oldtask.State != util.TASK_STATE_IN_CHECK {
				// 删除globel/location
				if oldtask.Address != nil {
					loc_coll := qc.Database.Collection(COLL_TASK_LOCATION)
					// 删除 local 报错不管
					loc_coll.RemoveId(sessCtx, task.Id)
				} else {
					// 删除 globel
					glo_coll := qc.Database.Collection(COLL_TASK_GLOBEL)
					glo_coll.RemoveId(sessCtx, task.Id)
				}
			}
			task.State = util.TASK_STATE_OPEN
			if task.isPublic() {
				err := addToPublic(qc, sessCtx, task)
				if err != nil {
					util.Log_error("insert task to public err:%s", err.Error())
					return nil, err
				}
			}
			// 更新 task
			coll := qc.Database.Collection(COLL_TASK)
			err = coll.ReplaceOne(sessCtx, bson.M{"_id": task.Id}, task)
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
		// 检查违规
		m.checkTask(&task)
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
	Search      string  `json:"search"`
	SelectType  int     `json:"select_type"`
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
	if taskconf.LocMax == 0 && taskconf.SelectType == 0 {
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
			coll := qc.Database.Collection(COLL_TASK_LOCATION)
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

	if taskconf.GlobelMax == 0 && taskconf.SelectType == 1 {
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
			bson.M{"$match": bson.M{"result": bson.M{"$ne": bson.A{}}}},
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
			coll := qc.Database.Collection(COLL_TASK_GLOBEL)
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
	objid := stringToObjectId(taskid)
	if objid == nil {
		m.ResponesError(c, 1, "数据错误")
		return
	}
	pipline := getTaskWithJoinPipline(*objid)
	hash := util.StringHash(taskid)
	res := m._mg_mgr.RequestFuncCallHash(hash, func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		task, err := getTaskByPipline(qc, ctx, pipline)
		if err != nil || task.State == util.TASK_STATE_ILLEGAL {
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

func taskListToTaskPipline(cid int64, skip int, neednum int) bson.A {
	return bson.A{
		bson.M{"$match": bson.M{"cid": cid}},
		bson.M{"$project": bson.M{"tasklist": bson.M{"$slice": bson.A{
			"$tasklist",
			-skip - neednum,
			neednum},
		}}},
		bson.M{"$lookup": bson.D{
			{Key: "from", Value: "task"},
			{Key: "localField", Value: "tasklist._id"},
			{Key: "foreignField", Value: "_id"},
			{Key: "as", Value: "result"},
		}},
		// bson.M{"$unwind": "$result"},
		// bson.M{"$replaceRoot": bson.M{"newRoot": "$result"}},
		bson.M{"$lookup": bson.D{
			{Key: "from", Value: "task_join"},
			{Key: "localField", Value: "tasklist._id"},
			{Key: "foreignField", Value: "_id"},
			{Key: "as", Value: "task_join"},
		}},
		// bson.M{"$addFields": bson.M{"join": bson.M{"$arrayElemAt": bson.A{"$join", 0}}}},
	}
}

type loadTaskInfo struct {
	Tasklist []My_join_task `json:"tasklist" bson:"tasklist"`
	Result   []mg_task      `json:"result" bson:"result"`
	TaskJoin []mg_task_join `json:"task_join" bson:"task_join"`
}

type loadTaskInterest struct {
	Tasklist []primitive.ObjectID `json:"tasklist" bson:"tasklist"`
	Result   []mg_task            `json:"result" bson:"result"`
	TaskJoin []mg_task_join       `json:"task_join" bson:"task_join"`
}

func (m *AppLoginModule) apiLoadMyTaskInfo(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	skip := util.StringToInt(c.DefaultQuery("skip", "0"))
	neednum := 20
	pipline := taskListToTaskPipline(cid, skip, neednum)

	var loadinfo loadTaskInfo

	m._mg_mgr.RequestFuncCall(func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		coll := qc.Database.Collection(COLL_USER_CREATE_TASK)
		err := coll.Aggregate(ctx, pipline).One(&loadinfo)
		if err != nil {
			util.Log_error("load task err:%s", err.Error())
			return false
		}
		return true
	})
	m.ResponesJsonData(c, loadinfo)
}

func (m *AppLoginModule) apiLoadOtherTaskInfo(c *gin.Context) {
	cid := util.StringToInt64(c.DefaultQuery("cid", "0"))
	skip := util.StringToInt(c.DefaultQuery("skip", "0"))
	if cid <= 0 {
		m.ResponesError(c, 1, "数据错误")
		return
	}
	neednum := 20
	now := util.GetSecond()
	pipline := bson.A{
		bson.M{"$match": bson.M{"cid": cid}},
		bson.M{"$lookup": bson.D{
			{Key: "from", Value: "task"},
			{Key: "localField", Value: "tasklist._id"},
			{Key: "foreignField", Value: "_id"},
			{Key: "as", Value: "result"},
			{Key: "pipeline", Value: bson.A{
				bson.M{"$match": bson.M{
					"end_time":  bson.M{"$gt": now},
					"nonpublic": bson.M{"$ne": 1},
					"state":     bson.M{"$eq": 1},
				}},
			}},
		}},
		bson.M{"$project": bson.M{"result": bson.M{"$slice": bson.A{
			"$result",
			-skip - neednum,
			neednum},
		}}},
		bson.M{"$unwind": "$result"},
		bson.M{"$replaceRoot": bson.M{"newRoot": "$result"}},
		bson.M{"$lookup": bson.D{
			{Key: "from", Value: "task_join"},
			{Key: "localField", Value: "_id"},
			{Key: "foreignField", Value: "_id"},
			{Key: "as", Value: "join"},
		}},
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
	pipline := taskListToTaskPipline(cid, skip, neednum)

	var loadinfo loadTaskInfo
	m._mg_mgr.RequestFuncCall(func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		coll := qc.Database.Collection(COLL_USER_TASK)
		err := coll.Aggregate(ctx, pipline).One(&loadinfo)
		if err != nil {
			util.Log_error("load user join task err:%s", err.Error())
			return false
		}
		return true
	})

	m.ResponesJsonData(c, loadinfo)
}

func (m *AppLoginModule) apiDeleteMyTaskInfo(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	taskid := c.DefaultQuery("taskid", "")
	if taskid == "" {
		m.ResponesError(c, 1, "数据错误")
		return
	}

	objid := stringToObjectId(taskid)
	if objid == nil {
		m.ResponesError(c, 1, "任务id错误")
		return
	}
	taskhash := util.StringHash(taskid)
	res := m._mg_mgr.RequestFuncCallHash(taskhash, func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		_, err := qc.DoTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
			coll := qc.Database.Collection(COLL_TASK)
			err := coll.UpdateOne(sessCtx, bson.M{"cid": cid, "_id": objid}, bson.M{"$set": bson.M{"delete": 1}})
			if err != nil {
				util.Log_error("delete task err:%s", err.Error())
				return nil, err
			}
			loc_coll := qc.Database.Collection(COLL_TASK_LOCATION)
			glo_coll := qc.Database.Collection(COLL_TASK_GLOBEL)
			// 删除 location
			loc_coll.RemoveId(sessCtx, objid)
			// 删除 globel
			glo_coll.RemoveId(sessCtx, objid)
			// 删除 审核
			collcheck := qc.Database.Collection(COLL_TASK_CHECK)
			collcheck.RemoveId(sessCtx, objid)
			return nil, nil
		})
		if err == nil {
			// 删除 user create
			createColl := qc.Database.Collection(COLL_USER_CREATE_TASK)
			createColl.UpdateOne(ctx, bson.M{"cid": cid}, bson.M{"$pull": bson.M{"tasklist": bson.M{"_id": objid}}})
			// user joinn
			userjoin := qc.Database.Collection(COLL_USER_TASK)
			userjoin.UpdateOne(ctx, bson.M{"cid": cid}, bson.M{"$pull": bson.M{"tasklist": bson.M{"_id": objid}}})
			// // task chat
			// chatcoll := qc.Database.Collection(COLL_TASK_CHAT)
			// chatcoll.UpdateOne(ctx, bson.M{"_id": objid}, bson.M{"$set": bson.M{"delete": 1}})
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
	objid := stringToObjectId(taskid)
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
		if task.State != util.TASK_STATE_OPEN {
			// 不是进行中不能加入
			util.Log_waring("join task not open taskid:%s", taskid)
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
		// 信用分限制
		if user.CreditScore+CREDIT_SCORE_BASE < task.CreditScore {
			return util.ERRCODE_CREDIT_SCORE
		}
		// 任务以取消
		if task.Delete > 0 {
			return util.ERRCODE_TASK_DELETE
		}

		// 是否黑名单
		if checkInBlackList(qc, ctx, task.Cid, cid) {
			return util.ERRCODE_IN_BLACK_LIST
		}

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
				Time: nowsec,
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
			err = usercoll.UpdateOne(sessCtx, bson.M{"cid": cid}, bson.M{"$addToSet": bson.M{"tasklist": userjoin}}, qmopt)
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

	objid := stringToObjectId(taskid)
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
	objid := stringToObjectId(taskid)
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
			// usercoll := qc.Database.Collection(COLL_USER_TASK)
			// err = usercoll.UpdateOne(sessCtx, bson.M{"cid": kickcid}, bson.M{"$pull": bson.M{"tasklist": bson.M{"_id": objid}}})
			// if err != nil {
			// 	util.Log_error("quit user task err:%s", err.Error())
			// 	return nil, err
			// }
			join = task.Join
			sendres = gin.H{"join": task.Join}
			return nil, nil
		})
		return err == nil
	}).(bool)
	if res {
		m.ResponesJsonData(c, sendres)
		m.updateServerTaskJoin(join)
		m._server_mod.sendMsg(handle.M_ON_TASK_KICK, &mg_task_join{
			Id:  *objid,
			Cid: kickcid,
		})
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

// 删除UserJoin
func (m *AppLoginModule) apiDeleteUserJoin(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	taskid := c.DefaultQuery("taskid", "")
	if taskid == "" {
		m.ResponesError(c, 1, "数据错误")
		return
	}
	objid := stringToObjectId(taskid)
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
		// joincoll := qc.Database.Collection(COLL_TASK_JOIN)
		// err = joincoll.UpdateOne(ctx, bson.M{"_id": objid, "data.cid": cid}, bson.M{"$set": bson.M{"data.$.nochat": 1}})
		// if err != nil {
		// 	util.Log_error("apiDeleteUserTaskJoin 2 %s", err.Error())
		// }
		return nil
	})
	// 通知 client server
	// m._server_mod.sendMsg(handle.M_ON_TASK_NO_CHAT, &mg_task_join{
	// 	Id:  *objid,
	// 	Cid: cid,
	// })
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
		if task.State != util.TASK_STATE_FINISH {
			coll := qc.Database.Collection(COLL_TASK)
			err := coll.UpdateOne(ctx, bson.M{"_id": task.Id}, bson.M{"$set": bson.M{"state": util.TASK_STATE_FINISH}})
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

// 设置完成任务状态
func (m *AppLoginModule) apiSetFinishTask(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	taskid := c.DefaultQuery("taskid", "")
	if taskid == "" {
		m.ResponesError(c, 1, "数据错误")
		return
	}
	objid := stringToObjectId(taskid)
	if objid == nil {
		m.ResponesError(c, 1, "数据错误")
		return
	}

	hash := util.StringHash(taskid)
	m._mg_mgr.RequestFuncCallHash(hash, func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		task, err := getTaskInfoByQmgo(qc, ctx, *objid)
		if err != nil {
			util.Log_error("apiSetFinishTask step 1 err:%s", err.Error())
			return false
		}

		if task.State != util.TASK_STATE_OPEN || task.Cid != cid {
			return false
		}

		coll := qc.Database.Collection(COLL_TASK)
		err = coll.UpdateOne(ctx, bson.M{"_id": objid}, bson.M{"$set": bson.M{"state": util.TASK_STATE_FINISH}})
		if err != nil {
			util.Log_error("apiSetFinishTask step 2 err:%s", err.Error())
			return false
		}
		// 删除global,location
		loc_coll := qc.Database.Collection(COLL_TASK_LOCATION)
		glo_coll := qc.Database.Collection(COLL_TASK_GLOBEL)
		// 删除 location
		loc_coll.RemoveId(ctx, objid)
		// 删除 globel
		glo_coll.RemoveId(ctx, objid)
		return true
	})

	m.ResponesJsonData(c, nil)
}

// 玩家领取任务奖励
func (m *AppLoginModule) apiGetTaskReward(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	taskid := c.DefaultQuery("taskid", "")
	if taskid == "" {
		m.ResponesError(c, 1, "数据错误")
		return
	}
	objid := stringToObjectId(taskid)
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
	objid := stringToObjectId(taskid)
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
	objid := stringToObjectId(taskid)
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
	if name == "" || namelen < 1 || namelen > 15 {
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

type mg_report_task struct {
	Id        primitive.ObjectID `json:"id,omitempty" bson:"_id"`
	Submitcid int64              `json:"submitcid,omitempty" bson:"submitcid"`
	Taskid    string             `json:"taskid" bson:"taskid"`
	Type      int                `json:"type" bson:"type"`
	Content   string             `json:"content" bson:"content"`
	Images    []string           `json:"images" bson:"images"`
}

type mg_report_user struct {
	Id        primitive.ObjectID `json:"id,omitempty" bson:"_id"`
	Submitcid int64              `json:"submitcid,omitempty" bson:"submitcid"`
	Cid       int64              `json:"cid" bson:"cid"`
	Type      int                `json:"type" bson:"type"`
	Content   string             `json:"content" bson:"content"`
	Images    []string           `json:"images" bson:"images"`
}

type mg_suggest struct {
	Id      primitive.ObjectID `json:"id,omitempty" bson:"_id"`
	Cid     int64              `json:"cid,omitempty" bson:"cid"`
	Content string             `json:"content" bson:"content"`
	Images  []string           `json:"images" bson:"images"`
}

func (m *AppLoginModule) apiReportTask(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	var reptask mg_report_task
	err := c.ShouldBindJSON(&reptask)
	if err != nil {
		util.Log_error("apiReportTask json error:%s", err.Error())
		m.ResponesError(c, 1, "输入数据错误")
		return
	}

	if len(reptask.Images) > 3 {
		util.Log_error("apiReportTask images len:%d", len(reptask.Images))
		m.ResponesError(c, 1, "输入数据错误")
		return
	}

	reptask.Submitcid = cid
	reptask.Id = qmgo.NewObjectID()
	// 存入mgdb
	m._mg_mgr.RequestFuncCallNoRes(func(ctx context.Context, qc *qmgo.QmgoClient) {
		coll := qc.Database.Collection(COLL_REPORT_TASK)
		_, err := coll.InsertOne(ctx, reptask)
		if err != nil {
			util.Log_error("apiReportTask err:%s", err.Error())
		}
	})
	m.ResponesJsonData(c, nil)
}

func (m *AppLoginModule) apiReportUser(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	var rep mg_report_user
	err := c.ShouldBindJSON(&rep)
	if err != nil {
		util.Log_error("apiReportUser json error:%s", err.Error())
		m.ResponesError(c, 1, "输入数据错误")
		return
	}

	if len(rep.Images) > 3 {
		util.Log_error("apiReportUser images len:%d", len(rep.Images))
		m.ResponesError(c, 1, "输入数据错误")
		return
	}

	rep.Id = qmgo.NewObjectID()
	rep.Submitcid = cid
	// 存入mgdb
	m._mg_mgr.RequestFuncCallNoRes(func(ctx context.Context, qc *qmgo.QmgoClient) {
		coll := qc.Database.Collection(COLL_REPORT_USER)
		_, err := coll.InsertOne(ctx, rep)
		if err != nil {
			util.Log_error("apiReportUser err:%s", err.Error())
		}
	})
	m.ResponesJsonData(c, nil)
}

func (m *AppLoginModule) apiUserSuggest(c *gin.Context) {
	var rep mg_suggest
	err := c.ShouldBindJSON(&rep)
	if err != nil {
		util.Log_error("apiUserSuggest json error:%s", err.Error())
		m.ResponesError(c, 1, "输入数据错误")
		return
	}

	if len(rep.Images) > 3 {
		util.Log_error("apiUserSuggest images len:%d", len(rep.Images))
		m.ResponesError(c, 1, "输入数据错误")
		return
	}

	rep.Id = qmgo.NewObjectID()
	// 存入mgdb
	m._mg_mgr.RequestFuncCallNoRes(func(ctx context.Context, qc *qmgo.QmgoClient) {
		coll := qc.Database.Collection(COLL_SUGGEST)
		_, err := coll.InsertOne(ctx, rep)
		if err != nil {
			util.Log_error("apiUserSuggest err:%s", err.Error())
		}
	})
	m.ResponesJsonData(c, nil)
}

func (m *AppLoginModule) apiPushBlackList(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	strcid := c.DefaultQuery("cid", "")
	blackcid := util.StringToInt64(strcid)
	if blackcid <= 0 || blackcid == cid {
		m.ResponesError(c, 1, "输入数据错误")
		return
	}
	upopts := options.Update().SetUpsert(true)
	qmopt := qmoption.UpdateOptions{UpdateHook: nil, UpdateOptions: upopts}
	res := m._mg_mgr.RequestFuncCallHash(uint32(cid), func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		coll := qc.Database.Collection(COLL_BLACK_LIST)
		err := coll.UpdateOne(ctx, bson.M{"cid": cid}, bson.M{"$addToSet": bson.M{"black": blackcid}}, qmopt)
		if err != nil {
			util.Log_error("push black err:%s", err.Error())
			return false
		}
		return true
	}).(bool)
	if res {
		m.ResponesJsonData(c, nil)
	} else {
		m.ResponesError(c, util.ERRCODE_ERROR, "服务器错误")
	}
}

func (m *AppLoginModule) apiPullBlackList(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	strcid := c.DefaultQuery("cid", "")
	blackcid := util.StringToInt64(strcid)
	if blackcid <= 0 || blackcid == cid {
		m.ResponesError(c, 1, "输入数据错误")
		return
	}
	res := m._mg_mgr.RequestFuncCallHash(uint32(cid), func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		coll := qc.Database.Collection(COLL_BLACK_LIST)
		err := coll.UpdateOne(ctx, bson.M{"cid": cid}, bson.M{"$pull": bson.M{"black": blackcid}})
		if err != nil {
			util.Log_error("pull black err:%s", err.Error())
			return false
		}
		return true
	}).(bool)
	if res {
		m.ResponesJsonData(c, nil)
	} else {
		m.ResponesError(c, util.ERRCODE_ERROR, "服务器错误")
	}
}

type mg_black_list struct {
	Id    primitive.ObjectID `json:"_id" bson:"_id"`
	Cid   int64              `json:"cid" bson:"cid"`
	Black []int64            `json:"black" bson:"black"`
}

func (m *AppLoginModule) apiGetBlackList(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	var res mg_black_list
	m._mg_mgr.RequestFuncCallHash(uint32(cid), func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		coll := qc.Database.Collection(COLL_BLACK_LIST)
		coll.Find(ctx, bson.M{"cid": cid}).One(&res)
		return nil
	})
	m.ResponesJsonData(c, res.Black)
}

func getSearchPipline(search string, skip int, num int) bson.A {
	return bson.A{
		bson.M{"$match": bson.M{"$text": bson.M{"$search": search}}},
		bson.M{"$skip": skip},
		bson.M{"$limit": num},
		bson.M{"$lookup": bson.D{
			{Key: "from", Value: "task"},
			{Key: "localField", Value: "_id"},
			{Key: "foreignField", Value: "_id"},
			{Key: "as", Value: "result"},
		},
		},
		// bson.M{"$match": bson.M{"result": bson.M{"$ne": bson.A{}}}},
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
}

func (m *AppLoginModule) apiSearchTask(c *gin.Context) {
	var taskconf taskGetConfig
	err := c.ShouldBindJSON(&taskconf)
	if err != nil || len(taskconf.Search) == 0 {
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
		pipline := getSearchPipline(taskconf.Search, taskconf.Loc_limit, neednum)
		res := m._mg_mgr.RequestFuncCall(func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
			coll := qc.Database.Collection(COLL_TASK_LOCATION)
			err := coll.Aggregate(ctx, pipline).All(&taskResult.Data)
			if err != nil {
				util.Log_error("search task err:%s", err.Error())
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
		if getlen <= neednum {
			taskconf.LocMax = 1
		}
	}

	if taskconf.GlobelMax == 0 {
		var globeldata []mg_task
		pipline := getSearchPipline(taskconf.Search, taskconf.GlobelLimit, neednum)
		res := m._mg_mgr.RequestFuncCall(func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
			coll := qc.Database.Collection(COLL_TASK_GLOBEL)
			err := coll.Aggregate(ctx, pipline).All(&globeldata)
			if err != nil {
				util.Log_error("search task_globel err:%s", err.Error())
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
		if getlen <= neednum {
			taskconf.GlobelMax = 1
		}
	}
	m.ResponesJsonData(c, taskResult)
}

type mg_user_interest struct {
	Id   primitive.ObjectID   `json:"_id" bson:"_id"`
	Cid  int64                `json:"cid" bson:"cid"`
	Data []primitive.ObjectID `json:"tasklist" bson:"tasklist"`
}

type mg_array_size struct {
	ArraySize int `json:"arraysize" bson:"arraysize"`
}

func (m *AppLoginModule) apiTaskPushInterest(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	taskid := c.DefaultQuery("taskid", "")
	if len(taskid) == 0 {
		m.ResponesError(c, 1, "输入数据错误")
		return
	}

	objid := stringToObjectId(taskid)
	if objid == nil {
		m.ResponesError(c, 1, "输入数据错误")
		return
	}

	upopts := options.Update().SetUpsert(true)
	qmopt := qmoption.UpdateOptions{UpdateHook: nil, UpdateOptions: upopts}
	code := m._mg_mgr.RequestFuncCallHash(uint32(cid), func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		coll := qc.Database.Collection(COLL_USER_INTEREST)
		var arrsize mg_array_size
		coll.Find(ctx, bson.M{"cid": cid}).Select(bson.M{"arraysize": bson.M{"$size": "$tasklist"}}).One(&arrsize)
		if arrsize.ArraySize >= MAX_USER_INTEREST {
			return util.ERRCODE_MAX_USER_INTEREST
		}
		err := coll.UpdateOne(ctx, bson.M{"cid": cid}, bson.M{"$addToSet": bson.M{"tasklist": objid}}, qmopt)
		if err != nil {
			util.Log_error("push interest err:%s", err.Error())
			return util.ERRCODE_ERROR
		}
		return util.ERRCODE_SUCCESS
	}).(int)

	m.ResponesError(c, code, "")
}

func (m *AppLoginModule) apiTaskPullInterest(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	taskid := c.DefaultQuery("taskid", "")
	if len(taskid) == 0 {
		m.ResponesError(c, 1, "输入数据错误")
		return
	}

	objid := stringToObjectId(taskid)
	if objid == nil {
		m.ResponesError(c, 1, "输入数据错误")
		return
	}

	m._mg_mgr.RequestFuncCallNoResHash(uint32(cid), func(ctx context.Context, qc *qmgo.QmgoClient) {
		coll := qc.Database.Collection(COLL_USER_INTEREST)
		err := coll.UpdateOne(ctx, bson.M{"cid": cid}, bson.M{"$pull": bson.M{"tasklist": objid}})
		if err != nil {
			util.Log_error("pull interest err:%s", err.Error())
		}
	})
	m.ResponesJsonData(c, nil)
}

func (m *AppLoginModule) apiLoadInterest(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	var interest mg_user_interest
	m._mg_mgr.RequestFuncCall(func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		coll := qc.Database.Collection(COLL_USER_INTEREST)
		coll.Find(ctx, bson.M{"cid": cid}).One(&interest)
		return nil
	})
	m.ResponesJsonData(c, interest.Data)
}

func (m *AppLoginModule) apiLoadInterestTask(c *gin.Context) {
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
			{Key: "localField", Value: "tasklist"},
			{Key: "foreignField", Value: "_id"},
			{Key: "as", Value: "result"},
		},
		},
		// bson.M{"$unwind": "$result"},
		// bson.M{"$replaceRoot": bson.M{"newRoot": "$result"}},
		bson.M{"$lookup": bson.D{
			{Key: "from", Value: "task_join"},
			{Key: "localField", Value: "tasklist"},
			{Key: "foreignField", Value: "_id"},
			{Key: "as", Value: "task_join"},
		},
		},
		// bson.M{"$addFields": bson.M{"join": bson.M{"$arrayElemAt": bson.A{"$join", 0}}}},
	}

	var loadinfo loadTaskInterest
	m._mg_mgr.RequestFuncCall(func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		coll := qc.Database.Collection(COLL_USER_INTEREST)
		err := coll.Aggregate(ctx, pipline).One(&loadinfo)
		if err != nil {
			util.Log_error("load interest task err:%s", err.Error())
			return false
		}
		return true
	})
	m.ResponesJsonData(c, loadinfo)
}

func (m *AppLoginModule) apiSetUserIcon(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	iconurl := c.PostForm("icon")
	if len(iconurl) == 0 {
		m.ResponesError(c, 1, "输入数据错误")
		return
	}

	res := m._sql_mgr.RequestFuncCall(func(d *gorm.DB) interface{} {
		r := d.Model(&b_user{Cid: cid}).Update("icon", iconurl)
		if r.Error != nil {
			util.Log_error("apiSetIcon %s", r.Error.Error())
			return false
		}
		return true
	}).(bool)
	if res {
		m.ResponesError(c, util.ERRCODE_SUCCESS, "设置成功")
	} else {
		m.ResponesError(c, util.ERRCODE_ERROR, "设置失败")
	}
}

type mg_app_crash struct {
	Id    primitive.ObjectID `json:"_id,omitempty" bson:"_id"`
	Crash string             `json:"crash" bson:"crash"`
	Time  int64              `json:"time,omitempty" bson:"time"`
}

func (m *AppLoginModule) apiAppCrash(c *gin.Context) {
	info := c.PostForm("crash")

	crash := mg_app_crash{
		Id:    qmgo.NewObjectID(),
		Crash: info,
		Time:  util.GetSecond(),
	}

	m._mg_mgr.RequestFuncCallNoRes(func(ctx context.Context, qc *qmgo.QmgoClient) {
		coll := qc.Database.Collection(COLL_APP_CRASH)
		_, err := coll.InsertOne(ctx, crash)
		if err != nil {
			util.Log_error(err.Error())
		}
	})
}

func (m *AppLoginModule) apiAppError(c *gin.Context) {
	// info := c.PostForm("error")
	// info, err := c.GetRawData()
	var crash mg_app_crash
	err := c.ShouldBindJSON(&crash)
	if err != nil {
		util.Log_error("apiAppError:%s", err.Error())
		return
	}

	crash.Id = qmgo.NewObjectID()
	crash.Time = util.GetSecond()
	// crash := mg_app_crash{
	// 	Id:    ,
	// 	Crash: string(info),
	// }

	m._mg_mgr.RequestFuncCallNoRes(func(ctx context.Context, qc *qmgo.QmgoClient) {
		coll := qc.Database.Collection(COLL_APP_CRASH)
		_, err := coll.InsertOne(ctx, crash)
		if err != nil {
			util.Log_error(err.Error())
		}
	})
}

func (m *AppLoginModule) apiTaskCheck(c *gin.Context) {
	var strlist JsonStringList
	err := c.ShouldBindJSON(&strlist)
	if err != nil {
		util.Log_error("apiTaskCheck json err:%s", err.Error())
		m.ResponesError(c, util.ERRCODE_ERROR, "数据错误")
		return
	}

	for _, v := range strlist.Data {
		objid := stringToObjectId(v)
		if objid != nil {
			m._mg_mgr.RequestFuncCallNoRes(func(ctx context.Context, qc *qmgo.QmgoClient) {
				err := setTaskChecked(qc, ctx, *objid)
				if err != nil {
					util.Log_error("set taskcheck err:%s", err.Error())
				}
			})
		}
	}
	m.ResponesJsonData(c, nil)
}

func (m *AppLoginModule) apiTaskCheckAll(c *gin.Context) {
	var checks []mg_task_check
	m._mg_mgr.RequestFuncCall(func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		coll := qc.Database.Collection(COLL_TASK_CHECK)
		err := coll.Find(ctx, bson.M{}).All(&checks)
		if err != nil {
			util.Log_error("apiTaskCheckAll err:%s", err.Error())
		}
		return nil
	})

	for _, v := range checks {
		m._mg_mgr.RequestFuncCallNoRes(func(ctx context.Context, qc *qmgo.QmgoClient) {
			err := setTaskChecked(qc, ctx, v.Id)
			if err != nil {
				util.Log_error("set taskcheck id:%s err:%s", v.Id.Hex(), err.Error())
			}
		})
	}
	m.ResponesJsonData(c, checks)
}

func (m *AppLoginModule) apiLoadTaskChat(c *gin.Context) {
	taskid := c.DefaultQuery("taskid", "")
	startindex := util.StringToInt(c.DefaultQuery("start", "0"))
	num := util.StringToInt(c.DefaultQuery("num", "0"))
	if len(taskid) == 0 || num <= 0 || num > 20 {
		m.ResponesError(c, 1, "输入数据错误")
		return
	}

	objid := stringToObjectId(taskid)
	if objid == nil {
		m.ResponesError(c, 1, "输入数据错误")
		return
	}

	chatinfo := mg_task_chat{
		Id:    *objid,
		Count: -1,
	}
	err := m._mg_mgr.RequestFuncCall(func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		coll := qc.Database.Collection(COLL_TASK_CHAT)
		pip := getTaskChatPipline(*objid, startindex, num)
		err := coll.Aggregate(ctx, pip).One(&chatinfo)
		return err
	})
	if err != nil {
		util.Log_error("apiLoadTaskChat: %s", err.(error).Error())
	}
	m.ResponesJsonData(c, chatinfo)
}

func (m *AppLoginModule) apiLoadUserChatList(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	var chatlist []mg_chat_user_list

	pipline := bson.A{
		bson.M{"$match": bson.M{"$or": bson.A{bson.M{"cidhei": cid}, bson.M{"cidlow": cid}},
			"delete": bson.M{"$nin": bson.A{cid}}}},
		bson.M{"$set": bson.M{"data": bson.M{"$slice": bson.A{"$data", -1, 1}}}},
	}

	m._mg_mgr.RequestFuncCall(func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		coll := qc.Database.Collection(COLL_CHAT_USER_LIST)
		// err := coll.Find(ctx, bson.M{"$or": bson.A{bson.M{"cidhei": cid}, bson.M{"cidlow": cid}}}). .All(&chatlist)
		err := coll.Aggregate(ctx, pipline).All(&chatlist)
		if err != nil {
			util.Log_error("load UserChatList err:%s", err.Error())
		}
		return nil
	})

	m.ResponesJsonData(c, chatlist)
}

func (m *AppLoginModule) apiLoadUserChatData(c *gin.Context) {
	id := c.DefaultQuery("id", "")
	startindex := util.StringToInt(c.DefaultQuery("start", "0"))
	num := util.StringToInt(c.DefaultQuery("num", "0"))

	if len(id) == 0 || num <= 0 || num > 20 {
		m.ResponesError(c, 1, "输入数据错误")
		return
	}

	objid := stringToObjectId(id)
	if objid == nil {
		m.ResponesError(c, 1, "输入数据错误")
		return
	}

	chatlist := mg_chat_user_list{
		Id:    *objid,
		Count: -1,
	}
	err := m._mg_mgr.RequestFuncCall(func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		coll := qc.Database.Collection(COLL_CHAT_USER_LIST)
		pip := getTaskChatPipline(*objid, startindex, num)
		err := coll.Aggregate(ctx, pip).One(&chatlist)
		return err
	})
	if err != nil {
		util.Log_error("apiLoadUserChatData err: %s", err.(error).Error())
	}
	m.ResponesJsonData(c, chatlist)
}

func (m *AppLoginModule) apiLoadOneUserChatData(c *gin.Context) {
	tocid := util.StringToInt64(c.DefaultQuery("cid", "0"))
	cid := c.MustGet("userId").(int64)
	if tocid <= 0 {
		m.ResponesJsonData(c, nil)
		return
	}

	cidhei := tocid
	cidlow := cid
	if cid > tocid {
		cidhei = cid
		cidlow = tocid
	}

	pip := bson.A{
		bson.M{"$match": bson.M{"cidhei": cidhei, "cidlow": cidlow}},
		bson.M{"$addFields": bson.M{"data": bson.M{"$slice": bson.A{
			"$data",
			-20,
			20,
		}},
		}}}
	var chatlist mg_chat_user_list
	err := m._mg_mgr.RequestFuncCall(func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		coll := qc.Database.Collection(COLL_CHAT_USER_LIST)
		err := coll.Aggregate(ctx, pip).One(&chatlist)
		return err
	})
	if err != nil {
		util.Log_error("apiLoadUserChatData err: %s", err.(error).Error())
	}
	m.ResponesJsonData(c, chatlist)
}

func (m *AppLoginModule) apiDeleteUserChatData(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	id := c.DefaultQuery("id", "")

	objid := stringToObjectId(id)
	if objid == nil {
		m.ResponesError(c, 1, "输入数据错误")
		return
	}

	upopts := options.Update().SetUpsert(true)
	qmopt := qmoption.UpdateOptions{UpdateHook: nil, UpdateOptions: upopts}
	m._mg_mgr.RequestFuncCallNoRes(func(ctx context.Context, qc *qmgo.QmgoClient) {
		coll := qc.Database.Collection(COLL_CHAT_USER_LIST)
		coll.UpdateOne(ctx, bson.M{"_id": objid}, bson.M{"$addToSet": bson.M{"delete": cid}}, qmopt)
	})
	m.ResponesJsonData(c, nil)
}

type b_user_credit_judge struct {
	Cid     int64 `gorm:"primaryKey" json:"cid"`
	FromCid int64 `gorm:"primaryKey;column:fromcid" json:"fromcid"`
	Ctype   int   `json:"ctype"`
	Time    int64 `json:"time"`
}

// 获取对某个用户的评价类型
func (m *AppLoginModule) apiGetCreditToUserType(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	tocid := util.StringToInt64(c.DefaultQuery("cid", "0"))
	if tocid <= 0 {
		m.ResponesError(c, 1, "输入数据错误")
		return
	}

	var bcredit b_user_credit_judge
	m._sql_mgr.RequestFuncCallHash(uint32(tocid), func(d *gorm.DB) interface{} {
		res := d.Table("b_user_credit_judge").Find(&bcredit, "cid = ? AND fromcid = ?", tocid, cid)
		if res.Error != nil {
			util.Log_error("get credit to user sql err:%s", res.Error.Error())
		}
		return nil
	})
	m.ResponesJsonData(c, bcredit)
}

// 评价某个用户
func (m *AppLoginModule) apiSetCreditToUserType(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	tocid := util.StringToInt64(c.DefaultQuery("cid", "0"))
	ctype := util.StringToInt(c.DefaultQuery("type", "0"))
	if tocid <= 0 || ctype < -1 || ctype > 1 {
		m.ResponesError(c, 1, "输入数据错误")
		return
	}

	now := util.GetSecond()
	m._sql_mgr.RequestFuncCallNoResHash(uint32(tocid), func(d *gorm.DB) { //interface{}
		// 事务
		var bcredit b_user_credit_judge
		res := d.Table("b_user_credit_judge").Find(&bcredit, "cid = ? AND fromcid = ?", tocid, cid)
		if res.Error != nil {
			util.Log_error("set credit type sql err1:%s", res.Error.Error())
		}
		if bcredit.Ctype == int(ctype) {
			return
		}
		// new - old
		score := ctype - bcredit.Ctype
		d.Transaction(func(tx *gorm.DB) error {
			var res *gorm.DB
			if ctype == 0 {
				res = d.Exec("DELETE from `b_user_credit_judge` WHERE `cid`=? AND `fromcid`=?;", tocid, cid)
			} else {
				res = d.Exec("INSERT INTO `b_user_credit_judge` VALUES(?,?,?,?) ON DUPLICATE KEY UPDATE `ctype`=?,`time`=?;", tocid, cid, ctype, now, ctype, now)
			}
			if res.Error != nil {
				util.Log_error("update b_user_credit_judge err: %s", res.Error.Error())
				return res.Error
			}

			res = tx.Model(&b_user{Cid: tocid}).Update("credit_score", gorm.Expr("credit_score + ?", score))
			if res.Error != nil {
				util.Log_error("set credit type sql err2: %s", res.Error.Error())
				return res.Error
			}
			return nil
		})
		// return nil
	})
	m.ResponesJsonData(c, nil)
}

type b_user_idcard struct {
	Cid    int64  `gorm:"primaryKey" json:"cid"`
	IdCard string `gorm:"primaryKey;column:id_card"`
	Name   string
}

func getAgeFromID(id string) int {
	birthday := id[6:14] // 从身份证号码中截取出生日期部分
	year, _ := strconv.Atoi(birthday[0:4])
	month, _ := strconv.Atoi(birthday[4:6])
	day, _ := strconv.Atoi(birthday[6:8])

	today := time.Now()
	birthDate := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)

	age := today.Year() - birthDate.Year()

	// 如果生日大于当前日期的生日，则年龄减1
	if today.Month() < birthDate.Month() || (today.Month() == birthDate.Month() && today.Day() < birthDate.Day()) {
		age--
	}
	return age
}

func getGender(idNumber string) int {
	// 通过身份证号的倒数第二位数字判断性别，偶数为女性，奇数为男性
	genderDigit, _ := strconv.Atoi(string(idNumber[len(idNumber)-2]))
	if genderDigit%2 == 0 {
		return util.SEX_WOMAN
	} else {
		return util.SEX_MAN
	}
}

// 实名验证
func (m *AppLoginModule) apiCheckIdCard(c *gin.Context) {
	cid := c.MustGet("userId").(int64)
	idcard := c.DefaultQuery("idcard", "")
	name := c.DefaultQuery("name", "")

	if len(idcard) == 0 || len(name) == 0 {
		m.ResponesError(c, 1, "输入数据错误")
		return
	}
	// 验证

	// 入库
	age := getAgeFromID(idcard)
	sex := getGender(idcard)
	m._sql_mgr.RequestFuncCallNoResHash(uint32(cid), func(d *gorm.DB) {
		res := d.Table("b_user_idcard").Create(&b_user_idcard{Cid: cid, IdCard: idcard, Name: name})
		if res.Error != nil {
			util.Log_error("insert idcard err:%s", res.Error.Error())
		}
		// age,sex
		res = d.Model(&b_user{Cid: cid}).Updates(map[string]interface{}{"age": age, "sex": sex})
		if res.Error != nil {
			util.Log_error("update user age err:%s", res.Error.Error())
		}
	})
	m.ResponesJsonData(c, nil)
}

func (m *AppLoginModule) apiGetOssCredential(c *gin.Context) {
	url := util.GetConfValue("nodeServerHost") + "getOssCredential"
	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		util.Log_error("apiGetOssCredential step 1 err:%s", err.Error())
		ResponesCommonError(c, "服务器错误")
		return
	}

	msg, err := io.ReadAll(resp.Body)
	if err != nil {
		util.Log_error("apiGetOssCredential step 2 err:%s", err.Error())
		ResponesCommonError(c, "服务器错误")
		return
	}
	c.String(http.StatusOK, string(msg))
}

func (m *AppLoginModule) apiGetPostPolicy(c *gin.Context) {
	ext := c.DefaultQuery("ext", "")
	if len(ext) == 0 {
		ResponesCommonError(c, "参数错误")
		return
	}
	url := util.GetConfValue("nodeServerHost") + "getPostPolicy?ext=" + ext
	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		util.Log_error("apiGetPostPolicy step 1 err:%s", err.Error())
		ResponesCommonError(c, "服务器错误")
		return
	}

	msg, err := io.ReadAll(resp.Body)
	if err != nil {
		util.Log_error("apiGetPostPolicy step 2 err:%s", err.Error())
		ResponesCommonError(c, "服务器错误")
		return
	}
	c.String(http.StatusOK, string(msg))
}

func (m *AppLoginModule) checkTask(task *mg_task) {
	go func() {
		url := util.GetConfValue("nodeServerHost") + "checkTask"
		pdata, err := json.Marshal(task)
		if err != nil {
			util.Log_error("checkTask step 1 err:%s", err.Error())
			return
		}

		req, err := http.NewRequest("POST", url, bytes.NewReader(pdata))
		if err != nil {
			util.Log_error("checkTask step 2 err:%s", err.Error())
			return
		}
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		_, err = http.DefaultClient.Do(req)
		if err != nil {
			util.Log_error("checkTask step 3 err:%s", err.Error())
		}
	}()
}
