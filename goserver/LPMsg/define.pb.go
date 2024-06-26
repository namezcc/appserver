// Code generated by protoc-gen-go. DO NOT EDIT.
// source: define.proto

package LPMsg

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type LP_CM_MSG_ID int32

const (
	LP_CM_MSG_ID_CM_MSG_NONE                     LP_CM_MSG_ID = 0
	LP_CM_MSG_ID_CM_BEGAN                        LP_CM_MSG_ID = 10000
	LP_CM_MSG_ID_CM_LOGIN                        LP_CM_MSG_ID = 10001
	LP_CM_MSG_ID_CM_ENTER_ROOM                   LP_CM_MSG_ID = 10002
	LP_CM_MSG_ID_CM_CREATE_ROLE                  LP_CM_MSG_ID = 10003
	LP_CM_MSG_ID_CM_PING                         LP_CM_MSG_ID = 10004
	LP_CM_MSG_ID_CM_ENTER_GAME                   LP_CM_MSG_ID = 10009
	LP_CM_MSG_ID_CM_ITEM_UNSET_NEW               LP_CM_MSG_ID = 10010
	LP_CM_MSG_ID_CM_ENTER_DUNGEON                LP_CM_MSG_ID = 10011
	LP_CM_MSG_ID_CM_BATTLE_SET_HERO              LP_CM_MSG_ID = 10012
	LP_CM_MSG_ID_CM_QUIT_BATTLE_DUNGEON          LP_CM_MSG_ID = 10013
	LP_CM_MSG_ID_CM_BATTLE_ACTION                LP_CM_MSG_ID = 10014
	LP_CM_MSG_ID_CM_PVE_DUNGEON_FINISH           LP_CM_MSG_ID = 10015
	LP_CM_MSG_ID_CM_GET_CHAPTER_REWARD           LP_CM_MSG_ID = 10016
	LP_CM_MSG_ID_CM_ENTER_RES_DUNGEON            LP_CM_MSG_ID = 10017
	LP_CM_MSG_ID_CM_SAODANG_RES_DUNGEON          LP_CM_MSG_ID = 10018
	LP_CM_MSG_ID_CM_SAODANG_DUNGEON              LP_CM_MSG_ID = 10019
	LP_CM_MSG_ID_CM_DEBUG                        LP_CM_MSG_ID = 10023
	LP_CM_MSG_ID_CM_ITEM_USE                     LP_CM_MSG_ID = 10024
	LP_CM_MSG_ID_CM_DRAW_CARD                    LP_CM_MSG_ID = 10030
	LP_CM_MSG_ID_CM_SHOP_MALL_BUY                LP_CM_MSG_ID = 10040
	LP_CM_MSG_ID_CM_SELL_ITEM                    LP_CM_MSG_ID = 10041
	LP_CM_MSG_ID_CM_HERO_ITEM_ADD_EXP            LP_CM_MSG_ID = 10050
	LP_CM_MSG_ID_CM_HERO_ORDER_UP                LP_CM_MSG_ID = 10051
	LP_CM_MSG_ID_CM_HERO_TALENT_UP               LP_CM_MSG_ID = 10052
	LP_CM_MSG_ID_CM_HERO_TALENT_SET_SKILL        LP_CM_MSG_ID = 10053
	LP_CM_MSG_ID_CM_HERO_SKILL_UP                LP_CM_MSG_ID = 10054
	LP_CM_MSG_ID_CM_GET_TASK_REWARD              LP_CM_MSG_ID = 10060
	LP_CM_MSG_ID_CM_GET_DAILY_TASK_ACTIVE_REWARD LP_CM_MSG_ID = 10061
	LP_CM_MSG_ID_CM_ACHIEVEMENT_GET_REWARD       LP_CM_MSG_ID = 10062
	LP_CM_MSG_ID_CM_MAIL_READ                    LP_CM_MSG_ID = 10070
	LP_CM_MSG_ID_CM_MAIL_GET                     LP_CM_MSG_ID = 10071
	LP_CM_MSG_ID_CM_MAIL_DELETE                  LP_CM_MSG_ID = 10072
	LP_CM_MSG_ID_CM_BUY_TILI                     LP_CM_MSG_ID = 10075
	LP_CM_MSG_ID_CM_CHANGE_NAME                  LP_CM_MSG_ID = 10076
	LP_CM_MSG_ID_CM_SET_SIGNATURE                LP_CM_MSG_ID = 10077
	LP_CM_MSG_ID_CM_HERO_CHANGE_EQUIP            LP_CM_MSG_ID = 10080
	LP_CM_MSG_ID_CM_HERO_CHANGE_GEM              LP_CM_MSG_ID = 10081
	LP_CM_MSG_ID_CM_EQUIP_ADD_EXP                LP_CM_MSG_ID = 10082
	LP_CM_MSG_ID_CM_EQUIP_STAR_UP                LP_CM_MSG_ID = 10083
	LP_CM_MSG_ID_CM_GEM_ADD_EXP                  LP_CM_MSG_ID = 10084
	LP_CM_MSG_ID_CM_ITEM_LOCK                    LP_CM_MSG_ID = 10085
	LP_CM_MSG_ID_CM_SET_HEAD                     LP_CM_MSG_ID = 10090
	LP_CM_MSG_ID_CM_SET_HERO_TEAM                LP_CM_MSG_ID = 10100
	LP_CM_MSG_ID_CM_SET_HERO_TEAM_NAME           LP_CM_MSG_ID = 10101
	LP_CM_MSG_ID_CM_SET_CLIENT_RECORD            LP_CM_MSG_ID = 10110
	LP_CM_MSG_ID_CM_RELATION_SEARCH_PLAYER       LP_CM_MSG_ID = 10120
	LP_CM_MSG_ID_CM_RELATION_RECOMMEND           LP_CM_MSG_ID = 10121
	LP_CM_MSG_ID_CM_RELATION_APPLY_FRIEND        LP_CM_MSG_ID = 10122
	LP_CM_MSG_ID_CM_RELATION_FRIEND_AGREE        LP_CM_MSG_ID = 10123
	LP_CM_MSG_ID_CM_RELATION_FRIEND_DISAGREE     LP_CM_MSG_ID = 10124
	LP_CM_MSG_ID_CM_RELATION_FRIEND_DELETE       LP_CM_MSG_ID = 10125
	LP_CM_MSG_ID_CM_RELATION_SET_BLACK           LP_CM_MSG_ID = 10126
	LP_CM_MSG_ID_CM_RELATION_BLACK_DELETE        LP_CM_MSG_ID = 10127
	LP_CM_MSG_ID_CM_GET_PLAYER_SIMPLE_INFO       LP_CM_MSG_ID = 10128
	LP_CM_MSG_ID_CM_RELATION_SET_NOTES           LP_CM_MSG_ID = 10129
	LP_CM_MSG_ID_CM_RELATION_SEND_REWARD         LP_CM_MSG_ID = 10130
	LP_CM_MSG_ID_CM_RELATION_GET_REWARD          LP_CM_MSG_ID = 10131
	LP_CM_MSG_ID_CM_SEND_CHAT                    LP_CM_MSG_ID = 10201
	LP_CM_MSG_ID_CM_TEST_BATTLE_DATA             LP_CM_MSG_ID = 11000
	LP_CM_MSG_ID_CM_GET_BATTLE_DATA              LP_CM_MSG_ID = 11001
	LP_CM_MSG_ID_CM_END                          LP_CM_MSG_ID = 15000
)

var LP_CM_MSG_ID_name = map[int32]string{
	0:     "CM_MSG_NONE",
	10000: "CM_BEGAN",
	10001: "CM_LOGIN",
	10002: "CM_ENTER_ROOM",
	10003: "CM_CREATE_ROLE",
	10004: "CM_PING",
	10009: "CM_ENTER_GAME",
	10010: "CM_ITEM_UNSET_NEW",
	10011: "CM_ENTER_DUNGEON",
	10012: "CM_BATTLE_SET_HERO",
	10013: "CM_QUIT_BATTLE_DUNGEON",
	10014: "CM_BATTLE_ACTION",
	10015: "CM_PVE_DUNGEON_FINISH",
	10016: "CM_GET_CHAPTER_REWARD",
	10017: "CM_ENTER_RES_DUNGEON",
	10018: "CM_SAODANG_RES_DUNGEON",
	10019: "CM_SAODANG_DUNGEON",
	10023: "CM_DEBUG",
	10024: "CM_ITEM_USE",
	10030: "CM_DRAW_CARD",
	10040: "CM_SHOP_MALL_BUY",
	10041: "CM_SELL_ITEM",
	10050: "CM_HERO_ITEM_ADD_EXP",
	10051: "CM_HERO_ORDER_UP",
	10052: "CM_HERO_TALENT_UP",
	10053: "CM_HERO_TALENT_SET_SKILL",
	10054: "CM_HERO_SKILL_UP",
	10060: "CM_GET_TASK_REWARD",
	10061: "CM_GET_DAILY_TASK_ACTIVE_REWARD",
	10062: "CM_ACHIEVEMENT_GET_REWARD",
	10070: "CM_MAIL_READ",
	10071: "CM_MAIL_GET",
	10072: "CM_MAIL_DELETE",
	10075: "CM_BUY_TILI",
	10076: "CM_CHANGE_NAME",
	10077: "CM_SET_SIGNATURE",
	10080: "CM_HERO_CHANGE_EQUIP",
	10081: "CM_HERO_CHANGE_GEM",
	10082: "CM_EQUIP_ADD_EXP",
	10083: "CM_EQUIP_STAR_UP",
	10084: "CM_GEM_ADD_EXP",
	10085: "CM_ITEM_LOCK",
	10090: "CM_SET_HEAD",
	10100: "CM_SET_HERO_TEAM",
	10101: "CM_SET_HERO_TEAM_NAME",
	10110: "CM_SET_CLIENT_RECORD",
	10120: "CM_RELATION_SEARCH_PLAYER",
	10121: "CM_RELATION_RECOMMEND",
	10122: "CM_RELATION_APPLY_FRIEND",
	10123: "CM_RELATION_FRIEND_AGREE",
	10124: "CM_RELATION_FRIEND_DISAGREE",
	10125: "CM_RELATION_FRIEND_DELETE",
	10126: "CM_RELATION_SET_BLACK",
	10127: "CM_RELATION_BLACK_DELETE",
	10128: "CM_GET_PLAYER_SIMPLE_INFO",
	10129: "CM_RELATION_SET_NOTES",
	10130: "CM_RELATION_SEND_REWARD",
	10131: "CM_RELATION_GET_REWARD",
	10201: "CM_SEND_CHAT",
	11000: "CM_TEST_BATTLE_DATA",
	11001: "CM_GET_BATTLE_DATA",
	15000: "CM_END",
}

var LP_CM_MSG_ID_value = map[string]int32{
	"CM_MSG_NONE":                     0,
	"CM_BEGAN":                        10000,
	"CM_LOGIN":                        10001,
	"CM_ENTER_ROOM":                   10002,
	"CM_CREATE_ROLE":                  10003,
	"CM_PING":                         10004,
	"CM_ENTER_GAME":                   10009,
	"CM_ITEM_UNSET_NEW":               10010,
	"CM_ENTER_DUNGEON":                10011,
	"CM_BATTLE_SET_HERO":              10012,
	"CM_QUIT_BATTLE_DUNGEON":          10013,
	"CM_BATTLE_ACTION":                10014,
	"CM_PVE_DUNGEON_FINISH":           10015,
	"CM_GET_CHAPTER_REWARD":           10016,
	"CM_ENTER_RES_DUNGEON":            10017,
	"CM_SAODANG_RES_DUNGEON":          10018,
	"CM_SAODANG_DUNGEON":              10019,
	"CM_DEBUG":                        10023,
	"CM_ITEM_USE":                     10024,
	"CM_DRAW_CARD":                    10030,
	"CM_SHOP_MALL_BUY":                10040,
	"CM_SELL_ITEM":                    10041,
	"CM_HERO_ITEM_ADD_EXP":            10050,
	"CM_HERO_ORDER_UP":                10051,
	"CM_HERO_TALENT_UP":               10052,
	"CM_HERO_TALENT_SET_SKILL":        10053,
	"CM_HERO_SKILL_UP":                10054,
	"CM_GET_TASK_REWARD":              10060,
	"CM_GET_DAILY_TASK_ACTIVE_REWARD": 10061,
	"CM_ACHIEVEMENT_GET_REWARD":       10062,
	"CM_MAIL_READ":                    10070,
	"CM_MAIL_GET":                     10071,
	"CM_MAIL_DELETE":                  10072,
	"CM_BUY_TILI":                     10075,
	"CM_CHANGE_NAME":                  10076,
	"CM_SET_SIGNATURE":                10077,
	"CM_HERO_CHANGE_EQUIP":            10080,
	"CM_HERO_CHANGE_GEM":              10081,
	"CM_EQUIP_ADD_EXP":                10082,
	"CM_EQUIP_STAR_UP":                10083,
	"CM_GEM_ADD_EXP":                  10084,
	"CM_ITEM_LOCK":                    10085,
	"CM_SET_HEAD":                     10090,
	"CM_SET_HERO_TEAM":                10100,
	"CM_SET_HERO_TEAM_NAME":           10101,
	"CM_SET_CLIENT_RECORD":            10110,
	"CM_RELATION_SEARCH_PLAYER":       10120,
	"CM_RELATION_RECOMMEND":           10121,
	"CM_RELATION_APPLY_FRIEND":        10122,
	"CM_RELATION_FRIEND_AGREE":        10123,
	"CM_RELATION_FRIEND_DISAGREE":     10124,
	"CM_RELATION_FRIEND_DELETE":       10125,
	"CM_RELATION_SET_BLACK":           10126,
	"CM_RELATION_BLACK_DELETE":        10127,
	"CM_GET_PLAYER_SIMPLE_INFO":       10128,
	"CM_RELATION_SET_NOTES":           10129,
	"CM_RELATION_SEND_REWARD":         10130,
	"CM_RELATION_GET_REWARD":          10131,
	"CM_SEND_CHAT":                    10201,
	"CM_TEST_BATTLE_DATA":             11000,
	"CM_GET_BATTLE_DATA":              11001,
	"CM_END":                          15000,
}

func (x LP_CM_MSG_ID) String() string {
	return proto.EnumName(LP_CM_MSG_ID_name, int32(x))
}

func (LP_CM_MSG_ID) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_f7e38f9743a0f071, []int{0}
}

type LP_SM_MSG_ID int32

const (
	LP_SM_MSG_ID_SM_MSG_NONE                    LP_SM_MSG_ID = 0
	LP_SM_MSG_ID_SM_BEGIN                       LP_SM_MSG_ID = 15000
	LP_SM_MSG_ID_SM_LOGIN_RES                   LP_SM_MSG_ID = 15001
	LP_SM_MSG_ID_SM_ENTER_ROOM                  LP_SM_MSG_ID = 15002
	LP_SM_MSG_ID_SM_CREATE_ROLE                 LP_SM_MSG_ID = 15003
	LP_SM_MSG_ID_SM_SELF_ROLE_INFO              LP_SM_MSG_ID = 15004
	LP_SM_MSG_ID_SM_PLAYER_ALL_INFO             LP_SM_MSG_ID = 15005
	LP_SM_MSG_ID_SM_PLAYER_INFO                 LP_SM_MSG_ID = 15006
	LP_SM_MSG_ID_SM_PONG                        LP_SM_MSG_ID = 15007
	LP_SM_MSG_ID_SM_ITEM_INFO                   LP_SM_MSG_ID = 15008
	LP_SM_MSG_ID_SM_RECORD_INFO                 LP_SM_MSG_ID = 15009
	LP_SM_MSG_ID_SM_HERO_INFO                   LP_SM_MSG_ID = 15010
	LP_SM_MSG_ID_SM_BATTLE_DUNGEON_INFO         LP_SM_MSG_ID = 15011
	LP_SM_MSG_ID_SM_BATTLE_SET_HERO_INFO        LP_SM_MSG_ID = 15012
	LP_SM_MSG_ID_SM_BATTLE_ACTION               LP_SM_MSG_ID = 15013
	LP_SM_MSG_ID_SM_PVE_DUNGEON_FINISH          LP_SM_MSG_ID = 15014
	LP_SM_MSG_ID_SM_LOGIN_ERROR                 LP_SM_MSG_ID = 15015
	LP_SM_MSG_ID_SM_CREATE_ROLE_ERROR           LP_SM_MSG_ID = 15016
	LP_SM_MSG_ID_SM_GET_ITEMS                   LP_SM_MSG_ID = 15017
	LP_SM_MSG_ID_SM_DUNGEON_INFO                LP_SM_MSG_ID = 15030
	LP_SM_MSG_ID_SM_DRAW_COUNT                  LP_SM_MSG_ID = 15040
	LP_SM_MSG_ID_SM_DRAW_CARD_REWARD            LP_SM_MSG_ID = 15041
	LP_SM_MSG_ID_SM_SHOP_LIMIT_INFO             LP_SM_MSG_ID = 15050
	LP_SM_MSG_ID_SM_REPLY_RES                   LP_SM_MSG_ID = 15055
	LP_SM_MSG_ID_SM_TASK_INFO                   LP_SM_MSG_ID = 15060
	LP_SM_MSG_ID_SM_ACHIEVEMENT_INFO            LP_SM_MSG_ID = 15061
	LP_SM_MSG_ID_SM_MAIL_INFO                   LP_SM_MSG_ID = 15070
	LP_SM_MSG_ID_SM_MAIL_DELETE                 LP_SM_MSG_ID = 15071
	LP_SM_MSG_ID_SM_EQUIP_INFO                  LP_SM_MSG_ID = 15080
	LP_SM_MSG_ID_SM_GEM_INFO                    LP_SM_MSG_ID = 15081
	LP_SM_MSG_ID_SM_EQUIP_DELETE                LP_SM_MSG_ID = 15082
	LP_SM_MSG_ID_SM_GEM_DELETE                  LP_SM_MSG_ID = 15083
	LP_SM_MSG_ID_SM_HERO_TEAM                   LP_SM_MSG_ID = 15090
	LP_SM_MSG_ID_SM_CLIENT_RECORD               LP_SM_MSG_ID = 15095
	LP_SM_MSG_ID_SM_RELATION_PLAYER_SIMPLE_DATA LP_SM_MSG_ID = 15100
	LP_SM_MSG_ID_SM_RELATION_INFO               LP_SM_MSG_ID = 15101
	LP_SM_MSG_ID_SM_RELATION_RECOMMEND          LP_SM_MSG_ID = 15102
	LP_SM_MSG_ID_SM_RELATION_ONLINE_STATE       LP_SM_MSG_ID = 15103
	LP_SM_MSG_ID_SM_CHAT_INFO                   LP_SM_MSG_ID = 15121
	LP_SM_MSG_ID_SM_GET_BATTLE_DATA             LP_SM_MSG_ID = 16001
	LP_SM_MSG_ID_SM_END                         LP_SM_MSG_ID = 20000
)

var LP_SM_MSG_ID_name = map[int32]string{
	0:     "SM_MSG_NONE",
	15000: "SM_BEGIN",
	15001: "SM_LOGIN_RES",
	15002: "SM_ENTER_ROOM",
	15003: "SM_CREATE_ROLE",
	15004: "SM_SELF_ROLE_INFO",
	15005: "SM_PLAYER_ALL_INFO",
	15006: "SM_PLAYER_INFO",
	15007: "SM_PONG",
	15008: "SM_ITEM_INFO",
	15009: "SM_RECORD_INFO",
	15010: "SM_HERO_INFO",
	15011: "SM_BATTLE_DUNGEON_INFO",
	15012: "SM_BATTLE_SET_HERO_INFO",
	15013: "SM_BATTLE_ACTION",
	15014: "SM_PVE_DUNGEON_FINISH",
	15015: "SM_LOGIN_ERROR",
	15016: "SM_CREATE_ROLE_ERROR",
	15017: "SM_GET_ITEMS",
	15030: "SM_DUNGEON_INFO",
	15040: "SM_DRAW_COUNT",
	15041: "SM_DRAW_CARD_REWARD",
	15050: "SM_SHOP_LIMIT_INFO",
	15055: "SM_REPLY_RES",
	15060: "SM_TASK_INFO",
	15061: "SM_ACHIEVEMENT_INFO",
	15070: "SM_MAIL_INFO",
	15071: "SM_MAIL_DELETE",
	15080: "SM_EQUIP_INFO",
	15081: "SM_GEM_INFO",
	15082: "SM_EQUIP_DELETE",
	15083: "SM_GEM_DELETE",
	15090: "SM_HERO_TEAM",
	15095: "SM_CLIENT_RECORD",
	15100: "SM_RELATION_PLAYER_SIMPLE_DATA",
	15101: "SM_RELATION_INFO",
	15102: "SM_RELATION_RECOMMEND",
	15103: "SM_RELATION_ONLINE_STATE",
	15121: "SM_CHAT_INFO",
	16001: "SM_GET_BATTLE_DATA",
	20000: "SM_END",
}

var LP_SM_MSG_ID_value = map[string]int32{
	"SM_MSG_NONE":                    0,
	"SM_BEGIN":                       15000,
	"SM_LOGIN_RES":                   15001,
	"SM_ENTER_ROOM":                  15002,
	"SM_CREATE_ROLE":                 15003,
	"SM_SELF_ROLE_INFO":              15004,
	"SM_PLAYER_ALL_INFO":             15005,
	"SM_PLAYER_INFO":                 15006,
	"SM_PONG":                        15007,
	"SM_ITEM_INFO":                   15008,
	"SM_RECORD_INFO":                 15009,
	"SM_HERO_INFO":                   15010,
	"SM_BATTLE_DUNGEON_INFO":         15011,
	"SM_BATTLE_SET_HERO_INFO":        15012,
	"SM_BATTLE_ACTION":               15013,
	"SM_PVE_DUNGEON_FINISH":          15014,
	"SM_LOGIN_ERROR":                 15015,
	"SM_CREATE_ROLE_ERROR":           15016,
	"SM_GET_ITEMS":                   15017,
	"SM_DUNGEON_INFO":                15030,
	"SM_DRAW_COUNT":                  15040,
	"SM_DRAW_CARD_REWARD":            15041,
	"SM_SHOP_LIMIT_INFO":             15050,
	"SM_REPLY_RES":                   15055,
	"SM_TASK_INFO":                   15060,
	"SM_ACHIEVEMENT_INFO":            15061,
	"SM_MAIL_INFO":                   15070,
	"SM_MAIL_DELETE":                 15071,
	"SM_EQUIP_INFO":                  15080,
	"SM_GEM_INFO":                    15081,
	"SM_EQUIP_DELETE":                15082,
	"SM_GEM_DELETE":                  15083,
	"SM_HERO_TEAM":                   15090,
	"SM_CLIENT_RECORD":               15095,
	"SM_RELATION_PLAYER_SIMPLE_DATA": 15100,
	"SM_RELATION_INFO":               15101,
	"SM_RELATION_RECOMMEND":          15102,
	"SM_RELATION_ONLINE_STATE":       15103,
	"SM_CHAT_INFO":                   15121,
	"SM_GET_BATTLE_DATA":             16001,
	"SM_END":                         20000,
}

func (x LP_SM_MSG_ID) String() string {
	return proto.EnumName(LP_SM_MSG_ID_name, int32(x))
}

func (LP_SM_MSG_ID) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_f7e38f9743a0f071, []int{1}
}

func init() {
	proto.RegisterEnum("LPMsg.LP_CM_MSG_ID", LP_CM_MSG_ID_name, LP_CM_MSG_ID_value)
	proto.RegisterEnum("LPMsg.LP_SM_MSG_ID", LP_SM_MSG_ID_name, LP_SM_MSG_ID_value)
}

func init() { proto.RegisterFile("define.proto", fileDescriptor_f7e38f9743a0f071) }

var fileDescriptor_f7e38f9743a0f071 = []byte{
	// 1116 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x6c, 0x56, 0x39, 0x8f, 0x1d, 0x45,
	0x10, 0x86, 0x00, 0x1b, 0xb5, 0xd7, 0xb8, 0xdc, 0xbe, 0x31, 0x18, 0x24, 0x88, 0x1c, 0x90, 0xf0,
	0x0b, 0x7a, 0x67, 0x6a, 0xe7, 0xb5, 0xb6, 0x8f, 0xe7, 0xe9, 0x7e, 0x36, 0x1b, 0xb5, 0x84, 0x30,
	0x88, 0x04, 0x23, 0x60, 0xc8, 0x08, 0xc8, 0xb8, 0xf1, 0x41, 0xe0, 0xfb, 0x04, 0x6c, 0x12, 0x42,
	0x04, 0x19, 0x77, 0x80, 0x38, 0x12, 0x4e, 0x71, 0x5f, 0x12, 0x18, 0x91, 0x20, 0x10, 0x20, 0x81,
	0x41, 0xd5, 0x33, 0x35, 0x7e, 0xf3, 0x76, 0xc3, 0x57, 0x5f, 0x55, 0x4d, 0x55, 0x7d, 0x55, 0x5f,
	0x3f, 0x31, 0x77, 0xc7, 0xde, 0x3b, 0xef, 0xbe, 0x67, 0xef, 0x2d, 0xf7, 0xde, 0xb7, 0xef, 0x81,
	0x7d, 0xf2, 0x2a, 0x33, 0xb6, 0xf7, 0xdf, 0xb5, 0xf3, 0xd7, 0x35, 0x62, 0xce, 0x8c, 0x53, 0x61,
	0x93, 0x0d, 0x55, 0xd2, 0xa5, 0x5c, 0x27, 0xd6, 0x74, 0x3f, 0x9c, 0x77, 0x08, 0x57, 0xc8, 0xb5,
	0xe2, 0xea, 0xc2, 0xa6, 0x79, 0xac, 0x94, 0x83, 0xfd, 0xae, 0xfb, 0x69, 0x7c, 0xa5, 0x1d, 0x1c,
	0x70, 0x52, 0x8a, 0xb5, 0x85, 0x4d, 0xe8, 0x22, 0xd6, 0xa9, 0xf6, 0xde, 0xc2, 0x41, 0x27, 0x37,
	0x88, 0x6b, 0x0a, 0x9b, 0x8a, 0x1a, 0x55, 0xc4, 0x54, 0x7b, 0x83, 0x70, 0xc8, 0xc9, 0x39, 0xb1,
	0xba, 0xb0, 0x69, 0xac, 0x5d, 0x05, 0xcf, 0x0c, 0xc3, 0x2a, 0x65, 0x11, 0x8e, 0x38, 0xb9, 0x59,
	0xac, 0x2f, 0x6c, 0xd2, 0x11, 0x6d, 0x9a, 0xb8, 0x80, 0x31, 0x39, 0xdc, 0x03, 0x47, 0x9d, 0xdc,
	0x24, 0xa0, 0xf7, 0x2d, 0x27, 0xae, 0x42, 0xef, 0xe0, 0x98, 0x93, 0x5b, 0x84, 0xa4, 0xba, 0x54,
	0x8c, 0x06, 0x13, 0xb9, 0x8f, 0xb0, 0xf6, 0x70, 0xdc, 0xc9, 0xed, 0x62, 0x73, 0x61, 0xd3, 0xae,
	0x89, 0x8e, 0x8c, 0x72, 0xd4, 0x09, 0x4e, 0xd6, 0xd9, 0x55, 0x11, 0xb5, 0x77, 0x70, 0xd2, 0xc9,
	0x6b, 0xc5, 0x26, 0xaa, 0x6e, 0x77, 0xef, 0x9b, 0x16, 0xb4, 0xd3, 0x61, 0x04, 0xa7, 0x18, 0xab,
	0x30, 0xa6, 0x62, 0xa4, 0xc6, 0xb9, 0x51, 0xdc, 0xa3, 0xea, 0x12, 0x4e, 0x3b, 0xb9, 0x4d, 0x6c,
	0xbc, 0xdc, 0x3e, 0x86, 0xfe, 0x4b, 0x67, 0xb8, 0x8c, 0xa0, 0x7c, 0xa9, 0x5c, 0x35, 0x00, 0xcf,
	0x72, 0xf1, 0x0c, 0x32, 0x70, 0x8e, 0xc7, 0x5b, 0xe2, 0xfc, 0xa4, 0x82, 0xf3, 0x4e, 0x42, 0x66,
	0xa3, 0x9d, 0x49, 0x40, 0xb8, 0xe0, 0xe4, 0x7a, 0x31, 0x47, 0x0e, 0xb5, 0xda, 0x93, 0x0a, 0x2a,
	0xe2, 0x45, 0xee, 0x29, 0x8c, 0xfc, 0x38, 0x59, 0x65, 0x4c, 0x9a, 0x9f, 0x2c, 0xc1, 0xcb, 0xec,
	0x19, 0xd0, 0x98, 0x9c, 0x00, 0x5e, 0xe1, 0x72, 0x69, 0x50, 0x6d, 0x4e, 0x55, 0x96, 0x09, 0x6f,
	0x1b, 0xc3, 0xeb, 0x9c, 0x24, 0x43, 0xbe, 0x2e, 0xb1, 0x4e, 0x93, 0x31, 0xbc, 0xc1, 0xa4, 0x64,
	0x73, 0x54, 0x06, 0x5d, 0x24, 0xfb, 0x9b, 0x4e, 0x5e, 0x2f, 0xb6, 0xce, 0xd8, 0x89, 0x82, 0xb0,
	0xa8, 0x8d, 0x81, 0xb7, 0x06, 0xd9, 0xb2, 0x8d, 0xa2, 0xde, 0xe6, 0xb6, 0x69, 0x94, 0x51, 0x85,
	0x45, 0x9e, 0xe3, 0xbb, 0x4e, 0xde, 0x2c, 0x6e, 0xe8, 0x80, 0x52, 0x69, 0xb3, 0xd4, 0xc2, 0x44,
	0xcf, 0x6e, 0x64, 0xaf, 0xf7, 0x9c, 0xdc, 0x21, 0xb6, 0x15, 0x36, 0xa9, 0x62, 0xa4, 0x71, 0x37,
	0x5a, 0xfa, 0x28, 0x45, 0x74, 0xf8, 0xfb, 0xdc, 0xb1, 0x55, 0xda, 0xa4, 0x1a, 0x55, 0x09, 0x1f,
	0xf3, 0x00, 0xb3, 0xa9, 0xc2, 0x08, 0x9f, 0xf0, 0x76, 0x66, 0x4b, 0x89, 0x06, 0x23, 0xc2, 0xa7,
	0xec, 0x36, 0x3f, 0x59, 0x4a, 0x51, 0x1b, 0x0d, 0x9f, 0xf7, 0x4b, 0x3c, 0x52, 0xae, 0xc2, 0xe4,
	0x68, 0x45, 0xbf, 0xe8, 0x27, 0x4d, 0x9d, 0xea, 0xca, 0xa9, 0x38, 0xa9, 0x11, 0xbe, 0x1c, 0x8c,
	0xb5, 0x0b, 0xc0, 0x5d, 0x13, 0x3d, 0x86, 0x6f, 0xb8, 0xe3, 0x69, 0xa8, 0x42, 0x0b, 0xdf, 0xf6,
	0x5b, 0x4d, 0x7e, 0x3d, 0x0d, 0xdf, 0x0d, 0xcd, 0x21, 0xaa, 0x4c, 0xc3, 0xf7, 0x5c, 0x4d, 0x35,
	0x45, 0xd9, 0x0f, 0xdc, 0x6e, 0x26, 0xd2, 0xf8, 0x62, 0x11, 0x7e, 0xe4, 0x3e, 0xda, 0x6b, 0x50,
	0x25, 0x5c, 0x9c, 0x2e, 0xb9, 0x25, 0x0b, 0x95, 0x85, 0xdf, 0x79, 0xa9, 0x07, 0xe6, 0xb6, 0xcb,
	0x3f, 0xb8, 0x1d, 0xc2, 0x0a, 0xa3, 0x69, 0xca, 0x35, 0x16, 0xbe, 0x2e, 0xe1, 0x12, 0x33, 0x50,
	0xa3, 0x51, 0x74, 0x39, 0x29, 0xa0, 0xaa, 0x8b, 0x51, 0x1a, 0x1b, 0xb5, 0x84, 0x35, 0x3c, 0xe2,
	0xbb, 0xb4, 0x3d, 0x4e, 0x81, 0xd6, 0xa2, 0x2b, 0xe1, 0x51, 0xdf, 0xad, 0x4c, 0x8f, 0xa9, 0xf1,
	0xd8, 0x2c, 0xa5, 0x85, 0x5a, 0x13, 0xfc, 0xd8, 0x32, 0xb8, 0x05, 0x92, 0xaa, 0x6a, 0x44, 0x78,
	0xdc, 0xcb, 0x1b, 0xc5, 0xf6, 0x15, 0xe0, 0x52, 0x87, 0xd6, 0xe3, 0x09, 0x3f, 0x5b, 0x1b, 0x7b,
	0xb4, 0x1c, 0x3f, 0xb9, 0xac, 0x36, 0xea, 0x6f, 0xde, 0xa8, 0x62, 0x11, 0x9e, 0x5a, 0xf6, 0xf1,
	0x6c, 0xe7, 0xd0, 0xa7, 0x39, 0x35, 0x2d, 0x5b, 0xdb, 0x6a, 0x0a, 0xda, 0x8e, 0x0d, 0x26, 0xed,
	0x16, 0x3c, 0xec, 0x5f, 0x31, 0xb5, 0xf3, 0x11, 0x03, 0x1c, 0xf0, 0xf2, 0x3a, 0xb1, 0x65, 0x88,
	0xb9, 0x92, 0x57, 0xf6, 0xa0, 0xef, 0x54, 0xa2, 0x47, 0xa7, 0xf6, 0xf9, 0x90, 0xef, 0x2f, 0xd8,
	0x95, 0xb4, 0x3c, 0x11, 0x3e, 0xf3, 0x72, 0xab, 0xd8, 0x50, 0xd8, 0x14, 0x31, 0x5c, 0x16, 0x37,
	0x15, 0x15, 0xfc, 0x35, 0x99, 0xba, 0xad, 0x69, 0xe0, 0xef, 0x89, 0x5c, 0x23, 0x56, 0x65, 0x8d,
	0x2a, 0xe1, 0x70, 0xb3, 0xf3, 0xe8, 0xea, 0xac, 0xf7, 0x61, 0x5a, 0xef, 0xc3, 0xac, 0xde, 0x87,
	0xac, 0xf7, 0xda, 0xc1, 0xe1, 0x86, 0x6a, 0x08, 0x9d, 0xde, 0x93, 0x88, 0xc1, 0x91, 0x86, 0xc4,
	0x3b, 0x0c, 0x34, 0xff, 0x68, 0x43, 0x0b, 0x1a, 0x86, 0x9a, 0x7f, 0xac, 0x21, 0xf1, 0x08, 0x59,
	0x81, 0x16, 0xb2, 0xa9, 0x1d, 0xd7, 0xf1, 0x86, 0x4a, 0x0d, 0x96, 0x47, 0x49, 0x8a, 0x95, 0x81,
	0x13, 0x9c, 0xa5, 0x03, 0xb2, 0xf1, 0x64, 0x43, 0x2f, 0x07, 0x19, 0xbd, 0xab, 0xe0, 0x14, 0xd7,
	0x93, 0x97, 0x3e, 0x3b, 0x9c, 0xe6, 0xa8, 0x76, 0x49, 0x5b, 0xe3, 0x19, 0xf6, 0x6b, 0xa5, 0x8e,
	0x4c, 0x67, 0x1b, 0x9a, 0x75, 0xb0, 0x33, 0x6f, 0x42, 0x0b, 0x9e, 0x6b, 0x88, 0xa6, 0xb0, 0xec,
	0x39, 0x69, 0xd1, 0x67, 0x1b, 0xba, 0xa2, 0x30, 0xfb, 0x6c, 0x3c, 0xd7, 0x10, 0xef, 0x61, 0xc5,
	0x67, 0xe3, 0x79, 0xae, 0xaa, 0x1d, 0x1c, 0xd6, 0xb5, 0xaf, 0xe1, 0x7c, 0x43, 0xa7, 0x35, 0x1c,
	0x53, 0x07, 0x5d, 0xe0, 0x82, 0x89, 0x3f, 0x6a, 0x2e, 0xc0, 0x0b, 0x8d, 0xdc, 0x28, 0xd6, 0x05,
	0x3b, 0xac, 0xf4, 0x25, 0x1e, 0x7f, 0xfb, 0x02, 0xf8, 0x89, 0x8b, 0xf0, 0x6a, 0x43, 0x6b, 0x11,
	0xa6, 0x5e, 0x05, 0xde, 0xa1, 0xd7, 0x78, 0xd6, 0xf9, 0x71, 0x30, 0xda, 0xea, 0xd8, 0xa6, 0x79,
	0x87, 0xbf, 0x57, 0x23, 0x5d, 0x21, 0x11, 0xfb, 0x01, 0x9b, 0xb2, 0xf6, 0x66, 0xaf, 0x0f, 0x39,
	0xf1, 0xb4, 0xe4, 0x66, 0xe4, 0x23, 0x76, 0xce, 0x3a, 0x9a, 0x4d, 0x5f, 0x71, 0xcb, 0xd3, 0xd2,
	0xfa, 0x75, 0xbf, 0x2d, 0x59, 0xd1, 0xb2, 0xe3, 0x4f, 0x0d, 0xc9, 0x54, 0x68, 0xe5, 0x2c, 0x5b,
	0x7e, 0xe6, 0x56, 0x5b, 0xaf, 0x2e, 0xf6, 0x22, 0xc7, 0x92, 0x5f, 0x67, 0xfb, 0x65, 0x40, 0x6c,
	0x16, 0xb3, 0xdf, 0x98, 0x9d, 0xa1, 0x58, 0xfd, 0xd9, 0xc8, 0x9b, 0xc4, 0x8e, 0x30, 0x75, 0x5b,
	0xc3, 0xd3, 0xcd, 0xd7, 0xf1, 0x0f, 0xc7, 0xf6, 0x4e, 0xb9, 0x9e, 0x7f, 0x99, 0xd9, 0x15, 0x84,
	0xec, 0x52, 0x43, 0x62, 0x31, 0x8d, 0x79, 0x67, 0xb4, 0x43, 0x52, 0xeb, 0x88, 0xf0, 0x1f, 0x17,
	0x48, 0x07, 0xdb, 0x66, 0x3b, 0xf0, 0x60, 0x47, 0xc2, 0xec, 0x6d, 0x3e, 0xfc, 0x90, 0x9c, 0x13,
	0xab, 0x42, 0x7b, 0x9b, 0xa7, 0x8f, 0x5f, 0x79, 0xfb, 0xaa, 0xfc, 0xd7, 0xec, 0xd6, 0xff, 0x03,
	0x00, 0x00, 0xff, 0xff, 0x03, 0x08, 0x81, 0x24, 0xaa, 0x09, 0x00, 0x00,
}
