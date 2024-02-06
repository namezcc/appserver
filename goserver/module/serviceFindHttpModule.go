package module

import (
	"goserver/handle"
	"goserver/util"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type ServiceFindHttpModule struct {
	HttpModule
}

func (m *ServiceFindHttpModule) Init(mgr *moduleMgr) {
	m.HttpModule.Init(mgr)
	m._httpbase = m

	handle.Handlemsg.AddMsgCall(handle.M_GET_SERVER_CONFIG, m.onGetServerConf)
}

func (m *ServiceFindHttpModule) initRoter(r *gin.Engine) {
	r.GET("/serverInfo", m.getServerConf)
}

func (m *ServiceFindHttpModule) getServerConf(c *gin.Context) {
	id, _ := strconv.Atoi(c.Query("id"))
	type_, _ := strconv.Atoi(c.Query("type"))
	qd := make([]int, 0)
	qd = append(qd, type_, id)

	rep := m.requestMsg(&m.modulebase, handle.M_GET_SERVER_CONFIG, qd)
	if rep == nil || rep.data == nil {
		c.String(http.StatusOK, "")
		return
	}
	serviceinfo := rep.data.(*ServiceInfo)
	c.JSON(http.StatusOK, *serviceinfo)
}

func (m *ServiceFindHttpModule) checkDbidSameHost(vec []string) *dbMysqlHost {
	var dbhost, checkhost dbMysqlHost
	for i, v := range vec {
		if i == 0 {
			err := m._sql.Get(&dbhost, "SELECT * FROM `mysql_host` WHERE `id`=?;", v)
			if err != nil {
				util.Log_error(err.Error())
				return nil
			}
		} else {
			err := m._sql.Get(&checkhost, "SELECT * FROM `mysql_host` WHERE `id`=?;", v)
			if err != nil {
				util.Log_error(err.Error())
				return nil
			}

			if dbhost.Ip != checkhost.Ip ||
				dbhost.Port != checkhost.Port ||
				dbhost.Dbname != checkhost.Dbname {
				return nil
			}
		}
	}
	return &dbhost
}

func (m *ServiceFindHttpModule) getMysqlDbidFromGroup(group int) []int {
	var servec []dbServer
	err := m._sql.Select(&servec, "SELECT * FROM `server` WHERE type=? AND `group`=?;", util.ST_MYSQL, group)
	if err != nil {
		util.Log_error(err.Error())
		return nil
	}
	res := make([]int, 0)
	for _, v := range servec {
		ids := util.SplitIntArray(v.Mysql, ",")
		res = append(res, ids...)
	}
	return res
}

func (m *ServiceFindHttpModule) onGetServerConf(msg *handle.BaseMsg) {
	rep := msg.Data.(*responesData)
	defer m.responesMsgData(rep)

	qd := rep.data.([]int)
	rep.data = nil

	var serviceinfo ServiceInfo
	err := m._sql.Get(&serviceinfo.Server, "SELECT * FROM `server` WHERE type=? AND id=?;", qd[0], qd[1])
	if err != nil {
		util.Log_error(err.Error())
		return
	}

	if serviceinfo.Server.Type == util.ST_LOGIN {
		var loginvec []dbLogin
		err = m._sql.Select(&loginvec, "SELECT * FROM `login_list` WHERE login=?;", serviceinfo.Server.Id)
		if err != nil {
			util.Log_error(err.Error())
			return
		}
		if len(loginvec) == 0 {
			return
		}
		for _, v := range loginvec {
			serviceinfo.Server.Uidgroup = append(serviceinfo.Server.Uidgroup, v.Uidgroup)
		}
		// 获取本组的所有dbid
		serviceinfo.Server.Dbidlist = m.getMysqlDbidFromGroup(serviceinfo.Server.Group)
	}

	if serviceinfo.Server.Mysql != "" {
		dbids := strings.Split(serviceinfo.Server.Mysql, ",")
		dbhost := m.checkDbidSameHost(dbids)
		if dbhost == nil {
			util.Log_error("dbid check error not same host sertype:%d serid:%d", qd[0], qd[1])
			return
		}
		serviceinfo.Server.MysqlHost = *dbhost
	}

	err = m._sql.Select(&serviceinfo.Service, "SELECT * FROM `service_find`;")
	if err != nil {
		util.Log_error(err.Error())
		return
	}
	rep.data = &serviceinfo
}
