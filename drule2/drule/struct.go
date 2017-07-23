// Copyright 2016-2017
// CoderG the 2016 project
// Insight 0+0 [ 洞悉 0+0 ]
// InDimensions Construct Source [ 忆黛蒙逝·建造源 ]
// Stephen Fire Meditation Qin [ 火志溟 ] -> firemeditation@gmail.com
// Use of this source code is governed by GNU LGPL v3 license

// drule2的“分布式统治者”
package drule

import (
	"sync"
	"time"

	"github.com/idcsource/insight00-lib/drule2/operator"
	"github.com/idcsource/insight00-lib/drule2/trule"
	"github.com/idcsource/insight00-lib/ilogs"
	"github.com/idcsource/insight00-lib/roles"
)

// 这是DRule——分布式统治者
type DRule struct {
	// 事务统治者
	trule *trule.TRule
	// 自己的名字
	selfname string

	// 已经关闭
	closed bool

	// 分布式服务模式，OPERATE_MODE_*
	dmode operator.DRuleOperateMode

	operators map[string]*operator.Operator // 这是operator的连接，连接到其他DRule上的

	areas map[string]*AreasRouter // 需要蔓延到其他drule上的区域列表

	loginuser      map[string]*loginUser // 登录进来的用户,string为用户名
	loginuser_lock *sync.RWMutex         // loginuser的锁

	// 事务映射
	transaction_map      map[string]*transactionMap // string为外来的id
	transaction_map_lock *sync.RWMutex              // transaction_map的锁

	// 日志
	logs *ilogs.Logs
}

// 事务映射
type transactionMap struct {
	tran_unid string                            // 对应外来的事务unid
	tran      *trule.Transaction                // 对应的本地trule的事务
	operators map[string]*operator.OTransaction // 对应的外部的
	alivetime time.Time                         // 活动时间
}

// 登录进来的用户
type loginUser struct {
	username   string
	unid       map[string]time.Time   // string为unid，time则为活动时间
	authority  operator.UserAuthority // 用户权限
	wrable     map[string]bool        // 与DRuleUser一致
	activetime time.Time              // 活动时间
	lock       *sync.RWMutex          // 锁
}

type AreasRouterRoot struct {
	roles.Role
}

// 蔓延到其他drule上的区域
type AreasRouter struct {
	roles.Role                     // 角色
	AreaName   string              // 区域名称
	Mirror     bool                // 是否为镜像，ture为镜像，则所有的文件都发给下面所有的drule
	Mirrors    []string            // string为drule的名字
	Chars      map[string][]string // 如果mirror为false，则看这个根据不同的字母进行路由，第一个stirng为首字母，第二个string为operator的名称
}

// 远程操作者的记录根
type DRuleOperatorRoot struct {
	roles.Role
}

// 远端操作者的记录，为OperatorRoot的子角色
type DRuleOperator struct {
	roles.Role
	Name     string // 名称
	Address  string // 地址与端口
	ConnNum  int    // 连接数
	TLS      bool   // 是否加密
	Username string // 用户名
	Password string // 密码
}

// Drule和Operator的用户，root用户为根，其他用户为子角色
type DRuleUser struct {
	roles.Role                        // 角色
	UserName   string                 // 用户名
	Password   string                 // 密码
	Email      string                 // 邮箱
	Authority  operator.UserAuthority // 权限，USER_AUTHORITY_*
	WRable     map[string]bool        // 读写权限，string为区域的名称，bool为true则是写，为false则为读
}
