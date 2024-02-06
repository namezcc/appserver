package handle

const (
	MS_BEGIN = 100

	M_CONNECT_SERVER = MS_BEGIN + iota
	M_SERVER_CONNECTED
	M_ACCEPT_CONN
	M_SEND_MSG
	M_CONN_CLOSE
	M_TIME_CALL
	M_EVENT_CALL
	M_ON_RESPONSE
	M_GET_SERVER_STATE
	M_GET_LOGIN_LIST
	M_HOT_LOAD
	M_SAVE_BATTLE_DATA
	M_GET_DRAW_LOG
	M_GET_SERVER_CONFIG
	M_SEND_BAN_INFO
	M_NOTICE_DB_NUM_INFO_CHANGE
	M_ADD_SERVER_MAIL
	M_ADD_PLAYER_MAIL
	M_ON_CLINET_MSG
	M_ON_TASK_JOIN_UPDATE
	M_ON_TASK_NO_CHAT = 121
	M_ON_TASK_DELETE  = 122

	N_REGISTE_SERVER      = 303
	N_TRANS_SERVER_MSG    = 304
	N_SERVER_PING         = 305
	N_SERVER_PONG         = 306
	N_SERVER_LINK_INFO    = 308
	N_CONN_SERVER_INFO    = 313
	N_REGIST_SERVICE_FIND = 314
	N_MASTER_LUA_ERROR    = 1001
	N_MASTER_BATTLE_DATA  = 1002
	N_SEND_BAN_INFO       = 1003
	N_WBE_TEST            = 2001
	N_WBE_TEST2           = 2002
	N_WBE_ON_RESPONSE     = 2003
	N_WBE_REQUEST_1       = 2004
	N_WEB_VIEW_MACHINE    = 2005
	N_WEB_GET_SERVER_INFO = 2006
	N_WEB_SERVER_OPT      = 2007

	N_SF_NOTICE_SERVER = 2100

	N_CM_LOGIN          = 3000
	N_CM_PING           = 3001
	N_CM_TASK_CHAT      = 3002
	N_CM_LOAD_TASK_CHAT = 3003
	N_CM_TASK_CHAT_READ = 3004

	MS_END = 4000

	IM_LOGIN_BEGIN       = 12000
	IM_LOGIN_WEB_HOTLOAD = 12500
	IM_LOGIN_AI_UPDATE   = 12501

	IM_LOGIN_END = 13000
)

const (
	SM_BEGIN = iota
	SM_PONG
	SM_ERROR_CODE
	SM_CHAT_UPDATE
	SM_CHAT_INDEX
	SM_CHAT_READ
	SM_END
)

type BaseMsg struct {
	Mid  int
	Data interface{}
}

type HandleFunc func(*BaseMsg)

type handleBlock struct {
	_handle    []HandleFunc
	_msg_begin int
	_msg_end   int
}

type handlemsg struct {
	_block []handleBlock
}

var Handlemsg = handlemsg{}

func (_h *handlemsg) InitMsg(msg_begin int, msg_end int) {
	_h._block = append(_h._block, handleBlock{
		_msg_begin: msg_begin,
		_msg_end:   msg_end,
		_handle:    make([]HandleFunc, msg_end-msg_begin),
	})
}

func (_h *handlemsg) AddMsgCall(mid int, _f HandleFunc) {
	for _, v := range _h._block {
		if v._msg_begin <= mid && v._msg_end >= mid {
			v._handle[mid-v._msg_begin] = _f
			return
		}
	}
}

func (_h *handlemsg) CallHandle(msg *BaseMsg) {
	mid := msg.Mid

	for _, v := range _h._block {
		if v._msg_begin <= mid && v._msg_end >= mid {
			if v._handle[mid-v._msg_begin] != nil {
				v._handle[mid-v._msg_begin](msg)
			}
			return
		}
	}
}
