// Copyright 2016-2017
// CoderG the 2016 project
// Insight 0+0 [ 洞悉 0+0 ]
// InDimensions Construct Source [ 忆黛蒙逝·建造源 ]
// Stephen Fire Meditation Qin [ 火志溟 ] -> firemeditation@gmail.com
// Use of this source code is governed by GNU LGPL v3 license

package webs2

import (
	"fmt"
	"net/http"
	"os"

	"github.com/idcsource/Insight-0-0-lib/cpool"
	"github.com/idcsource/Insight-0-0-lib/drule"
	"github.com/idcsource/Insight-0-0-lib/idb"
	"github.com/idcsource/Insight-0-0-lib/ilogs"
	"github.com/idcsource/Insight-0-0-lib/pubfunc"
)

// 创建一个Web，db数据库和log日志可以为nil
func NewWeb(config *cpool.Section, db *idb.DB, log *ilogs.Logs) (web *Web) {
	if log == nil {
		log, _ = ilogs.NewLog("", "", "Web")
	}
	web = &Web{
		local:    pubfunc.LocalPath(""),
		config:   config,
		database: db,
		ext:      make(map[string]interface{}),
		log:      log,
		router:   newRouter(log),
	}
	// 检查静态资源地址是不是有
	static, err := web.config.GetConfig("static")
	if err != nil {
		static = web.local
	} else {
		static = pubfunc.LocalPath(static)
		static = pubfunc.DirMustEnd(static)
	}
	web.static = static
	return
}

// 获取本地路径
func (web *Web) GetLocalPath() (path string) {
	return web.local
}

// 获取静态文件路径
func (web *Web) GetStaticPath() (path string) {
	return web.static
}

// 注册扩展
func (web *Web) RegExt(name string, ext interface{}) {
	web.ext[name] = ext
}

// 获取扩展
func (web *Web) GetExt(name string) (ext interface{}, err error) {
	_, find := web.ext[name]
	if find == false {
		err = fmt.Errorf("webs2[Web]GetExt: The Extend %v not registered.", name)
		return
	}
	return web.ext[name], nil
}

// 注册DRule
func (web *Web) RegDRule(d *drule.Operator) {
	web.drule = d
}

// 注册TRule
func (web *Web) RegTRule(t *drule.TRule) {
	web.trule = t
}

// 获得DRule，如果没有注册则返回错误
func (web *Web) GetDRule() (d *drule.Operator, err error) {
	if web.drule == nil {
		err = fmt.Errorf("webs2[Web]GetDRule: The DRule Operator not registered.")
		return
	}
	return web.drule, nil
}

// 获得TRule，如果没有注册则返回错误
func (web *Web) GetTRule() (d *drule.TRule, err error) {
	if web.trule == nil {
		err = fmt.Errorf("webs2[Web]GetDRule: The TRule Operator not registered.")
		return
	}
	return web.trule, nil
}

// 创建路由，设置根节点，并返回根结点，之后所有的对节点的添加操作均是*NodeTree提供的方法
func (web *Web) InitRouter(f FloorInterface, config *cpool.Block) (root *NodeTree) {
	return web.router.buildRouter(f, config)
}

// 创建静态地址,path必须是相对于静态地址(static)的地址
func (web *Web) AddStatic(url, path string) {
	path = web.static + path
	path = pubfunc.DirMustEnd(path)
	web.router.addStatic(url, path)
}

// 修改默认的404处理
func (web *Web) SetNotFound(f FloorInterface) {
	web.router.not_found = f
}

func (web *Web) Start() (err error) {
	// 如果没有初始化路由
	if web.router.router_ok == false {
		err = fmt.Errorf("webs2[Web]Start: The Router not initialization.")
		web.log.ErrLog(err)
		return
	}

	/* 检查一堆配置文件是否有 */

	// 检查端口是否有
	port, err := web.config.GetConfig("port")
	if err != nil {
		err = fmt.Errorf("webs2[Web]Start: The config port not be set.")
		web.log.ErrLog(err)
		return
	}
	// 检查是http还是https
	var ifHttps bool
	ifHttps, err = web.config.TranBool("https")
	if err != nil {
		ifHttps = false
		err = nil
	}
	var thecert, thekey string
	if ifHttps == true {
		var e2, e3 error
		thecert, e2 = web.config.GetConfig("sslcert")
		thekey, e3 = web.config.GetConfig("sslkey")
		if e2 != nil || e3 != nil {
			err = fmt.Errorf("webs2[Web]Start: The SSL cert or key not be set !")
			web.log.ErrLog(err)
			return
		}
	}

	/* 启动HTTP服务 */
	port = ":" + port

	go func() {
		if ifHttps == true {
			err = http.ListenAndServeTLS(port, pubfunc.LocalFile(thecert), pubfunc.LocalFile(thekey), web)
		} else {
			err = http.ListenAndServe(port, web)
		}
		if err != nil {
			err = fmt.Errorf("webs2[Web]Start: Can not start the web server: %v", err)
			web.log.ErrLog(err)
			return
		}
	}()
	return
}

//HTTP的路由，提供给"net/http"包使用
func (web *Web) ServeHTTP(httpw http.ResponseWriter, httpr *http.Request) {
	//要运行的Floor
	var runfloor FloorInterface
	//将获得的URL用斜线拆分成[]string
	urla, parameter := pubfunc.SplitUrl(httpr.URL.Path)
	//准备基本的RunTime
	rt := Runtime{AllRoutePath: httpr.URL.Path, NowRoutePath: urla, UrlRequest: parameter}

	//静态路由
	static, have := web.router.getStatic(httpr.URL.Path)
	if have == true {
		_, err := os.Stat(static)
		if err != nil {
			web.toNotFoundHttp(httpw, httpr, rt)
		} else {
			http.ServeFile(httpw, httpr, static)
		}
		return
	}

	// 如果为0,则处理首页，直接取出NodeTree的根节点
	if len(urla) == 0 {
		rt.RealNode = ""
		runfloor = web.router.node_tree.floor
	} else {
		runfloor, rt = web.router.getRunFloor(rt)
	}

	//开始执行
	runfloor.InitHTTP(httpw, httpr, web, rt)
	runfloor.ExecHTTP()
	return
}

// 去执行NotFound，不要直接调用这个方法
func (web *Web) toNotFoundHttp(w http.ResponseWriter, r *http.Request, rt Runtime) {
	runfloor := web.router.not_found
	runfloor.InitHTTP(w, r, web, rt)
	runfloor.ExecHTTP()
	return
}
