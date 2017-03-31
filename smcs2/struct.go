// Copyright 2016-2017
// CoderG the 2016 project
// Insight 0+0 [ 洞悉 0+0 ]
// InDimensions Construct Source [ 忆黛蒙逝·建造源 ]
// Stephen Fire Meditation Qin [ 火志溟 ] -> firemeditation@gmail.com
// Use of this source code is governed by GNU LGPL v3 license

// Status Monitor & Configure Spread v2 [ 状态监控与配置蔓延 ]。
//
// 中心与节点之间的状态与配置的相互通讯。这里是第二个版本，因为第一版存在剥离问题，故还未彻底放弃。
//
// 第二版将才用角色进行节点配置信息的保存并支持更好的在线配置修改。节点配置信息的保存使用drule包的TRule。
package smcs2

import (
	"encoding/gob"
	"reflect"

	"github.com/idcsource/Insight-0-0-lib/bridges"
	"github.com/idcsource/Insight-0-0-lib/cpool"
	"github.com/idcsource/Insight-0-0-lib/drule"
	"github.com/idcsource/Insight-0-0-lib/ilogs"
	"github.com/idcsource/Insight-0-0-lib/nst"
	"github.com/idcsource/Insight-0-0-lib/roles"
)

const (
	ROLE_PREFIX = "SMCS_"     // 保存角色的前缀
	ROLE_ROOT   = "SMCS_ROOT" // 配置目录的根
	SLEEP_TIME  = 60          // 每隔多长间隔发送一次，单位为秒
)

const (
	NODE_STATUS_NO_CONFIG  = iota // 节点状态，没有配置文件
	NODE_STATUS_OK                // 一切OK
	NODE_STATUS_BUSY              // 忙碌
	NODE_STATUS_IDLE              // 闲置
	NODE_STATUS_STORE_FULL        // 存储满
)

const (
	NODE_OPERATE_NO       = iota // 节点收到信息后的处理，无
	NODE_OPERATE_BRIDGE          // 使用桥
	NODE_OPERATE_FUNCTION        // 使用函数
	NODE_OPERATE_BOTH            // 上面二者都
)

const (
	CONFIG_NO        = iota // 配置信息空状态
	CONFIG_NOT_READY        // 配置没有准备好（在这种状态下，不发送配置文件，配合WORK_SET_GOON）
	CONFIG_ALL_READY        // 配置准备妥当（将会在下次同步时发送）
)

const (
	WORK_SET_NO    = iota // 没有这个节点
	WORK_SET_GOON         // 节点的工作设置，继续之前
	WORK_SET_START        // 开始工作
	WORK_SET_STOP         // 停止工作
)

const (
	ROLE_TYPE_GROUP = iota // 角色是一个分组
	ROLE_TYPE_NODE  = iota // 角色是一个节点
)

// 节点树的数据类型
type NodeTree struct {
	Name     string              // 显示名
	Id       string              // 角色id
	RoleType uint8               // 角色类型，是分组还是具体的，ROLE_TYPE_*
	Tree     map[string]NodeTree // 节点树
}

// 节点的配置信息
type NodeConfig struct {
	roles.Role                    // 角色
	Name         string           // 真实显示名称
	ConfigStatus uint8            // 配置的状态，配合CONFIG_*
	NextWorkSet  uint8            // 下一个工作状态设置，WORK_SET_*
	RoleType     uint8            // 角色类型，是分组还是具体的，ROLE_TYPE_*
	Config       cpool.PoolEncode // 配置信息
	NewConfig    bool             // 是否有新配置文件
	RunLog       []string         // 运行日志
	ErrLog       []string         // 错误日志
}

// 节点发送给中心的数据结构
type NodeSend struct {
	CenterName string   // 中央的名字，用来做身份验证
	Name       string   // 节点的名称
	Status     uint8    // 节点状态，NODE_STATUS_*
	WorkSet    uint8    // 当前工作状态，WORK_SET_*
	RunLog     []string // 要发送出去的运行日志
	ErrLog     []string // 要发送出去的错误日志
}

// 中心发送给节点的数据结构
type CenterSend struct {
	NextWorkSet  uint8            // 下一个工作状态设置，WORK_SET_*
	ConfigStatus uint8            // 配置的状态，配合CONFIG_*
	SetStartTime int64            // 下一个工作状态的开始时间
	NewConfig    bool             // 是否有新配置文件
	Config       cpool.PoolEncode // 配置文件
	Error        string           // 错误
}

// 一个节点对应的发送与接收信息
type sendAndReceive struct {
	centerSend CenterSend // 中心发送的信息
	nodeSend   NodeSend   // 节点发送的信息
}

// 中央的蔓延节点数据类型，也就是中央的服务器
type CenterSmcs struct {
	name    string                     // 自己的名字，用来做身份验证
	node    map[string]*sendAndReceive // 中心将要发送走的信息，string为节点的名称
	store   *drule.TRule               // 存储配置信息的方法，使用drule的TRule进行存储管理
	root_id string                     // 中央节点的ID
	root    roles.Roleer               // 中央节点，这是一个roles.Role类型
}

// 节点的蔓延数据类型，也就是节点的服务器
type NodeSmcs struct {
	name       string                 // 节点的名字
	centername string                 // 中央的名称
	tcpc       *nst.TcpClient         // TCP连接
	runtimeid  string                 // 运行时UNID
	operate    uint8                  // 对用NODE_OPERATE_*，来选择是outbridge还是outoperate处理
	outbridge  *bridges.BridgeOperate // 输出通讯桥，得到配置将发送到通讯桥
	outoperate reflect.Value          // 得到配置后将交由符合这个接口的方法
	nodesend   NodeSend               // 发送出去的类型
	sleeptime  int64                  // 每次请求中心的等待时间
	closeM     chan bool              // 关闭监控信号
	closeMt    bool                   // 是否处于关闭状态
	logn       *ilogs.Logs            // 需要发送给Center的日志
	logs       *ilogs.Logs            // 自己的日志
}

// 节点的配置后处理接口
type NodeOperator interface {
	SmcsNodeOperator(centerSend CenterSend) (err error)
}

// 为Gob注册角色类型
func RegInterfaceForGob() {
	gob.Register(&roles.Role{})
	gob.Register(&NodeConfig{})
}
