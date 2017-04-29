// Copyright 2016-2017
// CoderG the 2016 project
// Insight 0+0 [ 洞悉 0+0 ]
// InDimensions Construct Source [ 忆黛蒙逝·建造源 ]
// Stephen Fire Meditation Qin [ 火志溟 ] -> firemeditation@gmail.com
// Use of this source code is governed by GNU LGPL v3 license

package drule

import (
	"time"

	"github.com/idcsource/Insight-0-0-lib/drule2/operator"
	"github.com/idcsource/Insight-0-0-lib/iendecode"
	"github.com/idcsource/Insight-0-0-lib/nst"
	"github.com/idcsource/Insight-0-0-lib/random"
)

// 启动
func (d *DRule) sys_druleStart(conn_exec *nst.ConnExec, o_send *operator.O_OperatorSend) (errs error) {
	var err error
	// 查看用户权限
	auth, login := d.getUserAuthority(o_send.User, o_send.Unid)
	if login == false {
		d.sendReceipt(conn_exec, operator.DATA_USER_NOT_LOGIN, "", nil)
		return
	}
	if auth != operator.USER_AUTHORITY_ROOT {
		d.sendReceipt(conn_exec, operator.DATA_USER_NO_AUTHORITY, "", nil)
		return
	}
	// 执行
	err = d.Start()
	if err != nil {
		errs = d.sendReceipt(conn_exec, operator.DATA_RETURN_ERROR, err.Error(), nil)
		return
	} else {
		errs = d.sendReceipt(conn_exec, operator.DATA_ALL_OK, "", nil)
		return
	}
	return
}

// 暂停
func (d *DRule) sys_drulePause(conn_exec *nst.ConnExec, o_send *operator.O_OperatorSend) (errs error) {
	// var err error
	// 查看用户权限
	auth, login := d.getUserAuthority(o_send.User, o_send.Unid)
	if login == false {
		errs = d.sendReceipt(conn_exec, operator.DATA_USER_NOT_LOGIN, "", nil)
		return
	}
	if auth != operator.USER_AUTHORITY_ROOT {
		errs = d.sendReceipt(conn_exec, operator.DATA_USER_NO_AUTHORITY, "", nil)
		return
	}
	// 执行
	d.Pause()
	errs = d.sendReceipt(conn_exec, operator.DATA_ALL_OK, "", nil)
	return
}

// 运行模式
func (d *DRule) sys_druleMode(conn_exec *nst.ConnExec, o_send *operator.O_OperatorSend) (errs error) {
	var err error
	// 查看用户权限
	auth, login := d.getUserAuthority(o_send.User, o_send.Unid)
	if login == false {
		errs = d.sendReceipt(conn_exec, operator.DATA_USER_NOT_LOGIN, "", nil)
		return
	}
	if auth != operator.USER_AUTHORITY_ROOT {
		errs = d.sendReceipt(conn_exec, operator.DATA_USER_NO_AUTHORITY, "", nil)
		return
	}
	// 执行
	mode := d.dmode
	mode_b, err := iendecode.StructGobBytes(mode)
	if err != nil {
		errs = d.sendReceipt(conn_exec, operator.DATA_RETURN_ERROR, err.Error(), nil)
		return
	}
	errs = d.sendReceipt(conn_exec, operator.DATA_ALL_OK, "", mode_b)
	return
}

// 用户登录
func (d *DRule) sys_userLogin(conn_exec *nst.ConnExec, o_send *operator.O_OperatorSend) (errs error) {
	var err error
	// 解码
	var login operator.O_DRuleUser
	err = iendecode.BytesGobStruct(o_send.Data, &login)
	if err != nil {
		errs = d.sendReceipt(conn_exec, operator.DATA_RETURN_ERROR, err.Error(), nil)
		return
	}

	user_id := USER_PREFIX + login.UserName
	// 查看有没有这个用户
	user_have := d.trule.ExistRole(INSIDE_DMZ, user_id)
	if user_have == false {
		errs = d.sendReceipt(conn_exec, operator.DATA_USER_NO_EXIST, "", nil)
		return
	}
	// 查看密码
	var password string
	err = d.trule.ReadData(INSIDE_DMZ, user_id, "Password", &password)
	if err != nil {
		errs = d.sendReceipt(conn_exec, operator.DATA_RETURN_ERROR, err.Error(), nil)
		return
	}
	if password != login.Password {
		errs = d.sendReceipt(conn_exec, operator.DATA_USER_NO_EXIST, "", nil)
		return
	}
	// 规整权限
	var auth operator.UserAuthority
	d.trule.ReadData(INSIDE_DMZ, user_id, "Authority", &auth)
	wrable := make(map[string]bool)
	d.trule.ReadData(INSIDE_DMZ, user_id, "WRable", &wrable)

	unid := random.Unid(1, time.Now().String(), login.UserName)

	// 查看是否已经有登录的了，并写入登录的乱七八糟
	_, find := d.loginuser[login.UserName]
	if find {
		d.loginuser[login.UserName].wrable = wrable
		d.loginuser[login.UserName].unid[unid] = time.Now()
	} else {
		loginuser := &loginUser{
			username:  login.UserName,
			unid:      make(map[string]time.Time),
			authority: auth,
			wrable:    wrable,
		}
		loginuser.unid[unid] = time.Now()
		d.loginuser[login.UserName] = loginuser
	}
	errs = d.sendReceipt(conn_exec, operator.DATA_ALL_OK, "", nil)
	return
}

// 用户续命
func (d *DRule) sys_userAddLife(conn_exec *nst.ConnExec, o_send *operator.O_OperatorSend) (errs error) {
	yes := d.checkUserLogin(o_send.User, o_send.Unid)
	if yes == true {
		errs = d.sendReceipt(conn_exec, operator.DATA_ALL_OK, "", nil)
		return
	} else {
		errs = d.sendReceipt(conn_exec, operator.DATA_USER_NOT_LOGIN, "", nil)
		return
	}
}

// 新建用户
func (d *DRule) sys_userAdd(conn_exec *nst.ConnExec, o_send *operator.O_OperatorSend) (errs error) {
	var err error
	// 查看用户权限
	auth, login := d.getUserAuthority(o_send.User, o_send.Unid)
	if login == false {
		errs = d.sendReceipt(conn_exec, operator.DATA_USER_NOT_LOGIN, "", nil)
		return
	}
	if auth != operator.USER_AUTHORITY_ROOT {
		errs = d.sendReceipt(conn_exec, operator.DATA_USER_NO_AUTHORITY, "", nil)
		return
	}
	// 解码
	var newuser operator.O_DRuleUser
	err = iendecode.BytesGobStruct(o_send.Data, &newuser)
	if err != nil {
		errs = d.sendReceipt(conn_exec, operator.DATA_RETURN_ERROR, err.Error(), nil)
		return
	}

	return
}
