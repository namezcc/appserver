// import Koa from "koa";
const Koa  = require("koa")
const Router = require("koa-router")
const bodyParser = require('koa-bodyparser');

const { ReadConf, GetConfValue } = require("./util/util_conf")
const { log_error } = require("./util/util_log")
const { getAccessToken, startAccessTokenJob, getWxPhoneNumber } = require("./util/util_wx")

ReadConf("conf.ini")
var port = GetConfValue("nodeServerPort")

startAccessTokenJob()

var app = new Koa()
var router = new Router()

router.get("/accessToken",(ctx) => {
	ctx.body = getAccessToken()
})

router.post("/getPhoneNumber",async (ctx) => {
	let data = ctx.request.body
	if (data?.code) {
		ctx.body = await getWxPhoneNumber(data.code)
	}else{
		ctx.body = {
			errmsg:"data error"
		}
	}
})

app.use(bodyParser())
app.use(router.routes()).use(router.allowedMethods())

app.on("error",(err) => {
	log_error("server error",err)
})

app.listen(port,() => {
	console.log("start listen %d",port)
})