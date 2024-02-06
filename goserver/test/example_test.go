package test

import (
	"bytes"
	"container/list"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"goserver/module"
	"goserver/util"
	"hash/crc32"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"testing"

	"github.com/qiniu/qmgo"
	qmoption "github.com/qiniu/qmgo/options"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/yaml.v2"
)

func Test_Example(t *testing.T) {

	buf := new(bytes.Buffer)
	for i := 'a'; i <= 'z'; i++ {
		buf.WriteRune(i)
	}
	for i := 'A'; i <= 'Z'; i++ {
		buf.WriteRune(i)
	}
	for i := '0'; i <= '9'; i++ {
		buf.WriteRune(i)
	}

	str := buf.String()
	// fmt.Println(buf.String())
	// fmt.Println(byte(str[5]))

	const ARR_NUM uint32 = 32
	var arrnum [ARR_NUM]uint32
	var multnum = (^uint32(0)) / ARR_NUM
	var num = 100000

	for i := 0; i < num; i++ {
		var rlen = rand.Intn(11) + 5
		buf.Reset()
		for j := 0; j < rlen; j++ {
			buf.WriteByte(str[rand.Intn(len(str))])
		}

		_hval := crc32.ChecksumIEEE(buf.Bytes())
		// arrnum[_hval%32]++
		arrnum[_hval/multnum]++
	}

	for i := uint32(0); i < ARR_NUM; i++ {
		util.Log_info("index %d num -> %d\n", i, arrnum[i]*100/uint32(num))
	}
}

type TestCall struct {
	_i   int
	_map map[int]int
}

func (_t *TestCall) callClass() {
	fmt.Println(_t._i)
}

type callback func()

func Test_2(t *testing.T) {

	_eee := TestCall{}
	fmt.Println("mmm", _eee._map)

	_map := make(map[int]*list.List)
	_map[1] = new(list.List)
	l1 := _map[1]
	l1.PushBack(111)

	l2 := _map[1]
	fmt.Println(l2.Front().Value)
}

func Test_3(t *testing.T) {
	// var eve gnet.EventHandler = &gnet.EventServer{}
	// gnet.Serve(eve, "tcp://127.0.0.1:11111")

	ws := sync.WaitGroup{}

	_sc := make(chan int, 10)
	ws.Add(1)

	go func() {
		for i := 0; i < 20; i++ {
			_sc <- i
		}
		close(_sc)
		ws.Done()
	}()

	ws.Add(1)
	go func() {
		for v := range _sc {
			fmt.Println(v)
		}
		fmt.Println("chan close")
		ws.Done()
	}()

	ws.Wait()
}

type ibase1 interface {
	call()
}

type ibase2 interface {
	show()
}

type tba struct {
	val int
	vec []int
}

func (a tba) call(ib ibase2) {
	println("call tba")
	ib.show()
}

func (a *tba) show() {
	println("show tba")
}

type tbb struct {
	tba
}

func (b *tbb) show() {
	println("show tbb -----")
}

func returnself(v interface{}) interface{} {
	return v
}

func Test_inter(t *testing.T) {
	aaa := tba{
		val: 111,
		vec: []int{1, 2, 3, 4},
	}
	bbb := aaa
	bbb.val = 222
	bbb.vec[3] = 444
	v1 := aaa.vec
	v2 := v1
	v3 := returnself(v1).([]int)
	v2[0] = 11

	func() {
		defer fmt.Println("defer log")
		fmt.Println("in log")
	}()

	fmt.Println("2222 log")

	fmt.Println(aaa)
	fmt.Println(bbb)
	fmt.Println(v1)
	fmt.Println(v2)
	fmt.Println(v3)
}

func Test_defer(t *testing.T) {
	fmt.Println("1")
	func() {
		fmt.Println("2")
		defer fmt.Println("3")
		fmt.Println("4")
	}()

	fmt.Println("5")
}

func Test_md5(t *testing.T) {
	md := md5.Sum([]byte("asdjhkl"))
	str := hex.EncodeToString(md[:])
	fmt.Print(str)
}

func Test_yaml(t *testing.T) {
	ser := module.CService{
		Image:       "goserver:v0.0.1r",
		Command:     []string{"./service_find", "1"},
		NetworkMode: "host",
		Ports:       []string{"40001:40001", "8991:8991"},
		Volumes:     []string{"./conf.ini:/root/server/conf.ini", "./log:/root/server/log"},
	}

	gen := module.DCompose{
		Services: make(map[string]module.CService),
	}
	gen.Services["service_find_1"] = ser

	fname := "./test.yaml"
	fh, err := os.OpenFile(fname, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Print(err)
		return
	}

	ystr, err := yaml.Marshal(gen)
	if err != nil {
		fmt.Print(err)
		return
	}

	_, err = fh.Write(ystr)
	if err != nil {
		fmt.Print(err)
		return
	}
	fh.Close()
}

func Test_phonenum(t *testing.T) {
	pnum := "15757181904"
	mobregex := `^1[3-9]\d{9}$`
	reg := regexp.MustCompile(mobregex)
	ok := reg.MatchString(pnum)
	print(ok)
}

func Test_defer2(t *testing.T) {
	f1 := func(n int) {
		fmt.Println("defer", n)
	}

	if true {
		defer f1(2)
		f1(1)
	}
	f1(3)
}

func clusure1() func() {
	str := "123456"
	return func() {
		fmt.Println(str)
	}
}

func Test_clusure(*testing.T) {
	f1 := clusure1()
	f1()
}

func Test_defStruct(*testing.T) {
	var tval tbb
	fmt.Print(tval)
}

type jbase struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

type jdata1 struct {
	Cid  int    `json:"cid"`
	Name string `json:"name"`
}

func Test_json(*testing.T) {
	jb := jbase{
		Data: jdata1{},
	}
	str, err := json.Marshal(jb)
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Println(string(str))
}

var qmtimeoutms = int64(2000)

func createMongoClient(host string, db string) *qmgo.Client {
	ops := qmoption.ClientOptions{}
	cli, err := qmgo.NewClient(context.Background(), &qmgo.Config{
		Uri:              host,
		ConnectTimeoutMS: &qmtimeoutms,
	}, ops)
	if err != nil {
		panic(err)
	}
	return cli
}

type mg_location struct {
	Type        string    `bson:"type"`
	Coordinates []float64 `bson:"coordinates"`
}

type mg_people struct {
	ObjectID primitive.ObjectID `bson:"_id"`
	Id       int                `bson:"id"`
	Cid      int                `bson:"cid"`
	Title    string             `bson:"title"`
	Name     string             `bson:"name"`
	Location mg_location        `bson:"location"`
}

func Test_mongo(*testing.T) {
	cli := createMongoClient("mongodb://localhost:27017", "test")
	coll := cli.Database("test").Collection("people")
	res, err := coll.InsertOne(context.Background(), mg_people{
		ObjectID: qmgo.NewObjectID(),
		Id:       1,
		Cid:      1,
		Title:    "111",
		Name:     "222",
		Location: mg_location{
			Type:        "Point",
			Coordinates: []float64{10.2, 10.2},
		},
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(res.InsertedID)
}

func Test_mongo_select(*testing.T) {
	cli := createMongoClient("mongodb://localhost:27017", "test")
	coll := cli.Database("test").Collection("people")
	var res mg_people
	objid, _ := primitive.ObjectIDFromHex("65a25f3068d49ae6724b22a1")

	// opts := options.FindOne().SetProjection(bson.M{"title":1})
	// qmopt := qmoption.FindOptions{}
	err := coll.Find(context.Background(), bson.M{
		"_id": objid,
	}).Select(bson.M{"title": 1, "cid": 1}).One(&res)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(res)

}

func Test_mongo_select_geo(*testing.T) {
	cli := createMongoClient("mongodb://localhost:27017", "test")
	coll := cli.Database("test").Collection("people")
	var res []mg_people
	location := mg_location{
		Type:        "Point",
		Coordinates: []float64{10.2, 10.2},
	}
	filter := bson.D{
		{Key: "location",
			Value: bson.D{
				{Key: "$near", Value: bson.D{
					{Key: "$geometry", Value: location},
					{Key: "$maxDistance", Value: 1000},
				}},
			}},
	}
	err := coll.Find(context.Background(), filter).All(&res)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(res)

}

func createJoinTask(taskid string) module.My_join_task {
	objid, _ := primitive.ObjectIDFromHex(taskid)
	return module.My_join_task{
		Id:   objid,
		Time: util.GetSecond(),
	}
}

func Test_mongo_array(*testing.T) {
	cli := createMongoClient("mongodb://localhost:27017", "test")
	coll := cli.Database("test").Collection("user_task")

	usertask := module.UserTask{
		Id:  qmgo.NewObjectID(),
		Cid: 109153824768,
		TaskList: []module.My_join_task{
			createJoinTask("6583d9c4ab8ebe31ab77b6a3"),
			createJoinTask("65840e6bab8ebe31ab77b6a5"),
			createJoinTask("65840e7dab8ebe31ab77b6a7"),
		},
	}
	ctx := context.Background()
	_, err := coll.InsertOne(ctx, usertask)
	if err != nil {
		fmt.Println(err.Error())
	}
}

func genTaskObjId(id string) *primitive.ObjectID {
	objid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		util.Log_error("taskid err: %s", err.Error())
		return nil
	}
	return &objid
}

func Test_mongo_update_or_insert(*testing.T) {
	cli := createMongoClient("mongodb://localhost:27017", "test")
	coll := cli.Database("test").Collection("user_task")

	upopts := options.Update().SetUpsert(true)
	qmopt := qmoption.UpdateOptions{UpdateHook: nil, UpdateOptions: upopts}

	joininfo := module.UserJoinTask{
		Id:   *genTaskObjId("6583d9c4ab8ebe31ab77b6a3"),
		Time: 0,
	}

	ctx := context.Background()
	err := coll.UpdateOne(ctx, bson.M{"cid": 109153824768}, bson.M{"$push": bson.M{"tasklist": joininfo}}, qmopt)
	if err != nil {
		fmt.Println(err.Error())
	}
}

func Test_for_defer(*testing.T) {
	// for i := 0; i < 10; i++ {
	// 	defer fmt.Println(fmt.Sprintf("11111 %d", i))
	// 	fmt.Println(i)
	// }
	path, err := os.Executable()
	if err != nil {
		// log.Fatal(err)
		fmt.Println(err.Error())
	}
	absPath, _ := filepath.Abs(path) // 转换为绝对路径
	fmt.Println("绝对路径：", absPath, filepath.Dir(absPath))
}

func Test_vec_remove(*testing.T) {
	vec := []int{1, 2, 3, 4}

	res := util.VectorRemoveNoSort[int](vec, func(i *int) bool {
		return *i == 2
	})
	fmt.Print(res)
}

func Test_string_len(*testing.T) {
	str := "中文123..."
	fmt.Print(len([]rune(str)))
}

type TestUser struct {
	ping int64
}

func sortUser(a, b interface{}) bool {
	ua := a.(*TestUser)
	ub := b.(*TestUser)
	return ua.ping > ub.ping
}

func showListOrder(l *util.ListOrder) {
	fmt.Println(" -------------------- ")
	arr := l.ToArray()
	for _, v := range arr {
		fmt.Println(v.(*TestUser))
	}
}

func Test_list_order(*testing.T) {
	ol := util.NewListOrder(sortUser)

	na := ol.Push(&TestUser{ping: 8})
	showListOrder(ol)
	nb := ol.Push(&TestUser{ping: 50})
	showListOrder(ol)
	nc := ol.Push(&TestUser{ping: 7})
	showListOrder(ol)
	nd := ol.Push(&TestUser{ping: 15})
	showListOrder(ol)
	n5 := ol.Push(&TestUser{ping: 100})
	showListOrder(ol)

	head := ol.GetFirst().(*TestUser)
	fmt.Println(head)

	nc.Value.(*TestUser).ping = 35
	ol.ResetBackOrder(nc)
	showListOrder(ol)

	nd.Remove()
	showListOrder(ol)
	nc.Remove()
	showListOrder(ol)
	n5.Remove()
	showListOrder(ol)
	na.Remove()
	showListOrder(ol)
	nb.Remove()
}
