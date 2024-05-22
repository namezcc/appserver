const { GetConfValue } = require("./util_conf")
const { log_error, log_info } = require("./util_log")
const { getNowSecond } = require("./util_time")
const axios = require("axios").default

var access_token = {
	access_token:"",
	expires_time:0
}
var appid = ""
var secret = ""

function initWxData() {
	appid = GetConfValue("appid")
	secret = GetConfValue("app_secret")
}

async function getWxAccessToken() {
	let url = "https://api.weixin.qq.com/cgi-bin/stable_token"
	let rep = await axios.post(url,{
		grant_type:"client_credential",
		appid:appid,
		secret:secret
	})
	return rep?.data
}

async function getWxPhoneNumber(code) {
	let url = "https://api.weixin.qq.com/wxa/business/getuserphonenumber?access_token="+access_token.access_token
	let rep = await axios.post(url,{
		code:code
	})
	let res = {
		errcode:0,
		errmsg:"error",
		phoneNumber:"",
	}
	if (rep?.data) {
		res.errcode = rep.data.errcode
		res.errmsg = rep.data.errmsg
		res.phoneNumber = rep.data?.phone_info?.phoneNumber
	}
	return res
}

async function startAccessTokenJob() {
	initWxData()
	let res = await getWxAccessToken()
	if (res == null || res.errcode != null) {
		console.error("getWxAccessToken fail",res)
		process.exit(1)
	}

	log_info("get accesstoen success",res)
	// 过期前4分钟请求
	access_token.expires_time = getNowSecond() + res.expires_in - 4 * 60
	access_token.access_token = res.access_token
	setInterval(() => {
		let now = getNowSecond()
		if (now < access_token.expires_time) {
			return
		}
		res = getWxAccessToken()
		if (res?.expires_in) {
			access_token.expires_time = getNowSecond() + res.expires_in - 4 * 60
			access_token.access_token = res.access_token
		}else if (res?.errcode) {
			log_error("getWxAccessToken fail errcode:%d msg:%s",res.errcode,res.errmsg)
		}
	},5000)
	return true
}

function getAccessToken() {
	return access_token
}

module.exports = {
	startAccessTokenJob,
	getAccessToken,
	getWxAccessToken,
	getWxPhoneNumber,
}