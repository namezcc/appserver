package module

import (
	"context"
	"goserver/util"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/qiniu/qmgo"
	"go.mongodb.org/mongo-driver/bson"
	"gorm.io/gorm"
)

func ResponesJsonData(c *gin.Context, d interface{}) {
	jd := jsonBase{
		Code: util.ERRCODE_SUCCESS,
		Data: d,
	}
	c.JSON(http.StatusOK, jd)
}

func ResponesJsonBase(c *gin.Context, d jsonBase) {
	c.JSON(http.StatusOK, d)
}

func ResponesError(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, gin.H{"code": code, "msg": msg})
}

func ResponesCommonError(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, gin.H{"code": util.USER_HTTP_ERR_ERROR, "msg": msg})
}

type UserManagerModule struct {
	HttpModule
	_safe_req map[string]bool
	_sql_mgr  *MysqlManagerModule
	_mg_mgr   *MongoManagerModule
}

func (m *UserManagerModule) Init(mgr *moduleMgr) {
	m.HttpModule.Init(mgr)
	m._httpbase = m
	m.SetHost(":" + util.GetConfValue("userManagerPort"))
	m.SetMysqlHots("mysqluser")
	m._safe_req = make(map[string]bool)
	m._sql_mgr = mgr.GetModule(MOD_MYSQL_MGR).(*MysqlManagerModule)
	m._mg_mgr = mgr.GetModule(MOD_MONGO_MGR).(*MongoManagerModule)
}

func (m *UserManagerModule) BeforRun() {
	m.HttpModule.BeforRun()

}

func (m *UserManagerModule) getMiddlew() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, ok := m._safe_req[c.FullPath()]
		if ok {
			c.Next()
			return
		}

		checkUser, claim := Verify(c) //第二个值是用户名，这里没有使用
		if checkUser == false {
			c.JSON(401, gin.H{"code": 0, "msg": "身份认证失败"})
			c.Abort()
			return
		} else {
			c.Set("userId", claim.UserID)
			c.Set("userName", claim.Username)
		}
		c.Next()
	}
}

func (m *UserManagerModule) safeGet(r *gin.Engine, p string, h gin.HandlerFunc) {
	m._safe_req[p] = true
	r.GET(p, h)
}

func (m *UserManagerModule) safePost(r *gin.Engine, p string, h gin.HandlerFunc) {
	m._safe_req[p] = true
	r.POST(p, h)
}

func (m *UserManagerModule) initRoter(r *gin.Engine) {
	r.Use(m.getMiddlew())

	m.safePost(r, "/apiUserLogin", m.apiUserLogin)
	r.GET("/apiRefreshToken", m.apiRefreshToken)
	r.GET("/apiGetUserInfo", m.apiGetUserInfo)
	r.GET("/apiLoadUserTsak", m.apiLoadUserTsak)
	r.GET("/apiRemoveTsak", m.apiRemoveTsak)
	r.GET("/apiLoadReportTsak", m.apiLoadReportTsak)
	r.GET("/apiLoadReportUser", m.apiLoadReportUser)
	r.GET("/apiLoadOneTask", m.apiLoadOneTask)

}

func (m *UserManagerModule) apiUserLogin(c *gin.Context) {
	var info userInfo
	err := c.ShouldBindJSON(&info)
	if err != nil {
		ResponesError(c, util.USER_HTTP_ERR_ERROR, "数据错误")
		return
	}

	var dbuser dbAdmin
	m.doSql(func(d *sqlx.DB) {
		err = m._sql.QueryRow("SELECT * FROM `user` WHERE `name`=?;", info.Name).Scan(&dbuser.Uid, &dbuser.Name, &dbuser.Pass)
	})

	if err != nil {
		util.Log_error(err.Error())
		ResponesError(c, util.USER_HTTP_ERR_ERROR, err.Error())
		return
	}

	passmd5 := util.StringMd5(dbuser.Pass)
	if info.Pass == passmd5 {
		claims := &JWTClaims{
			UserID:      dbuser.Uid,
			Username:    info.Name,
			Password:    info.Pass,
			FullName:    info.Name,
			Permissions: []string{},
		}
		claims.IssuedAt = time.Now().Unix()
		claims.ExpiresAt = time.Now().Add(time.Second * time.Duration(ExpireTime)).Unix()
		signedToken, err := getToken(claims)
		if err != nil {
			ResponesError(c, util.USER_HTTP_ERR_ERROR, err.Error())
			return
		}

		claims.ExpiresAt = time.Now().Add(time.Second * time.Duration(ExpireRefreshTime)).Unix()
		refreshToken, err := getToken(claims)
		if err != nil {
			ResponesError(c, util.USER_HTTP_ERR_ERROR, err.Error())
			return
		}

		ResponesJsonData(c, gin.H{"token": signedToken, "refreshToken": refreshToken, "expireTime": ExpireTime, "uid": dbuser.Uid, "name": dbuser.Name})
	} else {
		ResponesError(c, util.USER_HTTP_ERR_ERROR, "密码错误")
	}
}

func (m *UserManagerModule) apiRefreshToken(c *gin.Context) {
	signToken, refreshToken := Refresh(c)
	if len(signToken) == 0 {
		ResponesError(c, util.USER_HTTP_ERR_ERROR, "刷新失败")
	} else {
		ResponesJsonData(c, gin.H{"token": signToken, "refreshToken": refreshToken, "expireTime": ExpireTime})
	}
}

func (m *UserManagerModule) apiGetUserInfo(c *gin.Context) {
	uid := util.StringToInt64(c.DefaultQuery("uid", "0"))
	if uid <= 0 {
		ResponesError(c, util.USER_HTTP_ERR_ERROR, "数据错误")
		return
	}

	var buser b_user
	m._sql_mgr.RequestFuncCallNoRes(func(d *gorm.DB) {
		res := d.Find(&buser, "cid = ?", uid)
		if res.Error != nil {
			util.Log_error("getuserinfo cid:%d %s", uid, res.Error.Error())
		}
	})

	ResponesJsonData(c, buser)
}

func (m *UserManagerModule) apiLoadUserTsak(c *gin.Context) {
	uid := util.StringToInt64(c.DefaultQuery("uid", "0"))
	skip := util.StringToInt(c.DefaultQuery("skip", "0"))
	if uid <= 0 {
		ResponesError(c, util.USER_HTTP_ERR_ERROR, "数据错误")
		return
	}

	neednum := 20
	pipline := taskListToTaskPipline(uid, skip, neednum)

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
	ResponesJsonData(c, loadinfo)
}

// 下架任务
func (m *UserManagerModule) apiRemoveTsak(c *gin.Context) {
	taskid := c.DefaultQuery("taskid", "")
	if taskid == "" {
		ResponesError(c, 1, "数据错误")
		return
	}
	objid := stringToObjectId(taskid)
	if objid == nil {
		ResponesError(c, 1, "数据错误")
		return
	}

	res := m._mg_mgr.RequestFuncCall(func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		coll := qc.Database.Collection(COLL_TASK)
		err := coll.UpdateId(ctx, objid, bson.M{"$set": bson.M{"state": util.TASK_STATE_ILLEGAL}})
		if err != nil {
			util.Log_error("removetask err: %s", err.Error())
			return false
		}
		loc_coll := qc.Database.Collection(COLL_TASK_LOCATION)
		glo_coll := qc.Database.Collection(COLL_TASK_GLOBEL)
		// 删除 location
		loc_coll.RemoveId(ctx, objid)
		// 删除 globel
		glo_coll.RemoveId(ctx, objid)
		// 删除 审核
		collcheck := qc.Database.Collection(COLL_TASK_CHECK)
		collcheck.RemoveId(ctx, objid)
		return true
	}).(bool)
	if res {
		ResponesError(c, util.USER_HTTP_ERR_SUCCESS, "下架失败")
	} else {
		ResponesError(c, util.USER_HTTP_ERR_ERROR, "")
	}
}

func (m *UserManagerModule) apiLoadReportTsak(c *gin.Context) {
	skip := util.StringToInt(c.DefaultQuery("skip", "0"))

	var list []mg_report_task

	m._mg_mgr.RequestFuncCallNoRes(func(ctx context.Context, qc *qmgo.QmgoClient) {
		coll := qc.Database.Collection(COLL_REPORT_TASK)
		err := coll.Find(ctx, bson.M{}).Skip(int64(skip)).Limit(20).All(&list)
		if err != nil {
			util.Log_error("load report task err:%s", err.Error())
		}
	})

	ResponesJsonData(c, list)
}

func (m *UserManagerModule) apiLoadReportUser(c *gin.Context) {
	skip := util.StringToInt(c.DefaultQuery("skip", "0"))

	var list []mg_report_user

	m._mg_mgr.RequestFuncCallNoRes(func(ctx context.Context, qc *qmgo.QmgoClient) {
		coll := qc.Database.Collection(COLL_REPORT_USER)
		err := coll.Find(ctx, bson.M{}).Skip(int64(skip)).Limit(20).All(&list)
		if err != nil {
			util.Log_error("load report user err:%s", err.Error())
		}
	})

	ResponesJsonData(c, list)
}

func (m *UserManagerModule) apiLoadOneTask(c *gin.Context) {
	taskid := c.DefaultQuery("taskid", "")
	if taskid == "" {
		ResponesError(c, 1, "数据错误")
		return
	}
	objid := stringToObjectId(taskid)
	if objid == nil {
		ResponesError(c, 1, "数据错误")
		return
	}
	pipline := getTaskWithJoinPipline(*objid)
	res := m._mg_mgr.RequestFuncCall(func(ctx context.Context, qc *qmgo.QmgoClient) interface{} {
		task, err := getTaskByPipline(qc, ctx, pipline)
		if err != nil {
			return nil
		}
		return task
	})
	if res == nil {
		ResponesError(c, 1, "数据错误")
	} else {
		ResponesJsonData(c, res.(*mg_task))
	}
}
