// import Koa from "koa";
const Koa  = require("koa")
const Router = require("koa-router")
const bodyParser = require('koa-bodyparser');

const { ReadConf, GetConfValue } = require("./util/util_conf")
const { log_error, log_info } = require("./util/util_log")
const { getAccessToken, startAccessTokenJob, getWxPhoneNumber, startOssCredentialJob, getOssGredent, getPostPolicy } = require("./util/util_wx");
const { initClient } = require("./util/util_mongo");
const { startCheckTaskJob, pushTask } = require("./util/util_task_check");

ReadConf("conf.ini")
var port = GetConfValue("nodeServerPort")

// mogo client
initClient()

startAccessTokenJob()
startOssCredentialJob()
startCheckTaskJob()


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

router.get("/getOssCredential",(ctx) => {
	ctx.body = getOssGredent()
})

router.get("/getPostPolicy",(ctx) => {
	ctx.body = getPostPolicy(ctx.query.ext)
})

router.post("/checkTask",(ctx) => {
	let data = ctx.request.body
	
	log_info("check task:",data)
	pushTask(data)

	ctx.body = ""
})

app.use(bodyParser())
app.use(router.routes()).use(router.allowedMethods())

app.on("error",(err) => {
	log_error("server error",err)
})

app.listen(port,() => {
	console.log("start listen %d",port)
})