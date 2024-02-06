package module

import (
	"bytes"
	"encoding/json"
	"fmt"
	"goserver/util"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"strconv"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v2"
)

type yPort struct {
	Port          int `yaml:"port,omitempty"`
	TargetPort    int `yaml:"targetPort,omitempty"`
	ContainerPort int `yaml:"containerPort,omitempty"`
}

type yVolumes struct {
	Name      string            `yaml:"name"`
	MountPath string            `yaml:"mountPath,omitempty"`
	HostPath  map[string]string `yaml:"hostPath,omitempty"`
}

type yContainer struct {
	Name            string     `yaml:"name"`
	Image           string     `yaml:"image"`
	ImagePullPolicy string     `yaml:"imagePullPolicy"`
	Command         []string   `yaml:",flow"`
	Ports           []yPort    `yaml:"ports"`
	VolumeMounts    []yVolumes `yaml:"volumeMounts,omitempty"`
}

type yMetadata struct {
	Name      string            `yaml:"name"`
	Namespace string            `yaml:"namespace"`
	Labels    map[string]string `yaml:"labels,omitempty"`
}

type yPodBase struct {
	Containers    []yContainer `yaml:"containers"`
	RestartPolicy string       `yaml:"restartPolicy"`
	Volumes       []yVolumes   `yaml:"volumes,omitempty"`
	HostNetwork   bool         `yaml:"hostNetwork,omitempty"`
	NodeName      string       `yaml:"nodeName,omitempty"`
}

type YRes struct {
	ApiVersion string    `yaml:"apiVersion"`
	Kind       string    `yaml:"kind"`
	Metadata   yMetadata `yaml:"metadata"`
}

type yPod struct {
	YRes `yaml:",inline"`
	Spec yPodBase `yaml:"spec"`
}

type yServiceBase struct {
	Selector  map[string]string `yaml:"selector"`
	ClusterIP string            `yaml:"clusterIP"`
	Type      string            `yaml:"type"`
	Ports     []yPort           `yaml:"ports"`
}

type yService struct {
	YRes `yaml:",inline"`
	Spec yServiceBase `yaml:"spec"`
}

type DCompose struct {
	Name     string
	Services map[string]CService `yaml:"services"`
}

type CService struct {
	Image       string   `yaml:"image,omitempty"`
	Command     []string `yaml:"command,flow"`
	NetworkMode string   `yaml:"network_mode,omitempty"`
	Ports       []string `yaml:"ports,omitempty"`
	Volumes     []string `yaml:"volumes,omitempty"`
}

const (
	COM_TYPE = iota
	COM_GROUP
)

type K8HttpModule struct {
	HttpModule
}

func (m *K8HttpModule) Init(mgr *moduleMgr) {
	m.HttpModule.Init(mgr)
	m._httpbase = m
	m._host = ":" + util.GetConfValue("httpport")
}

func (m *K8HttpModule) initRoter(r *gin.Engine) {
	r.POST("/makeyaml", m.onMakeyaml)
	r.POST("/makeyamlSvFind", m.onMakeyamlServiceFind)
	r.GET("/yamlserver", m.onGetMakeyaml)

	r.GET("/composeSfind", m.onComposeServiceFind)
	r.GET("/composeServer", m.onComposeServer)

	r.GET("/servercmd", m.onServerCommond)
	r.GET("/yamlcmd", m.onYamlFileCommond)
	r.GET("/logpod", m.onLogPod)
	r.GET("/realip", m.onGetRealIp)
}

func (m *K8HttpModule) onMakeyaml(c *gin.Context) {
	d, err := c.GetRawData()
	if err != nil {
		c.String(http.StatusOK, err.Error())
		return
	}
	var syaml []dbServer
	err = json.Unmarshal(d, &syaml)
	if err != nil {
		c.String(http.StatusOK, err.Error())
		return
	}

	m.makeyaml(syaml, c)
}

func (m *K8HttpModule) onGetMakeyaml(c *gin.Context) {
	stype := c.Query("type")
	sid := c.Query("id")
	sgroup := c.Query("group")

	syaml := m.getallServer(&serverWhere{
		Type:  stype,
		Id:    sid,
		Group: sgroup,
	})

	if len(syaml) == 0 {
		c.String(http.StatusOK, "server len 0")
		return
	}
	m.makeyaml(syaml, c)
}

func (m *K8HttpModule) makeyaml(syaml []dbServer, c *gin.Context) {
	typechange := make([]int, 0)
	groupchange := make([]int, 0)

	for _, v := range syaml {
		if v.Type >= 40 {
			if !m.yamlgoServer(&v, c) {
				break
			}
		} else {
			if !m.yamlServer(&v, c) {
				break
			}
			typechange = append(typechange, v.Type)
			groupchange = append(groupchange, v.Group)
		}
	}

	allser := m.getallServer(nil)
	for _, v := range typechange {
		m.combinyamlServer(COM_TYPE, v, allser, c)
	}
	for _, v := range groupchange {
		m.combinyamlServer(COM_GROUP, v, allser, c)
	}
}

func (m *K8HttpModule) onMakeyamlServiceFind(c *gin.Context) {
	d, err := c.GetRawData()
	if err != nil {
		c.String(http.StatusOK, err.Error())
		return
	}
	var syaml yamlSfind
	err = json.Unmarshal(d, &syaml)
	if err != nil {
		c.String(http.StatusOK, err.Error())
		return
	}
	for _, v := range syaml.Server {
		if !m.yamlsfind(&v, c, syaml.Version) {
			break
		}
		if !m.yamlsfindService(&v, c) {
			break
		}
	}
}

func (m *K8HttpModule) onComposeServiceFind(c *gin.Context) {
	version := c.Query("version")
	var sfind []dbServiceFind

	lockFunc(&m._http_lock, func() {
		m._sql.Select(sfind, "SELECT * FROM `service_find`;")
	})
	m.composesfind(sfind, c, version)
}

func (m *K8HttpModule) onComposeServer(c *gin.Context) {
	stype := c.Query("type")
	sid := c.Query("id")
	sgroup := c.Query("group")

	syaml := m.getallServer(&serverWhere{
		Type:  stype,
		Id:    sid,
		Group: sgroup,
	})

	if len(syaml) == 0 {
		c.String(http.StatusOK, "server len 0")
		return
	}
	m.composeServer(syaml, c)
}

func (m *K8HttpModule) yamlsfind(sfind *dbServiceFind, c *gin.Context, version string) bool {
	basename := "service-find"
	name := fmt.Sprintf("service-find-%d", sfind.Id)

	lab := make(map[string]string, 0)
	lab["sername"] = basename
	lab["serid"] = name

	pod := &yPod{}
	pod.ApiVersion = "v1"
	pod.Kind = "Pod"
	pod.Metadata = yMetadata{
		Name:      name,
		Namespace: "loop",
		Labels:    lab,
	}

	imhost := util.GetConfValue("imagehost")

	yp1 := yPort{
		ContainerPort: sfind.Port,
	}

	yp2 := yPort{
		ContainerPort: sfind.Httpport,
	}

	cont := yContainer{
		Name:            basename,
		Image:           imhost + "goserver:" + version,
		ImagePullPolicy: "Always",
		Command:         []string{"./service_find", strconv.Itoa(sfind.Id)},
		Ports:           []yPort{yp1, yp2},
	}

	pod.Spec = yPodBase{
		Containers:    []yContainer{cont},
		RestartPolicy: "OnFailure",
	}

	ystr, err := yaml.Marshal(pod)
	if err != nil {
		c.String(http.StatusOK, err.Error())
		return false
	}
	outdir := util.GetConfValue("outdir")
	err = os.WriteFile(outdir+"/"+name+".yaml", ystr, 0644)
	if err != nil {
		c.String(http.StatusOK, err.Error())
		return false
	}
	c.String(http.StatusOK, string(ystr))
	return true
}

func (m *K8HttpModule) composesfind(sfinds []dbServiceFind, c *gin.Context, version string) bool {
	gen := DCompose{
		Services: make(map[string]CService),
	}

	for _, v := range sfinds {
		ser := CService{
			Image:       "goserver:" + version,
			Command:     []string{"./service_find", strconv.Itoa(v.Id)},
			NetworkMode: "host",
			Ports: []string{
				fmt.Sprintf("%d:%d", v.Port, v.Port),
				fmt.Sprintf("%d:%d", v.Httpport, v.Httpport),
			},
			Volumes: []string{
				"./conf.ini:/root/server/conf.ini",
				"./log:/root/server/log",
				"/tmp/coredump:/tmp/coredump",
			},
		}
		gen.Services[fmt.Sprintf("service_find_%d", v.Id)] = ser
	}

	ystr, err := yaml.Marshal(gen)
	if err != nil {
		c.String(http.StatusOK, err.Error())
		return false
	}
	outdir := util.GetConfValue("outdir")
	err = os.WriteFile(outdir+"/docker-compose.sfind.yml", ystr, 0644)
	if err != nil {
		c.String(http.StatusOK, err.Error())
		return false
	}
	c.String(http.StatusOK, string(ystr))
	return true
}

func (m *K8HttpModule) composeSaveFile(compose *DCompose, c *gin.Context) bool {
	ystr, err := yaml.Marshal(compose)
	if err != nil {
		c.String(http.StatusOK, err.Error())
		return false
	}
	outdir := util.GetConfValue("outdir")
	err = os.WriteFile(outdir+"/"+compose.Name+".yml", ystr, 0644)
	if err != nil {
		c.String(http.StatusOK, err.Error())
		return false
	}
	c.Writer.Write(ystr)
	return true
}

func (m *K8HttpModule) composeServer(sers []dbServer, c *gin.Context) bool {
	sergroup := make(map[int]*DCompose)
	globalser := make([]DCompose, 0)
	goserver := make([]DCompose, 0)

	for _, v := range sers {
		var iname string
		var cmd []string
		port := make([]string, 0)
		if v.Port > 0 {
			port = append(port, strconv.Itoa(v.Port))
		}

		vol := []string{
			"./log:/root/server/log",
			"/tmp/coredump:/tmp/coredump",
		}

		sid := strconv.Itoa(v.Id)

		if v.Type == util.ST_MASTER {
			iname = util.GetConfValue("imagegoser")
			cmd = []string{"./master", "1"}
			port = append(port, util.GetConfValue("masterport"))
			vol = append(vol, "./conf.ini:/root/server/conf.ini")
		} else if v.Type == util.ST_MONITOR {
			iname = util.GetConfValue("imagegoser")
			cmd = []string{"./monitor", sid}
			vol = append(vol, "./conf.ini:/root/server/conf.ini")
		} else if v.Type == util.ST_OPERATE {
			iname = util.GetConfValue("imagegoser")
			cmd = []string{"./operate", sid}
			port = append(port, util.GetConfValue("operateport"))
			vol = append(vol, "./conf.ini:/root/server/conf.ini")
		} else {
			iname = util.GetConfValue("imageserver")
			cmd = []string{"./Server", "-t", strconv.Itoa(v.Type), "-n", sid}
			vol = append(vol, "./Common.json:/root/server/commonconf/Common.json")
		}

		ser := CService{
			Image:   iname + ":" + v.Version,
			Command: cmd,
			Ports:   port,
			Volumes: vol,
		}

		if v.Type > 40 {
			dc := DCompose{
				Services: make(map[string]CService),
			}
			sname := v.Name + "_" + sid
			dc.Services[sname] = ser
			dc.Name = sname
			goserver = append(goserver, dc)
		} else {
			if v.Group == 0 {
				dc := DCompose{
					Services: make(map[string]CService),
				}
				dc.Services[fmt.Sprintf("%s_%d", v.Name, v.Id)] = ser
				dc.Name = fmt.Sprintf("server_%d_%d", v.Type, v.Id)
				globalser = append(globalser, dc)
			} else {
				dc, ok := sergroup[v.Group]
				if !ok {
					dc = &DCompose{
						Services: make(map[string]CService),
					}
					dc.Name = fmt.Sprintf("group_%d", v.Group)
					sergroup[v.Group] = dc
				}
				dc.Services[fmt.Sprintf("%s_%d", v.Name, v.Id)] = ser
			}
		}
	}

	for _, v := range sergroup {
		if !m.composeSaveFile(v, c) {
			return false
		}
	}

	for _, v := range globalser {
		if !m.composeSaveFile(&v, c) {
			return false
		}
	}

	for _, v := range goserver {
		if !m.composeSaveFile(&v, c) {
			return false
		}
	}
	return true
}

func (m *K8HttpModule) yamlsfindService(sfind *dbServiceFind, c *gin.Context) bool {
	name := fmt.Sprintf("service-find-%d", sfind.Id)

	pod := &yService{}
	pod.ApiVersion = "v1"
	pod.Kind = "Service"
	pod.Metadata = yMetadata{
		Name:      "sv-" + name,
		Namespace: "loop",
	}

	sel := map[string]string{
		"serid": name,
	}

	yp := yPort{
		Port:       sfind.Port,
		TargetPort: sfind.Port,
	}

	pod.Spec = yServiceBase{
		Selector:  sel,
		ClusterIP: sfind.Ip,
		Type:      "ClusterIP",
		Ports:     []yPort{yp},
	}

	ystr, err := yaml.Marshal(pod)
	if err != nil {
		c.String(http.StatusOK, err.Error())
		return false
	}
	outdir := util.GetConfValue("outdir")
	err = os.WriteFile(outdir+"/sv-"+name+".yaml", ystr, 0644)
	if err != nil {
		c.String(http.StatusOK, err.Error())
		return false
	}
	c.String(http.StatusOK, string(ystr))
	return true
}

func (m *K8HttpModule) yamlgoServer(sfind *dbServer, c *gin.Context) bool {
	basename := fmt.Sprintf("server-%d", sfind.Type)
	name := fmt.Sprintf("server-%d-%d", sfind.Type, sfind.Id)

	lab := make(map[string]string, 0)
	lab["sername"] = basename
	lab["serid"] = name

	pod := &yPod{}
	pod.ApiVersion = "v1"
	pod.Kind = "Pod"
	pod.Metadata = yMetadata{
		Name:      name,
		Namespace: "loop",
		Labels:    lab,
	}

	imhost := util.GetConfValue("imagehost")

	var exename string
	httport := 0
	if sfind.Type == util.ST_MONITOR {
		exename = "./monitor"
	} else if sfind.Type == util.ST_MASTER {
		exename = "./master"
		httport = util.GetConfInt("masterport")
	}

	vers := sfind.Version
	if vers == "" {
		vers = "latest"
	}

	cont := yContainer{
		Name:            basename,
		Image:           imhost + "goserver:" + vers,
		ImagePullPolicy: "IfNotPresent",
		Command:         []string{exename, strconv.Itoa(sfind.Id)},
	}

	if sfind.Port > 0 {
		cont.Ports = append(cont.Ports, yPort{ContainerPort: sfind.Port})
	}

	if httport > 0 {
		cont.Ports = append(cont.Ports, yPort{ContainerPort: httport})
	}

	pod.Spec = yPodBase{
		Containers:    []yContainer{cont},
		RestartPolicy: "OnFailure",
	}

	if sfind.Type == util.ST_MASTER {
		pod.Spec.HostNetwork = true
		pod.Spec.NodeName = util.GetConfValue("masternode")
	}

	ystr, err := yaml.Marshal(pod)
	if err != nil {
		c.String(http.StatusOK, err.Error())
		return false
	}
	outdir := util.GetConfValue("outdir")
	err = os.WriteFile(outdir+"/"+name+".yaml", ystr, 0644)
	if err != nil {
		c.String(http.StatusOK, err.Error())
		return false
	}
	c.String(http.StatusOK, string(ystr))
	return true
}

func (m *K8HttpModule) yamlServer(sfind *dbServer, c *gin.Context) bool {
	name := fmt.Sprintf("server-%d-%d", sfind.Type, sfind.Id)
	pod := m.makeServerPod(sfind)

	ystr, err := yaml.Marshal(pod)
	if err != nil {
		c.String(http.StatusOK, err.Error())
		return false
	}

	outdir := util.GetConfValue("outdir")
	err = os.WriteFile(outdir+"/"+name+".yaml", ystr, 0644)
	if err != nil {
		c.String(http.StatusOK, err.Error())
		return false
	}
	c.Writer.Write(ystr)
	c.Writer.WriteString("-----------------------------------------")
	return true
}

func (m *K8HttpModule) makeServerPod(sfind *dbServer) *yPod {
	basename := fmt.Sprintf("server-%d", sfind.Type)
	name := fmt.Sprintf("server-%d-%d", sfind.Type, sfind.Id)

	lab := make(map[string]string, 0)
	lab["sername"] = basename
	lab["serid"] = name
	lab["sergroup"] = fmt.Sprintf("group-%d", sfind.Group)

	pod := &yPod{}
	pod.ApiVersion = "v1"
	pod.Kind = "Pod"
	pod.Metadata = yMetadata{
		Name:      name,
		Namespace: "loop",
		Labels:    lab,
	}

	imhost := util.GetConfValue("imagehost")

	version := sfind.Version
	if version == "" {
		version = "latest"
	}

	cont := yContainer{
		Name:            basename,
		Image:           imhost + "loopserver:" + version,
		ImagePullPolicy: "IfNotPresent",
		Command:         []string{"./Server", "-t", strconv.Itoa(sfind.Type), "-n", strconv.Itoa(sfind.Id)},
	}

	if sfind.Port > 0 {
		yp1 := yPort{ContainerPort: sfind.Port}
		cont.Ports = []yPort{yp1}
	}

	cm := yVolumes{
		Name:      "log-dir",
		MountPath: "/root/server/logs",
	}
	cont.VolumeMounts = []yVolumes{cm}

	vm := yVolumes{
		Name: "log-dir",
		HostPath: map[string]string{
			"path": "/root/serverlog",
			"type": "DirectoryOrCreate",
		},
	}

	pod.Spec = yPodBase{
		Containers:    []yContainer{cont},
		RestartPolicy: "OnFailure",
		Volumes:       []yVolumes{vm},
	}

	if sfind.Type == util.ST_LOGIN || sfind.Type == util.ST_GATE {
		pod.Spec.HostNetwork = true
	}
	return pod
}

type serverWhere struct {
	Type  string `db:"type"`
	Id    string `db:"id"`
	Group string `db:"group"`
}

func (m *K8HttpModule) getallServer(sw *serverWhere) []dbServer {
	wcon := ""
	if sw != nil {
		t := reflect.ValueOf(*sw)
		for i := 0; i < t.NumField(); i++ {
			ft := t.Type().Field(i)
			fv := t.Field(i)
			if fv.String() != "" {
				if len(wcon) > 0 {
					wcon += fmt.Sprintf(" AND `%s`=%s", ft.Tag.Get("db"), fv.String())
				} else {
					wcon += fmt.Sprintf(" `%s`=%s", ft.Tag.Get("db"), fv.String())
				}
			}
		}
	}

	if len(wcon) > 0 {
		wcon = " WHERE " + wcon
	}

	var svec []dbServer
	func() {
		m._http_lock.Lock()
		defer m._http_lock.Unlock()
		err := m._sql.Select(&svec, fmt.Sprintf("SELECT * FROM `server`%s;", wcon))
		if err != nil {
			util.Log_error(err.Error())
			return
		}
	}()
	return svec
}

func (m *K8HttpModule) combinyamlServer(comtype int, comval int, svec []dbServer, c *gin.Context) {
	podvec := make([]yPod, 0)
	for _, v := range svec {
		if comtype == COM_TYPE {
			if v.Type == comval {
				podvec = append(podvec, *m.makeServerPod(&v))
			}
		} else if comtype == COM_GROUP {
			if v.Group == comval && v.Type < 40 {
				podvec = append(podvec, *m.makeServerPod(&v))
			}
		}
	}

	if len(podvec) == 0 {
		return
	}

	var fname string
	if comtype == COM_TYPE {
		fname = fmt.Sprintf("/server-type-%d.yaml", comval)
	} else if comtype == COM_GROUP {
		fname = fmt.Sprintf("/server-group-%d.yaml", comval)
	}

	outdir := util.GetConfValue("outdir")
	fh, err := os.OpenFile(outdir+fname, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		c.String(http.StatusOK, err.Error())
		return
	}

	for _, v := range podvec {
		ystr, err := yaml.Marshal(v)
		if err != nil {
			c.String(http.StatusOK, err.Error())
			return
		}
		_, err = fh.Write(ystr)
		if err != nil {
			c.String(http.StatusOK, err.Error())
			return
		}
		_, err = fh.WriteString("\n---\n\n")
		if err != nil {
			c.String(http.StatusOK, err.Error())
			return
		}
	}

	fh.Close()
}

func (m *K8HttpModule) onServerCommond(c *gin.Context) {
	stype := c.Query("type")
	sid := c.Query("id")
	sgroup := c.Query("group")
	cmdtype := c.Query("cmd")

	var err error
	defer func() {
		if err != nil {
			c.String(http.StatusOK, err.Error())
		} else {
			c.String(http.StatusOK, "cmd down")
		}
	}()

	if sid == "" {
		if stype == "" {
			// 执行group
			fname := fmt.Sprintf("server-group-%s.yaml", sgroup)
			m.yamlCommond(fname, cmdtype, c)
			return
		} else if sgroup == "" {
			// 执行type
			fname := fmt.Sprintf("server-type-%s.yaml", stype)
			m.yamlCommond(fname, cmdtype, c)
			return
		}
	}

	svec := m.getallServer(&serverWhere{
		Type:  stype,
		Id:    sid,
		Group: sgroup,
	})

	if len(svec) == 0 {
		c.String(http.StatusOK, "empty server list")
		return
	}

	for _, v := range svec {
		fname := fmt.Sprintf("server-%d-%d.yaml", v.Type, v.Id)
		m.yamlCommond(fname, cmdtype, c)
	}
}

func (m *K8HttpModule) yamlCommond(fname string, cmdtype string, c *gin.Context) {
	var opt string
	if cmdtype == "start" {
		opt = "apply"
	} else if cmdtype == "close" {
		opt = "delete"
	} else if cmdtype == "restart" {
		m.yamlCommond(fname, "close", c)
		m.yamlCommond(fname, "start", c)
		return
	} else {
		c.Writer.WriteString(fmt.Sprintf("opt error %s", cmdtype))
		return
	}

	outdir := util.GetConfValue("outdir")
	cmd := exec.Command("kubectl", opt, "-f", outdir+"/"+fname)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	c.Writer.WriteString(fmt.Sprintf("begin %s %s\n", opt, fname))

	err := cmd.Run()
	c.Writer.Write(stdout.Bytes())
	c.Writer.Write(stderr.Bytes())
	if err != nil {
		c.Writer.WriteString(err.Error())
	}

	c.Writer.WriteString(fmt.Sprintf("end %s %s\n", opt, fname))
}

func (m *K8HttpModule) onYamlFileCommond(c *gin.Context) {
	fname := c.Query("fname")
	cmdtype := c.Query("cmd")
	m.yamlCommond(fname, cmdtype, c)
}

func (m *K8HttpModule) onLogPod(c *gin.Context) {
	pod := c.Query("pod")
	ns := c.Query("namespace")
	tailnum := c.Query("tailnum")

	cmd := "kubectl"
	args := []string{"logs", "--tail", tailnum, pod, "-n", ns}
	m.execcmd(cmd, args, c)
}

func (m *K8HttpModule) execcmd(cmdstr string, args []string, c *gin.Context) {
	cmd := exec.Command(cmdstr, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	c.Writer.WriteString(fmt.Sprintf("begin %s \n", cmdstr))
	err := cmd.Run()
	c.Writer.Write(stdout.Bytes())
	c.Writer.Write(stderr.Bytes())
	if err != nil {
		c.Writer.WriteString(err.Error())
	}
}

func (m *K8HttpModule) onGetRealIp(c *gin.Context) {
	stype := c.Query("type")
	sid := c.Query("id")
	c.String(http.StatusOK, c.ClientIP())
	util.Log_info("type:%s id:%s realip %s\n", stype, sid, c.ClientIP())
}
