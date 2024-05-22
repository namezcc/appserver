package util

const (
	ST_GATE         = 0
	ST_APP_SERVICE  = 1
	ST_LOGIN        = 2
	ST_MYSQL        = 3
	ST_ROOM         = 8
	ST_ROOM_MANAGER = 9
	ST_LOGIN_LOCK   = 10
	ST_LOGSERVER    = 11
	ST_PUBLICK      = 13
	ST_ADMIN_MGR    = 14
	ST_ACCOUNT_ROLE = 15

	ST_SERVICE_FIND   = 40
	ST_MONITOR        = 41
	ST_MASTER         = 42
	ST_OPERATE        = 43
	ST_LOGIN_SERVICE  = 101
	ST_K8SERVICE      = 102
	ST_CLIENT_SERVICE = 103
	ST_USER_MANAGER   = 104
)

const (
	MYSQL_TYPE_GAME    = 0
	MYSQL_TYPE_ACCOUNT = 1
	MYSQL_TYPE_MASTER  = 2
)

const (
	SEX_WOMAN = 0
	SEX_MAN   = 1
)

const (
	MONEY_REWARD = 0
	MONEY_COST   = 1
)

const (
	TASK_STATE_IN_CHECK   = 0 //审核中
	TASK_STATE_OPEN       = 1 //进行中
	TASK_STATE_FINISH     = 2 //已完成
	TASK_STATE_CHECK_FAIL = 3 //审核未通过
	TASK_STATE_ILLEGAL    = 4 //非法下架
)

const (
	ERRCODE_SUCCESS = iota
	ERRCODE_ERROR
	ERRCODE_PEOPLE_FULL            //人数已满
	ERRCODE_TASK_OVER_ENDTIME      //报名时间已过
	ERRCODE_TASK_DELETE            //任务以取消
	ERRCODE_TASK_HAVE_JOIN         //已经报名
	ERRCODE_TASK_QUIT_ERROR        //退出失败
	ERRCODE_IN_BLACK_LIST     = 7  //被拉黑名单
	ERRCODE_MAX_USER_INTEREST = 8  //收藏已满
	ERRCODE_CREDIT_SCORE      = 9  //信用分不足
	ERRCODE_UNAUTHORIZED      = 10 //身份验证失败
)

const (
	USER_HTTP_ERR_SUCCESS = 0
	USER_HTTP_ERR_ERROR   = 1
)
