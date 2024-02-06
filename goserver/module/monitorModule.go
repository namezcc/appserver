package module

import (
	"bytes"
	"encoding/json"
	"goserver/handle"
	"goserver/network"
	"goserver/util"
	"hash/crc32"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
)

type luaerrs struct {
	num  int
	time int64
	text string
}

type MonitorModule struct {
	ServerModule
	_luaerr  map[uint32]*luaerrs
	_dingurl string
}

func (m *MonitorModule) Init(mgr *moduleMgr) {
	m.ServerModule.Init(mgr)
	m._luaerr = make(map[uint32]*luaerrs)

	handle.Handlemsg.AddMsgCall(handle.N_MASTER_LUA_ERROR, m.onErrorLog)
	handle.Handlemsg.AddMsgCall(handle.N_MASTER_BATTLE_DATA, m.onSaveBattleData)

}

func (m *MonitorModule) AfterInit() {
	m.ServerModule.AfterInit()
	m._dingurl = util.GetConfValue("luaerrorhost")
	if !m.getConfigFromHttp(util.ST_MONITOR, util.GetSelfServerId()) {
		os.Exit(1)
	}
}

type pding struct {
	Msgtype string            `json:"msgtype"`
	Text    map[string]string `json:"text"`
}

func (m *MonitorModule) onErrorLog(msg *handle.BaseMsg) {
	p := msg.Data.(*network.Msgpack)
	str := p.ReadString()
	pam := p.ReadString()
	hash32 := crc32.ChecksumIEEE(str)

	lerr, ok := m._luaerr[hash32]

	if ok {
		lerr.num++
		if lerr.time > util.GetSecond() {
			return
		}

		lerr.time = util.GetSecond() + 10*60
	} else {
		lerr = &luaerrs{
			text: string(str),
			num:  1,
			time: util.GetSecond() + 10*60,
		}
		m._luaerr[hash32] = lerr
	}

	util.Log_info("hash:%d num:%d %s", hash32, lerr.num, string(str))

	cont := "luaerror:" + strconv.Itoa(lerr.num) + ":" + string(str) + string(pam)

	pjs := pding{}

	pjs.Msgtype = "text"
	pjs.Text = make(map[string]string)

	pjs.Text["content"] = cont

	js, jerr := json.Marshal(pjs)
	if jerr != nil {
		util.Log_error(jerr.Error())
		return
	}

	req, _ := http.NewRequest("POST", m._dingurl, bytes.NewReader(js))

	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		util.Log_error(err.Error())
	} else {
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			util.Log_error(err.Error())
		} else {
			util.Log_info(string(b))
		}
	}
}

func (m *MonitorModule) onSaveBattleData(msg *handle.BaseMsg) {
	p := msg.Data.(*network.Msgpack)

	serid := p.ReadInt32()
	ntime := p.ReadInt32()
	iscomp := p.ReadInt32()
	orglen := p.ReadInt32()
	buff := p.ReadString()

	_, err := m._sql.Exec("INSERT INTO battledata(`serverid`,`time`,`compress`,`orglen`,`data`) VALUES(?,?,?,?,?);", serid, ntime, iscomp, orglen, buff)
	if err != nil {
		util.Log_error(err.Error())
	}
}
