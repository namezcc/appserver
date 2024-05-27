const mongoose = require('mongoose');
const { GetConfValue } = require('./util_conf');
const { log_error, log_info } = require('./util_log');

var TASK_STATE = {
	TASK_STATE_IN_CHECK   : 0, //审核中
	TASK_STATE_OPEN       : 1, //进行中
	TASK_STATE_FINISH     : 2, //已完成
	TASK_STATE_CHECK_FAIL : 3, //审核未通过
	TASK_STATE_ILLEGAL    : 4, //非法下架
}

const {Schema} = mongoose

const taskIllegalSchema = new Schema({
	taskid: String,
	illegalType: Number,
	label: String,
	detail: String,
	taskDelete: Number,
	contentType: String,
},{
	collection:"task_illegal"
})

const taskGlobelSchema = new Schema({
	title: String,
	// updateAt: Date,
},{
	collection:"task_globel"
})

const taskLocationSchema = new Schema({
	title: String,
},{
	collection:"task_location"
})

const taskSchema = new Schema({
	title: String,
	state: Number,
},{
	collection:"task"
})

const taskIllegalModule = mongoose.model("task_illegal",taskIllegalSchema)
const taskGlobelModule = mongoose.model("task_globel",taskGlobelSchema)
const taskLocationModule = mongoose.model("task_location",taskLocationSchema)
const taskModule = mongoose.model("task",taskSchema)

function initClient() {
	let dbname = GetConfValue("mongo-db")
	var url = GetConfValue("mongo-host")+"/"+dbname
	mongoose.connect(url).catch(err => {
		log_error("mongodb connect err:",err)
		process.exit(2)
	}).then(() => {
		log_info("connect mongodb %s",url)
	})
}

function closeMongoClient() {
	mongoose.disconnect()
}

function addTaskIllegal(task,illegalInfo,rep,contentType,removeTask) {
	let taskid = task.id
	let mgillegal = {
		taskid:taskid,
		illegalType:illegalInfo.type,
		label:illegalInfo.label,
		detail:JSON.stringify(rep),
		taskDelete: removeTask ? 1 : 1,
		contentType:contentType,
	}

	taskIllegalModule.create(mgillegal).catch((err) => {
		log_error("taskIllegalModule.create err:",err)
	}).then((res) => {
		log_info("taskIllegalModule.create id:%s res:",taskid,res)
	})

	if (removeTask) {
		taskGlobelModule.deleteOne({_id:taskid}).catch((err) => {
			log_error("taskGlobelModule.deleteOne id:%s err:",taskid,err)
		}).then((res) => {
			log_info("taskGlobelModule.deleteOne ",res)
		})
		taskLocationModule.deleteOne({_id:taskid}).catch((err) => {
			log_error("taskLocationModule.deleteOne id:%s err:",taskid,err)
		}).then((res) => {
			log_info("taskLocationModule.deleteOne ",res)
		})
		taskModule.updateOne({_id:taskid},{state:TASK_STATE.TASK_STATE_ILLEGAL}).catch((err) => {
			log_error("taskModule.updateOne id:%s err:",taskid,err)
		}).then((res) => {
			log_info("taskModule.updateOne ",res)
		})
	}
}

module.exports = {
	initClient,
	addTaskIllegal,
	closeMongoClient,
}