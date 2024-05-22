function log_info(fmt,...param) {
	console.log(fmt,...param)
}

function log_error(fmt,...param) {
	console.log(fmt,...param)
}

function log_debug(fmt,...param) {
	console.log(fmt,...param)
}

module.exports = {
	log_info,
	log_error,
	log_debug,
}