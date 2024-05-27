const { default: axios } = require("axios");
const { ReadConf, GetConfValue } = require("../util/util_conf");
const { log_info } = require("../util/util_log");
const { getWxAccessToken } = require("../util/util_wx");
const STS = require("qcloud-cos-sts")
var COS = require('cos-nodejs-sdk-v5');
var MongoClient = require('mongodb').MongoClient;
const mongoose = require('mongoose');
const { testreq } = require("./test_req");
const { testreq2 } = require("./test_req2");
const { initClient, addTaskIllegal, closeMongoClient } = require("../util/util_mongo");
const { LinkedList } = require("../util/nodelist");

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

function test_sts() {
	// 配置参数
var config = {
	secretId: "AKIDVvGTbv7A7sV0Nm8WI4WpGjEjL0UEtQ2W",   // 固定密钥
	secretKey: "TpHj6Nep0GSgqvuhGkKlz42UtALQI2e5",  // 固定密钥
	proxy: '',
	host: 'sts.tencentcloudapi.com', // 域名，非必须，默认为 sts.tencentcloudapi.com
	// endpoint: 'sts.internal.tencentcloudapi.com', // 域名，非必须，与host二选一，默认为 sts.tencentcloudapi.com
	durationSeconds: 1800,  // 密钥有效期
	// 放行判断相关参数
	bucket: 'bangbang1-1326763244', // 换成你的 bucket
	region: 'ap-shanghai', // 换成 bucket 所在地区
	allowPrefix: 'bangbang' // 这里改成允许的路径前缀，可以根据自己网站的用户登录态判断允许上传的具体路径，例子： a.jpg 或者 a/* 或者 * (使用通配符*存在重大安全风险, 请谨慎评估使用)
  };
  
  
  var shortBucketName = config.bucket.substr(0, config.bucket.lastIndexOf('-'));
  var appId = config.bucket.substr(1 + config.bucket.lastIndexOf('-'));
  var policy = {
	'version': '2.0',
	'statement': [{
	  'action': [
		// 简单上传
		'name/cos:PutObject',
		'name/cos:PostObject',
		// 分片上传
		'name/cos:InitiateMultipartUpload',
		'name/cos:ListMultipartUploads',
		'name/cos:ListParts',
		'name/cos:UploadPart',
		'name/cos:CompleteMultipartUpload',
		// 简单上传和分片，需要以上权限，其他权限列表请看 https://cloud.tencent.com/document/product/436/31923
  
		// 文本审核任务
		// 'name/ci:CreateAuditingTextJob',
		// 开通媒体处理服务
		// 'name/ci:CreateMediaBucket'
		// 更多数据万象授权可参考：https://cloud.tencent.com/document/product/460/41741
	  ],
	  'effect': 'allow',
	  'principal': { 'qcs': ['*'] },
	  'resource': [
		// cos相关授权，按需使用
		'qcs::cos:' + config.region + ':uid/' + appId + ':' + config.bucket + '/' + config.allowPrefix,
		// ci相关授权，按需使用
		'qcs::ci:' + config.region + ':uid/' + appId + ':bucket/' + config.bucket + '/*',
	  ],
	  // condition生效条件，关于 condition 的详细设置规则和COS支持的condition类型可以参考https://cloud.tencent.com/document/product/436/71306
	  // 'condition': {
	  //   // 比如限定ip访问
	  //   'ip_equal': {
	  //     'qcs:ip': '10.121.2.10/24'
	  //   }
	  // }
	}],
  };
  
  // getCredential
	STS.getCredential({
	secretId: config.secretId,
	secretKey: config.secretKey,
	proxy: config.proxy,
	durationSeconds: config.durationSeconds,
	region: config.region,
	endpoint: config.endpoint,
	policy: policy,
	}, function (err, credential) {
	console.log('getCredential:');
	console.log(JSON.stringify(policy, null, '    '));
	if (err) {
		console.log(err)
	}else{
		console.log(JSON.stringify(credential, null, '    '));
	}
	});
}

let IllegalType = {
	None : 0,
	Illegal:1,
	Warning:2,
}

function checkImageResult(rep) {
	if (rep.JobsDetail == null) {
		return {type:IllegalType.None,label:""}
	}

	for (const v of rep.JobsDetail) {
		if (v.Result != 0 && v.Score >= 99) {
			if (v.Result == 1) {
				return {type:IllegalType.Illegal,label:""}
			}else{
				return {type:IllegalType.Warning,label:""}
			}
		}
	}
	return {type:IllegalType.None,label:""}
}

function checkTextResult(rep) {
	let d = rep.JobsDetail
	if (d == null) {
		return {type:IllegalType.None,label:""}
	}

	if (d.Result != 0 && d.Score >= 99) {
		if (d.Result == 1) {
			return {type:IllegalType.Illegal,label:d.Label}
		}
		return {type:IllegalType.Warning,label:d.Label}
	}
	return {type:IllegalType.None,label:""}
}

function postImagesAuditing(cos) {
	const config = {
	  // 需要替换成您自己的存储桶信息
	  Bucket: 'bangbang1-1326763244', // 存储桶，必须
	  Region: 'ap-shanghai', // 存储桶所在地域，比如ap-beijing，必须
	};
	const key = 'image/auditing';  // 固定值，必须
	const host = config.Bucket + '.ci.' + config.Region + '.myqcloud.com';
	const url = `https://${host}/${key}`;
	const body = COS.util.json2xml({
	  Request: {
		Input: [{
		//   Object: '1.png', // 需要审核的图片，存储桶里的路径
			Url:"https://bangbang1-1326763244.cos.ap-shanghai.myqcloud.com/images/20240524_173521_/IMG_20240524_173521__318748.png",
		}, {
		//   Object: 'a/6.png', // 需要审核的图片，存储桶里的路径
			Url:"https://bangbang1-1326763244.cos.ap-shanghai.myqcloud.com/images/20240524_173521_/IMG_20240524_173521__318748.png",
		}],
		Conf: {
		  BizType: '', // 不填写代表默认策略
		}
	  }
	});
	cos.request({
		Method: 'POST', // 固定值，必须
		Url: url, // 请求的url，必须
		Key: key, // 固定值，必须
		ContentType: 'application/xml', // 固定值，必须
		Body: body // 请求体参数，必须
	},
	function(err, data){
		if (err) {
		  // 处理请求失败
		  console.log(err);
		} else {
		  // 处理请求成功
		  console.log(JSON.stringify(data.Response,null,"	"));
		  console.log("check type:",checkImageResult(data.Response))
		}
	});
  }
  

function test_check_image() {
	// SECRETID 和 SECRETKEY 请登录 https://console.cloud.tencent.com/cam/capi 进行查看和管理
	var cos = new COS({
		SecretId: 'AKIDVvGTbv7A7sV0Nm8WI4WpGjEjL0UEtQ2W',
		SecretKey: 'TpHj6Nep0GSgqvuhGkKlz42UtALQI2e5'
	});
	postImagesAuditing(cos)
}

async function postTextContentAuditing(cos,content) {
	const config = {
	  // 需要替换成您自己的存储桶信息
	  Bucket: 'bangbang1-1326763244', // 存储桶，必须
	  Region: 'ap-shanghai', // 存储桶所在地域，比如ap-beijing，必须
	};
	const host = config.Bucket + '.ci.' + config.Region + '.myqcloud.com';
	const key = 'text/auditing'; // 固定值，必须
	const url = `https://${host}/${key}`;
	const body = COS.util.json2xml({
	  Request: {
		Input: {
		  // 使用 COS.util.encodeBase64 方法需要sdk版本至少为1.4.19
		  Content: COS.util.encodeBase64(content), /* 需要审核的文本内容 */
		},
		Conf: {
		  BizType: '',
		}
	  }
	});

	try {
		let data = await cos.request({
			Method: 'POST', // 固定值，必须
			Url: url, // 请求的url，必须
			Key: key, // 固定值，必须
			ContentType: 'application/xml', // 固定值，必须
			Body: body // 请求体参数，必须
		})
		// 处理请求成功
		console.log(JSON.stringify(data.Response,null,"	") );
		// console.log(checkTextResult(data.Response))
		return {rep:data.Response,res:checkTextResult(data.Response)}
	} catch (err) {
		// 处理请求失败
		console.log(err);
	}
	return null
}
  

function test_check_text() {
	var cos = new COS({
		SecretId: 'AKIDVvGTbv7A7sV0Nm8WI4WpGjEjL0UEtQ2W',
		SecretKey: 'TpHj6Nep0GSgqvuhGkKlz42UtALQI2e5'
	});
	return postTextContentAuditing(cos,"测试敏感文本爱仕达十九大精神办法基本算法框架BASF")
}

const {Schema} = mongoose

const taskIllegalSchema = new Schema({
	taskid: String,
	illegalType: Number,
	label: String,
	detail: String,
})

const taskIllegalModule = mongoose.model("task_illegal_test",taskIllegalSchema)

function connectMongodb() {
	let dbname = GetConfValue("mongo-db")
	var url = GetConfValue("mongo-host")+"/"+dbname
	mongoose.connect(url).catch(err => console.log("mongodb connect err:",err))
}

async function test_mongodb() {
	connectMongodb()
	let res = await taskIllegalModule.create({
		taskid:"111",
		illegalType:0,
		label:"porn",
	})
	console.log("insert res:",res)
	await mongoose.disconnect()
}

async function test_mongo_delete() {
	connectMongodb()
	let res = await taskIllegalModule.deleteOne({_id:"6651621e414082262417878f"})
	console.log(res)
	await mongoose.disconnect()
}

function test_mongo_update() {
	connectMongodb()
	taskIllegalModule.updateOne({_id:"66516649b6db0ea3c5277206"},{label:"none"}).then((res) => {
		console.log(res)
	}).catch((err) => {
		console.log(err)
	}).finally(_=>mongoose.disconnect())
}

function test_req_double() {
	testreq()
	testreq2()
}
// test_req_double()

async function test_task_illegal(task) {

	initClient()

	let checkres = await test_check_text()
	if (checkres?.res == null || checkres.res.type == IllegalType.None) {
		return
	}

	addTaskIllegal(task,checkres.res,checkres.rep,"text",true)
	// closeMongoClient()
}

function test_link_list() {
	var list = new LinkedList()

	for (let index = 0; index < 10; index++) {
		list.push({n:index})
	}
	
	while (!list.empty()) {
		console.log(list.pop())
	}
}

// test_wxAccessToken()
// test_log()
// test_sts()
// test_check_image()
// test_check_text()
// test_mongodb()
// test_mongo_delete()
// test_mongo_update()

let task = {
	id: '665056c92ee339a92158facd',
}

// test_task_illegal(task)
test_link_list()