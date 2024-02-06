package module

import (
	"goserver/handle"
	"goserver/network"
	"goserver/util"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	BAN_UID = iota
	BAN_IP
	BAN_IP_UNLIMIT
)

type OperateHttpModule struct {
	HttpModule
	_server_mod *OperateModule
}

func (m *OperateHttpModule) Init(mgr *moduleMgr) {
	m.HttpModule.Init(mgr)
	m._httpbase = m
	m._server_mod = mgr.GetModule(MOD_OPERATE).(*OperateModule)
	m.SetHost(":" + util.GetConfValue("operateport"))
}

func (m *OperateHttpModule) initRoter(r *gin.Engine) {
	r.GET("/banInfo", m.onBanInfo)
}

func (m *OperateHttpModule) onBanInfo(c *gin.Context) {
	btype, err := strconv.Atoi(c.Query("type"))
	if err != nil {
		util.Log_error(err.Error())
		return
	}

	isadd, err := strconv.Atoi(c.Query("add"))
	if err != nil {
		util.Log_error(err.Error())
		return
	}

	pack := network.NewMsgPackDef()
	pack.WriteInt32(btype)
	pack.WriteInt32(isadd)

	switch btype {
	case BAN_UID:
		etime, err := strconv.Atoi(c.Query("time"))
		if err != nil {
			util.Log_error(err.Error())
			return
		}
		uid := c.Query("uid")
		pack.WriteRealString(uid)
		pack.WriteInt32(etime)
	case BAN_IP:
		ip := c.Query("ip")
		pack.WriteRealString(ip)
	case BAN_IP_UNLIMIT:
		ip := c.Query("ip")
		ipend := c.Query("ipend")
		pack.WriteRealString(ip)
		pack.WriteRealString(ipend)
	default:
		return
	}
	m._server_mod.sendMsg(handle.M_SEND_BAN_INFO, pack)
	c.String(http.StatusOK, "")
}
