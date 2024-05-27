const { GetConfValue } = require("./util_conf")
const { log_error, log_info } = require("./util_log")
const { getNowSecond } = require("./util_time")
const axios = require("axios").default
const STS = require("qcloud-cos-sts")
var crypto = require('crypto');
var moment = require('moment');

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

function handleError(error) {
	if (error.response) {
	  // 请求已发出，服务器用状态码响应
	  log_error(`HTTP ${error.response.status} - ${error.response.statusText}`);
	} else if (error.request) {
	  // 请求已发出但没有收到响应
	  log_error('No response received');
	} else {
	  // 在尝试执行请求时出现错误
	  log_error('Error during request setup:', error.message);
	}
	log_error(error.config);
  }

async function getWxAccessToken() {
	let url = "https://api.weixin.qq.com/cgi-bin/stable_token"
	try {
		let rep = await axios.post(url,{
			grant_type:"client_credential",
			appid:appid,
			secret:secret
		})
		return rep?.data
	} catch (error) {
		handleError(error)
	}
	return null
}

async function getWxPhoneNumber(code) {
	let url = "https://api.weixin.qq.com/wxa/business/getuserphonenumber?access_token="+access_token.access_token
	let res = {
		errcode:0,
		errmsg:"error",
		phoneNumber:"",
	}
	try {
		let rep = await axios.post(url,{
			code:code
		})
		if (rep?.data) {
			res.errcode = rep.data.errcode
			res.errmsg = rep.data.errmsg
			res.phoneNumber = rep.data?.phone_info?.phoneNumber
		}
	} catch (error) {
		handleError(error)
	}
	return res
}

async function startAccessTokenJob() {
	initWxData()
	let res = await getWxAccessToken()
	if (res == null || res.errcode != null) {
		log_error("getWxAccessToken fail",res)
		process.exit(1)
	}

	log_info("get accesstoen success",res)
	// 过期前4分钟请求
	access_token.expires_time = getNowSecond() + res.expires_in - 4 * 60
	// access_token.expires_time = getNowSecond() + 30
	access_token.access_token = res.access_token
	setInterval(async () => {
		let now = getNowSecond()
		if (now < access_token.expires_time) {
			return
		}
		log_info("getWxAccessToken task")
		res = await getWxAccessToken()
		if (res?.expires_in) {
			access_token.expires_time = getNowSecond() + res.expires_in - 4 * 60
			// access_token.expires_time = getNowSecond() + 30
			access_token.access_token = res.access_token
			log_info("update token expires %d",res.expires_in)
		}else if (res?.errcode) {
			log_error("getWxAccessToken fail errcode:%d msg:%s",res.errcode,res.errmsg)
		}else{
			log_error("get other error ",res)
		}
	},5000)
	return true
}

function getAccessToken() {
	return access_token
}

var ossConf = {
	// endpoint: 'sts.internal.tencentcloudapi.com', // 域名，非必须，与host二选一，默认为 sts.tencentcloudapi.com
	proxy : '',
	host : 'sts.tencentcloudapi.com', // 域名，非必须，默认为 sts.tencentcloudapi.com
	durationSeconds : 1800,  // 密钥有效期
	allowPrefix : 'bangbang', // 这里改成允许的路径前缀，可以根据自己网站的用户登录态判断允许上传的具体路径，例子： a.jpg 或者 a/* 或者 * (使用通配符*存在重大安全风险, 请谨慎评估使用)
	secretId:"",
	secretKey:"",
	bucket:"",
	region:"",
}
var osspolicy = {}
function initOssData() {
	ossConf.secretId = GetConfValue("oss_secretId")
	ossConf.secretKey = GetConfValue("oss_secretKey")
	ossConf.bucket = GetConfValue("oss_buket")
	ossConf.region = GetConfValue("oss_region")

	var appId = ossConf.bucket.substr(1 + ossConf.bucket.lastIndexOf('-'));
	osspolicy = {
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
			'qcs::cos:' + ossConf.region + ':uid/' + appId + ':' + ossConf.bucket + '/' + ossConf.allowPrefix,
			// ci相关授权，按需使用
			'qcs::ci:' + ossConf.region + ':uid/' + appId + ':bucket/' + ossConf.bucket + '/*',
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
}

function getOssConfig() {
	return ossConf
}

var ossCredential = {
	expiredTime : 0,
}
function getOssCredential() {
	STS.getCredential({
		secretId: ossConf.secretId,
		secretKey: ossConf.secretKey,
		proxy: ossConf.proxy,
		durationSeconds: ossConf.durationSeconds,
		region: ossConf.region,
		endpoint: ossConf.endpoint,
		policy: osspolicy,
		}, function (err, credential) {
		log_info('getCredential:');
		// console.log(JSON.stringify(policy, null, '    '));
		if (err) {
			log_error(err)
		}else{
			log_info(JSON.stringify(credential, null, '    '));
			ossCredential = credential
		}
	});
}

function startOssCredentialJob() {
	initOssData()
	// getOssCredential()
	// setInterval(() => {
	// 	let now = getNowSecond()
	// 	if (now < ossCredential.expiredTime - 5*60) {
	// 		return
	// 	}
	// 	getOssCredential()
	// }, 30000);
}

function getOssGredent() {
	return ossCredential
}

// 生成要上传的 COS 文件路径文件名
var generateCosKey = function (ext) {
	var ymd = moment().format('YYYYMMDD');
	var ymd = moment().format('YYYYMMDD_HHmmss_');
	var r = ('000000' + Math.random() * 1000000).slice(-6);
	var cosKey = `images/${ymd}/IMG_${ymd}_${r}.${ext}`;
	return cosKey;
};

function getPostPolicy(ext) {
	var cosHost = `${ossConf.bucket}.cos.${ossConf.region}.myqcloud.com`;
	var cosKey = generateCosKey(ext);
    var now = Math.round(Date.now() / 1000);
    var exp = now + 900;
    var qKeyTime = now + ';' + exp;
    var qSignAlgorithm = 'sha1';
    // 生成上传要用的 policy
    // PostObject 签名保护文档 https://cloud.tencent.com/document/product/436/14690#.E7.AD.BE.E5.90.8D.E4.BF.9D.E6.8A.A4
    var policy = JSON.stringify({
        'expiration': new Date(exp * 1000).toISOString(),
        'conditions': [
            // {'acl': query.ACL},
            // ['starts-with', '$Content-Type', 'image/'],
            // ['starts-with', '$success_action_redirect', redirectUrl],
            // ['eq', '$x-cos-server-side-encryption', 'AES256'],
            {'q-sign-algorithm': qSignAlgorithm},
            {'q-ak': ossConf.secretId},
            {'q-sign-time': qKeyTime},
            {'bucket': ossConf.bucket},
            {'key': cosKey},
        ]
    });

    // 步骤一：生成 SignKey
    var signKey = crypto.createHmac('sha1', ossConf.secretKey).update(qKeyTime).digest('hex');
    // 步骤二：生成 StringToSign
    var stringToSign = crypto.createHash('sha1').update(policy).digest('hex');
    // 步骤三：生成 Signature
    var qSignature = crypto.createHmac('sha1', signKey).update(stringToSign).digest('hex');

	var data = {
		cosHost: cosHost,
		cosKey: cosKey,
		policy: Buffer.from(policy).toString('base64'),
		qSignAlgorithm: qSignAlgorithm,
		qAk: ossConf.secretId,
		qKeyTime: qKeyTime,
		qSignature: qSignature,
		// securityToken: securityToken, // 如果 SecretId、SecretKey 是临时密钥，要返回对应的 sessionToken 的值
	}
	return data
}

module.exports = {
	startAccessTokenJob,
	getAccessToken,
	getWxAccessToken,
	getWxPhoneNumber,
	startOssCredentialJob,
	getOssGredent,
	getPostPolicy,
	getOssConfig,
}