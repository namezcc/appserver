function getNowSecond() {
	return parseInt(Date.now()/1000)
}

module.exports = {
	getNowSecond,
}