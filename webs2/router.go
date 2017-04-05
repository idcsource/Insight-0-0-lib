// Copyright 2016-2017
// CoderG the 2016 project
// Insight 0+0 [ 洞悉 0+0 ]
// InDimensions Construct Source [ 忆黛蒙逝·建造源 ]
// Stephen Fire Meditation Qin [ 火志溟 ] -> firemeditation@gmail.com
// Use of this source code is governed by GNU LGPL v3 license

package webs2

import (
	"reflect"
	"regexp"

	"github.com/idcsource/Insight-0-0-lib/cpool"
	"github.com/idcsource/Insight-0-0-lib/ilogs"
	"github.com/idcsource/Insight-0-0-lib/pubfunc"
)

// 创建路由器，在NewWeb()中调用，不需要手动调用
func newRouter(log *ilogs.Logs) (router *Router) {
	router = &Router{
		router_ok:    false,
		static_route: make(map[string]*regexp.Regexp),
		not_found:    reflect.ValueOf(&NotFoundFloor{}),
	}
	return
}

// 创建路由，行为是增加根节点
func (router *Router) buildRouter(f FloorInterface, config *cpool.Block) (root *NodeTree) {
	root = AddRootNode(f, config)
	router.node_tree = root
	router.router_ok = true
	return
}

// 添加静态路由，path为相对于static设定的地址
func (router *Router) addStatic(url, path string) {
	url = pubfunc.PathMustBegin(url)
	url = "^" + url + "/(.*)"
	tu, err := regexp.Compile(url)
	if err != nil {
		return
	}
	router.static_route[path] = tu
	return
}

// 获取静态路由对应的地址
func (r *Router) getStatic(mark string) (path string, have bool) {
	have = false
	if len(r.static_route) != 0 {
		for k, v := range r.static_route {
			if v.MatchString(mark) {
				nameA := v.FindStringSubmatch(mark)
				if len(nameA) > 1 {
					name := nameA[1]
					path = pubfunc.LocalFile("") + pubfunc.DirMustEnd(k) + name
					have = true
				}
				return
			}
		}
	}
	return
}

// 找到现在需要去运行的Floor
func (r *Router) getRunFloor(rt Runtime) (runfloor reflect.Value, rtn Runtime) {
	var np *NodeTree
	var nothing bool
	runfloor = r.not_found
	np, rtn, nothing = r.node_tree.getRunFloor(rt)
	// 如果最终返回的是一个Door
	if np.node_type == NODE_IS_DOOR {
		floorlistv := np.floor.MethodByName("FloorList").Call(nil)
		floorlist := floorlistv[0].Interface().(FloorDoor)
		var fname string
		if len(rtn.NowRoutePath) > 0 {
			fname = rtn.NowRoutePath[0]
		} else {
			fname = "/"
		}
		dfloor, fok := floorlist[fname]
		if fok == false {
			runfloor = r.not_found
		} else {
			runfloor = reflect.ValueOf(dfloor)
			if len(rtn.NowRoutePath) > 0 {
				rtn.NowRoutePath = rtn.NowRoutePath[1:]
			}
		}
	} else {
		if nothing == true {
			runfloor = r.not_found
		} else {
			runfloor = np.floor
		}
	}
	return
}