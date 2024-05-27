const { LinkedList } = require("./nodelist")
var COS = require('cos-nodejs-sdk-v5');
const { getOssConfig } = require("./util_wx");
const { log_error } = require("./util_log");
const { addTaskIllegal } = require("./util_mongo");

let cos = null
function getCos() {
	if (cos == null) {
		let ossConf = getOssConfig()
		cos = new COS({
			SecretId: ossConf.secretId,
			SecretKey: ossConf.secretKey
		});
	}
	return cos
}

let IllegalType = {
	None : 0,
	Illegal:1,
	Warning:2,
}

function checkImageResult(rep) {
	if (rep?.JobsDetail == null) {
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
	let d = rep?.JobsDetail
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

async function postImagesAuditing(taskid,cos,images) {
	let ossConf = getOssConfig()
	const config = {
	  // 需要替换成您自己的存储桶信息
	  Bucket: ossConf.bucket, // 存储桶，必须
	  Region: ossConf.region, // 存储桶所在地域，比如ap-beijing，必须
	};
	const key = 'image/auditing';  // 固定值，必须
	const host = config.Bucket + '.ci.' + config.Region + '.myqcloud.com';
	const url = `https://${host}/${key}`;

	let input = []
	for (const imgurl of images) {
		input.push({
			Url:imgurl,
		})
	}

	const body = COS.util.json2xml({
	  Request: {
		Input: input,
		Conf: {
		  BizType: '', // 不填写代表默认策略
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
		return {rep:data.Response,res:checkImageResult(data.Response)}
	} catch (err) {
		// 处理请求失败
		log_error("postImagesAuditing fail task:",taskid,err)
	}
	return null
}

async function postTextContentAuditing(taskid,cos,content) {
	let ossConf = getOssConfig()
	const config = {
	  // 需要替换成您自己的存储桶信息
	  Bucket: ossConf.bucket, // 存储桶，必须
	  Region: ossConf.region, // 存储桶所在地域，比如ap-beijing，必须
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
		// console.log(JSON.stringify(data.Response,null,"	") );
		return {rep:data.Response,res:checkTextResult(data.Response)}
	} catch (err) {
		// 处理请求失败
		log_error("postTextContentAuditing fail task:",taskid,err)
	}
	return null
}

async function checkTaskIllegal(task) {
	// 图片检查,图片违规下架并进入嫌疑列表
	let cos = getCos()
	if (task?.images?.length > 0) {
		let checkres = await postImagesAuditing(task.id,cos,task.images)
		if (checkres && checkres.res.type != IllegalType.None) {
			addTaskIllegal(task,checkres.res,checkres.rep,"image",checkres.res.type == IllegalType.Illegal)
			return
		}
	}
	// 文本检查,文本违法进入嫌疑列表
	let content = `${task.title},${task.content},${task.contact_way}`
	checkres = await postTextContentAuditing(task.id,cos,content)
	if (checkres && checkres.res.type != IllegalType.None) {
		addTaskIllegal(task,checkres.res,checkres.rep,"text",false)
	}
}

var taskList = new LinkedList()

function pushTask(task) {
	taskList.push(task)
}

function startCheckTaskJob() {
	setTimeout(async () => {
		while (!taskList.empty()) {
			let task = taskList.pop()
			await checkTaskIllegal(task)
		}
		startCheckTaskJob()
	},3000)
}

module.exports = {
	startCheckTaskJob,
	pushTask,	
}