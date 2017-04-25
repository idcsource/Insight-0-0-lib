// Copyright 2016-2017
// CoderG the 2016 project
// Insight 0+0 [ 洞悉 0+0 ]
// InDimensions Construct Source [ 忆黛蒙逝·建造源 ]
// Stephen Fire Meditation Qin [ 火志溟 ] -> firemeditation@gmail.com
// Use of this source code is governed by GNU LGPL v3 license

package operator

type UserAuthority uint8

const (
	USER_AUTHORITY_NO     UserAuthority = iota // 没有权限
	USER_AUTHORITY_ROOT                        // 根权限
	USER_AUTHORITY_DRULE                       // DRule设备
	USER_AUTHORITY_NORMAL                      // 普通权限
)

// 几个时间
const (
	USER_ADD_LIFE   = 3000 // 续命间隔时间(单位秒)，不要大于USER_ALIVE_TIME
	USER_ALIVE_TIME = 3600 // 用户的登录生存期（单位秒）
)

type DRuleReturnStatus uint8

// 数据标记状态
const (
	// 数据没有任何的状态
	DATA_NOTHING DRuleReturnStatus = iota
	// 数据并不是期望的
	DATA_NOT_EXPECT
	// 数据一切正常
	DATA_ALL_OK
	// 数据终止
	DATA_END
	// 请发送数据
	DATA_PLEASE
	// 数据将发送
	DATA_WILL_SEND
	// 数据返回有错误
	DATA_RETURN_ERROR
	// 数据返回为True
	DATA_RETURN_IS_TRUE
	// 数据返回为flase
	DATA_RETURN_IS_FALSE
	// 事务找不到
	DATA_TRAN_NOT_EXIST
	// 已经关闭
	DATA_DRULE_CLOSED
	// 用户没有登录
	DATA_USER_NOT_LOGIN
	// 用户重复
	DATA_USER_EXIST
	// 用户不存在
	DATA_USER_NO_EXIST
	// 用户没有权限
	DATA_USER_NO_AUTHORITY
)

// OperatorType是Operator向Drule请求的操作类型
type OperatorType uint

// 这是OperatorType所可以使用的值
const (
	// 什么操作都没有
	OPERATE_NOTHING OperatorType = iota
	// 强制保存
	OPERATE_TOSTORE

	// 获取一个角色
	OPERATE_READ_ROLE
	// 写入一个角色
	OPERATE_WRITE_ROLE
	// 存在一个角色
	OPERATE_EXIST_ROLE

	// 创建一个新角色
	OPERATE_NEW_ROLE
	// 删除一个角色
	OPERATE_DEL_ROLE

	// 获取角色的一个值
	OPERATE_GET_DATA
	// 设置角色的一个值
	OPERATE_SET_DATA

	// 设置father
	OPERATE_SET_FATHER
	// 获取father
	OPERATE_GET_FATHER
	// 重置father
	OPERATE_RESET_FATHER

	// 设置children
	OPERATE_SET_CHILDREN
	// 获取children
	OPERATE_GET_CHILDREN
	// 重置children
	OPERATE_RESET_CHILDREN

	// 添加一个child
	OPERATE_ADD_CHILD
	// 删除一个child
	OPERATE_DEL_CHILD
	// 存在某个child
	OPERATE_EXIST_CHILD

	// 设置friends
	OPERATE_SET_FRIENDS
	// 获取friends
	OPERATE_GET_FRIENDS
	// 重置friends
	OPERATE_RESET_FRIENDS

	// 添加一个friend
	OPERATE_ADD_FRIEND
	// 删除一个friend
	OPERATE_DEL_FRIEND
	// 修改一个friend
	OPERATE_CHANGE_FRIEND
	// 获取同样绑定值的friend
	OPERATE_SAME_BIND_FRIEND

	// 添加一个空的上下文组
	OPERATE_ADD_CONTEXT
	// 删除一个上下文组
	OPERATE_DROP_CONTEXT
	// 获取所有上下文的名称
	OPERATE_GET_CONTEXTS_NAME
	// 读取一个上下文的全部
	OPERATE_READ_CONTEXT
	// 获取一个上下文中同样绑定值的角色id
	OPERATE_SAME_BIND_CONTEXT

	// 添加一个上下文绑定
	OPERATE_ADD_CONTEXT_BIND
	// 删除一个上下文绑定
	OPERATE_DEL_CONTEXT_BIND
	// 修改一个上下文的绑定
	OPERATE_CHANGE_CONTEXT_BIND
	// 返回上下文中同样绑定的元素
	OPERATE_CONTEXT_SAME_BIND

	// 添加一个上文
	OPERATE_ADD_CONTEXT_UP
	// 删除上文
	OPERATE_DEL_CONTEXT_UP
	// 修改上文
	OPERATE_CHANGE_CONTEXT_UP
	// 返回同样绑定的上文
	OPERATE_SAME_BIND_CONTEXT_UP

	// 添加一个下文
	OPERATE_ADD_CONTEXT_DOWN
	// 删除下文
	OPERATE_DEL_CONTEXT_DOWN
	// 修改下文
	OPERATE_CHANGE_CONTEXT_DOWN
	// 返回同样绑定的下文
	OPERATE_SAME_BIND_CONTEXT_DOWN

	// 设置朋友的状态
	OPERATE_SET_FRIEND_STATUS
	// 获取朋友的状态
	OPERATE_GET_FRIEND_STATUS

	// 设置上下文的状态
	OPERATE_SET_CONTEXT_STATUS
	// 获取上下文的状态
	OPERATE_GET_CONTEXT_STATUS

	// 设置contexts
	OPERATE_SET_CONTEXTS
	// 获取contexts
	OPERATE_GET_CONTEXTS
	// 重置contexts
	OPERATE_RESET_CONTEXTS

	// 启动事务
	OPERATE_TRAN_BEGIN
	// 回滚事务
	OPERATE_TRAN_ROLLBACK
	// 执行事务
	OPERATE_TRAN_COMMIT
	// 事务错误
	OPERATE_TRAN_ERROR
	// 准备事务
	OPERATE_TRAN_PREPARE

	// 用户登录
	OPERATE_USER_LOGIN
	// 用户续命
	OPERATE_USER_ADD_LIFE
	// 用户新建
	OPERATE_USER_ADD
	// 用户改密码
	OPERATE_USER_PASSWORD
	// 用户删除
	OPERATE_USER_DEL
	// 用户登出
	OPERATE_USER_LOGOUT
	// 用户列表
	OPERATE_USER_LIST

	// 添加区域
	OPERATE_AREA_ADD
	// 删除区域
	OPERATE_AREA_DEL
	// 修改区域名称
	OPERATE_AREA_RENAME
	// 区域是否存在
	OPERATE_AREA_EXIST
	// 区域列表
	OPERATE_AREA_LIST

	// 用户和区域的关系
	OPERATE_USER_AREA
)
