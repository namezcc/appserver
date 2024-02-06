package module

type RedisManagerModule struct {
	modulebase
	_redismod    []*RedisWorker
	_redis_index int
}

const REDIS_WOKER_NUM = 5

func (m *RedisManagerModule) Init(*moduleMgr) {
	m._redismod = make([]*RedisWorker, 0)
	m._redis_index = 0
	for i := 0; i < REDIS_WOKER_NUM; i++ {
		rdm := RedisWorker{}
		rdm.Init()
		m._redismod = append(m._redismod, &rdm)
	}

}

func (m *RedisManagerModule) AfterInit() {

}

func (m *RedisManagerModule) rangeRedisIndex() int {
	rid := m._redis_index
	m._redis_index++
	if m._redis_index >= REDIS_WOKER_NUM {
		m._redis_index = 0
	}
	return rid
}

// 阻塞执行
func (m *RedisManagerModule) RequestRedis(cmdid int, d interface{}, backdata bool) interface{} {
	rid := m.rangeRedisIndex()
	return m._redismod[rid].RequestCmd(cmdid, d, backdata, true)
}

// 阻塞执行
func (m *RedisManagerModule) RequestRedisHash(hashval uint32, cmdid int, d interface{}, backdata bool) interface{} {
	var rid int
	if hashval == 0 {
		rid = int(hashval % REDIS_WOKER_NUM)
	} else {
		rid = m.rangeRedisIndex()
	}
	return m._redismod[rid].RequestCmd(cmdid, d, backdata, true)
}

// 不阻塞函数
func (m *RedisManagerModule) RedisWorkNoBlock(cmdid int, d interface{}) {
	rid := m.rangeRedisIndex()
	m._redismod[rid].RequestCmd(cmdid, d, false, false)
}

// 不阻塞函数
func (m *RedisManagerModule) RedisWorkNoBlockHash(hashval uint32, cmdid int, d interface{}) {
	var rid int
	if hashval == 0 {
		rid = int(hashval % REDIS_WOKER_NUM)
	} else {
		rid = m.rangeRedisIndex()
	}
	m._redismod[rid].RequestCmd(cmdid, d, false, false)
}

// 阻塞函数,有返回值
func (m *RedisManagerModule) RequestRedisFuncCall(f RedisDynamicFunc) interface{} {
	return m.RequestRedis(RCMD_FUNC_CALL, f, true)
}

// 阻塞函数,有返回值
func (m *RedisManagerModule) RequestRedisFuncCallHash(hashval uint32, f RedisDynamicFunc) interface{} {
	return m.RequestRedisHash(hashval, RCMD_FUNC_CALL, f, true)
}

// 阻塞函数,无返回值
func (m *RedisManagerModule) RequestRedisFuncCallNoRes(f RedisDynamicFuncNoRes) {
	m.RequestRedis(RCMD_FUNC_CALL, f, false)
}

// 阻塞函数,无返回值
func (m *RedisManagerModule) RequestRedisFuncCallNoResHash(hashval uint32, f RedisDynamicFuncNoRes) {
	m.RequestRedisHash(hashval, RCMD_FUNC_CALL, f, false)
}

// 不阻塞函数,不知道是否有闭包问题,参数提前被释放
func (m *RedisManagerModule) RedisWorkFuncCallNoBlock(f RedisDynamicFuncNoRes) {
	m.RedisWorkNoBlock(RCMD_FUNC_CALL, f)
}

// 不阻塞函数,不知道是否有闭包问题,参数提前被释放
func (m *RedisManagerModule) RedisWorkFuncCallNoBlockHash(hashval uint32, f RedisDynamicFuncNoRes) {
	m.RedisWorkNoBlockHash(hashval, RCMD_FUNC_CALL, f)
}
