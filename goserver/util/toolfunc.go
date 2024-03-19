package util

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
)

func GetSecond() int64 {
	return time.Now().Unix()
}

func GetMillisecond() int64 {
	return time.Now().UnixNano() / 1e6
}

func NowTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

var confdata = make(map[string]string)

func SetConfValue(f string, v string) {
	confdata[f] = v
}

func GetConfValue(f string) string {
	return confdata[f]
}

func GetConfInt(f string) int {
	res, _ := strconv.Atoi(confdata[f])
	return res
}

func parseLine(str string) {
	reg := regexp.MustCompile("([\\w-]*)=(.*)")
	sarr := reg.FindStringSubmatch(str)
	if len(sarr) == 3 {
		SetConfValue(sarr[1], sarr[2])
	}
}

func Readconf(fname string) {
	f, err := os.Open(fname)
	if err != nil {
		Log_info("err=%s", err.Error())
		return
	}

	defer f.Close()

	reader := bufio.NewReader(f)

	for {
		str, _, err := reader.ReadLine()

		if len(str) > 0 && str[0] != '#' {
			parseLine(string(str))
		}

		if err == io.EOF {
			break
		}
	}

}

var logerr *log.Logger
var logwaring *log.Logger
var loginfo *log.Logger

func Init_loger(fname string) {
	dir := "./log"
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			log.Fatal(err)
		}
	}

	datestr := time.Now().Format("2006-01-02")
	fname = fmt.Sprintf("%s/%s_%d_%s.log", dir, fname, GetSelfServerId(), datestr)
	lf, err := os.OpenFile(fname, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		panic(err)
	}
	logerr = log.New(lf, "[error]", log.Ldate|log.Ltime|log.Lshortfile)
	logwaring = log.New(lf, "[waring]", log.Ldate|log.Ltime|log.Lshortfile)
	loginfo = log.New(lf, "[info]", log.Ldate|log.Ltime|log.Lshortfile)
}

var sysType = runtime.GOOS

func Log_error(sfmt string, a ...interface{}) {
	_, filePath, lineNum, _ := runtime.Caller(1) // 第二个参数为0表示当前函数，所以传入1
	fileName := filepath.Base(filePath)
	sfmt = fmt.Sprintf("%s:%d ", fileName, lineNum) + sfmt
	if sysType == "windows" {
		fmt.Printf(sfmt+"\n", a...)
	}
	// log.Printf(sfmt, a...)
	logerr.Printf(sfmt+"\n", a...)
}

func Log_waring(sfmt string, a ...interface{}) {
	_, filePath, lineNum, _ := runtime.Caller(1) // 第二个参数为0表示当前函数，所以传入1
	fileName := filepath.Base(filePath)
	sfmt = fmt.Sprintf("%s:%d ", fileName, lineNum) + sfmt
	if sysType == "windows" {
		fmt.Printf(sfmt+"\n", a...)
	}
	// log.Printf(sfmt, a...)
	logwaring.Printf(sfmt+"\n", a...)
}

func Log_info(sfmt string, a ...interface{}) {
	_, filePath, lineNum, _ := runtime.Caller(1) // 第二个参数为0表示当前函数，所以传入1
	fileName := filepath.Base(filePath)
	sfmt = fmt.Sprintf("%s:%d ", fileName, lineNum) + sfmt
	if sysType == "windows" {
		fmt.Printf(sfmt+"\n", a...)
	}
	// log.Printf(sfmt, a...)
	loginfo.Printf(sfmt+"\n", a...)
}

var selfServerType int
var selfServerId int

func SetServer(stype int, sid int) {
	selfServerType = stype
	selfServerId = sid
}

func GetSelfServer() (int, int) {
	return selfServerId, selfServerType
}

func GetSelfServerId() int {
	return selfServerId
}

func ParseArgsId(stype int) {
	if len(os.Args) != 2 {
		fmt.Print("args error needid")
		os.Exit(1)
	}

	sid, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Print(err.Error())
		os.Exit(1)
	}

	SetServer(stype, sid)
}

func NewSqlConn(host string) *sqlx.DB {
	sql, sqlerr := sqlx.Open("mysql", host)
	if sqlerr != nil {
		Log_error("sql open error:%s", sqlerr.Error())
		return nil
	}
	Log_info("conn mysql success %s\n", host)
	sql.SetMaxOpenConns(10)
	// 闲置连接数
	sql.SetMaxIdleConns(5)
	// 最大连接周期
	sql.SetConnMaxLifetime(100 * time.Second)
	return sql
}
