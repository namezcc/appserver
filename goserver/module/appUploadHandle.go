package module

import (
	"context"
	"fmt"
	"goserver/util"
	"os"
	"path"

	"github.com/gin-gonic/gin"
	"github.com/qiniu/qmgo"
	"go.mongodb.org/mongo-driver/bson"
)

func apiUploadTaskImage(c *gin.Context, m *AppLoginModule) {
	hashval := c.MustGet("phoneHash").(uint32)
	taskid := c.PostForm("id")
	upname := c.PostFormArray("upname")

	task := m.GetTaskInfo(taskid)
	if task == nil {
		m.ResponesError(c, 1, "没有任务")
		return
	}

	if err := os.MkdirAll("./upload", 0777); err != nil {
		util.Log_error("upload image 3 err: %s", err.Error())
		m.ResponesError(c, 1, "服务器错误")
		return
	}

	nowsec := util.GetMillisecond()
	sendback := make([][]string, 0)
	for i, v := range upname {
		index := util.StringToInt(v)
		if index < 0 || index >= len(task.Images) {
			util.Log_error("upload image index err: %d", index)
			m.ResponesError(c, 1, "上传图片错误")
			return
		}

		header, err := c.FormFile(v)
		if err != nil {
			util.Log_error("upload image 4 err: %s", err.Error())
			m.ResponesError(c, 1, "上传图片错误")
			return
		}
		extName := path.Ext(header.Filename)
		filename := fmt.Sprintf("%s_%d%s", taskid, nowsec+int64(i), extName)
		newname := fmt.Sprintf("./upload/%s", filename)
		err = c.SaveUploadedFile(header, newname)
		if err != nil {
			util.Log_error("upload image 4 err: %s", err.Error())
			m.ResponesError(c, 1, "服务器错误")
			return
		}
		task.Images[index] = filename
		sendback = append(sendback, []string{v, filename})
	}
	// 更新图片名
	m._mg_mgr.RequestFuncCallNoResHash(hashval, func(ctx context.Context, qc *qmgo.QmgoClient) {
		coll := qc.Database.Collection("task")
		err := coll.UpdateOne(ctx, bson.M{"_id": task.Id}, bson.M{"$set": bson.M{"images": task.Images}})
		if err != nil {
			util.Log_error("upload image 5 err:%s", err.Error())
		}
	})
	m.ResponesJsonData(c, sendback)
}

func apiUploadOssImage(c *gin.Context, m *AppLoginModule) {
	if err := os.MkdirAll("./upload", 0777); err != nil {
		util.Log_error("upload oss image err: %s", err.Error())
		m.ResponesError(c, 1, "服务器错误")
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		util.Log_error("oss imgae 2 %s", err.Error())
		return
	}
	files := form.File["file"]
	for _, v := range files {
		oid := qmgo.NewObjectID().Hex()
		extName := path.Ext(v.Filename)
		fname := fmt.Sprintf("%s%s", oid, extName)
		newname := fmt.Sprintf("./upload/%s", fname)
		err := c.SaveUploadedFile(v, newname)
		if err != nil {
			util.Log_error("save file %s", err.Error())
			m.ResponesError(c, util.ERRCODE_ERROR, "error")
			return
		}
		m.ResponesJsonData(c, fname)
		return
	}
	m.ResponesError(c, util.ERRCODE_ERROR, "error")
}
