const { default: axios } = require("axios");
const { ReadConf } = require("../util/util_conf");
const { log_info } = require("../util/util_log");
const { getWxAccessToken } = require("../util/util_wx");

ReadConf("conf.ini")

async function test_wxAccessToken() {
	let res = await getWxAccessToken()
	console.log(res)
}

function test_log() {
	log_info("info %s %d","aaa",1000)
}

function test_get_accessToken() {
	axios.get("http://127.0.0.1:8882/accessToken").then((res) => {
		console.log(res.data)
	})
}

// test_wxAccessToken()
// test_log()