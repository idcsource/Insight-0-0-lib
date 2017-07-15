// Copyright 2016-2017
// CoderG the 2016 project
// Insight 0+0 [ 洞悉 0+0 ]
// InDimensions Construct Source [ 忆黛蒙逝·建造源 ]
// Stephen Fire Meditation Qin [ 火志溟 ] -> firemeditation@gmail.com
// Use of this source code is governed by GNU LGPL v3 license

package trule

import (
	"fmt"
	"strings"
	"time"

	"github.com/idcsource/Insight-0-0-lib/iendecode"
	"github.com/idcsource/Insight-0-0-lib/roles"
)

// 返回事务的ID
func (t *Transaction) TransactionID() (id string) {
	return t.unid
}

// 是否存在这个角色
func (t *Transaction) ExistRole(area, id string) (have bool, err error) {
	if t.be_delete == true {
		return false, fmt.Errorf("drule[Transaction]ReadRole: This transaction has been deleted.")
	}
	t.tran_time = time.Now()
	_, exist, err := t.getrole(area, id, TRAN_LOCK_MODE_READ)
	return exist, err
}

// 读取一个角色
//
// 角色会缓存并配置成写锁被本事务占用，如果在事务周期中不执行StoreRole保存，那么对这个角色的修改也不会被保存，信息将丢失。
func (t *Transaction) ReadRole(area, id string, role roles.Roleer) (err error) {
	if t.be_delete == true {
		return fmt.Errorf("drule[Transaction]ReadRole: This transaction has been deleted.")
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_WRITE)
	if err != nil {
		err = fmt.Errorf("drule[Transacion]ReadRole: %v", err)
		return
	}
	err = roles.DecodeMiddleToRole(*rolec.role, role)
	if err != nil {
		err = fmt.Errorf("drule[Transacion]ReadRole: %v", err)
	}
	return
}

// 从永久存储读出角色的MiddleData格式
func (t *Transaction) ReadRoleMiddleData(area, id string) (mid roles.RoleMiddleData, err error) {
	if t.be_delete == true {
		err = fmt.Errorf("drule[Transaction]readRoleMiddle: This transaction has been deleted.")
		return
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_WRITE)
	if err != nil {
		err = fmt.Errorf("drule[Transacion]readRoleByte: %v", err)
		return
	}
	mid = *rolec.role
	return
}

func (t *Transaction) ReadRoleByte(area, id string) (b []byte, err error) {
	if t.be_delete == true {
		return nil, fmt.Errorf("drule[Transaction]readRoleByte: This transaction has been deleted.")
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_WRITE)
	if err != nil {
		err = fmt.Errorf("drule[Transacion]readRoleByte: %v", err)
		return
	}
	b, err = iendecode.StructGobBytes(rolec.role)
	return
}

// 写入一个角色
//
// 依然会去缓存中尝试获取角色的写权限，如果找不到,则去写一个新的
func (t *Transaction) StoreRole(area string, role roles.Roleer) (err error) {
	if t.be_delete == true {
		return fmt.Errorf("drule[Transaction]StoreRole: This transaction has been deleted.")
	}
	t.tran_time = time.Now()
	t.lock.RLock()
	defer t.lock.RUnlock()
	roleid := role.ReturnId()
	var find bool
	rolec, find := t.tran_cache[roleid]
	if find == true {
		mid, err := roles.EncodeRoleToMiddle(role)
		if err != nil {
			return fmt.Errorf("drule[Transaction]StoreRole: %v", err)
		}
		rolec.role = &mid
		rolec.be_delete = TRAN_ROLE_BE_DELETE_NO
	} else {
		mid, err := roles.EncodeRoleToMiddle(role)
		if err != nil {
			return fmt.Errorf("drule[Transaction]StoreRole: %v", err)
		}
		rolec, err = t.tran_service.addRole(t.unid, area, mid)
		if err != nil {
			return fmt.Errorf("drule[Transaction]StoreRole: %v", err)
		}
		t.tran_cache[roleid] = rolec
	}
	rolec.be_change = true
	return nil
}

func (t *Transaction) StoreRoleFromMiddleData(area string, mid roles.RoleMiddleData) (err error) {
	if t.be_delete == true {
		return fmt.Errorf("drule[Transaction]StoreRole: This transaction has been deleted.")
	}
	t.tran_time = time.Now()
	t.lock.RLock()
	defer t.lock.RUnlock()
	roleid := mid.Version.Id
	var find bool
	rolec, find := t.tran_cache[roleid]
	if find == true {
		rolec.role = &mid
		rolec.be_delete = TRAN_ROLE_BE_DELETE_NO
	} else {
		rolec, err = t.tran_service.addRole(t.unid, area, mid)
		if err != nil {
			return fmt.Errorf("drule[Transaction]StoreRole: %v", err)
		}
		t.tran_cache[roleid] = rolec
	}
	rolec.be_change = true
	return nil
}

func (t *Transaction) StoreRoleByte(area string, b []byte) (err error) {
	if t.be_delete == true {
		return fmt.Errorf("drule[Transaction]StoreRole: This transaction has been deleted.")
	}
	t.tran_time = time.Now()
	t.lock.RLock()
	defer t.lock.RUnlock()
	rolemid := roles.RoleMiddleData{}
	err = iendecode.BytesGobStruct(b, &rolemid)
	if err != nil {
		return fmt.Errorf("drule[Transaction]storeRoleByte: %v", err)
	}
	roleid := rolemid.Version.Id
	var find bool
	rolec, find := t.tran_cache[roleid]
	if find == true {
		rolec.role = &rolemid
		rolec.be_delete = TRAN_ROLE_BE_DELETE_NO
	} else {
		rolec, err = t.tran_service.addRole(t.unid, area, rolemid)
		if err != nil {
			return fmt.Errorf("drule[Transaction]StoreRole: %v", err)
		}
		t.tran_cache[roleid] = rolec
	}
	rolec.be_change = true
	return nil
}

// 删除一个角色
func (t *Transaction) DeleteRole(area, id string) (err error) {
	if t.be_delete == true {
		return fmt.Errorf("drule[Transaction]DeleteRole: This transaction has been deleted.")
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_WRITE)
	if err != nil {
		return err
	}
	rolec.be_delete = TRAN_ROLE_BE_DELETE_YES
	return nil
}

// 设置父角色
func (t *Transaction) WriteFather(area, id, father string) (err error) {
	if t.be_delete == true {
		return fmt.Errorf("drule[Transaction]WriteFather: This transaction has been deleted.")
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_WRITE)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]WriteFather:  %v", err)
		return
	}
	rolec.lock.Lock()
	defer rolec.lock.Unlock()
	rolec.role.Relation.Father = father
	rolec.be_change = true
	return
}

// 读出父角色
func (t *Transaction) ReadFather(area, id string) (father string, err error) {
	if t.be_delete == true {
		return "", fmt.Errorf("drule[Transaction]ReadFather: This transaction has been deleted.")
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_READ)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]ReadFather: %v", err)
		return
	}
	father = rolec.role.GetFather()
	return
}

// 重置父角色
func (t *Transaction) ResetFather(area, id string) (err error) {
	return t.WriteFather(area, id, "")
}

// 获取所有子角色名
func (t *Transaction) ReadChildren(area, id string) (children []string, err error) {
	if t.be_delete == true {
		err = fmt.Errorf("drule[Transaction]ReadChildren: This transaction has been deleted.")
		return
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_READ)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]ReadChildren: %v", err)
		return
	}
	children = rolec.role.GetChildren()
	return
}

// 写入角色的所有子角色名
func (t *Transaction) WriteChildren(area, id string, children []string) (err error) {
	if t.be_delete == true {
		err = fmt.Errorf("drule[Transaction]WriteChildren: This transaction has been deleted.")
		return
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_WRITE)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]WriteChildren: %v", err)
		return
	}
	rolec.lock.Lock()
	defer rolec.lock.Unlock()
	rolec.role.SetChildren(children)
	rolec.be_change = true
	return
}

// 重置角色
func (t *Transaction) ResetChildren(area, id string) (err error) {
	children := make([]string, 0)
	return t.WriteChildren(area, id, children)
}

// 写一个子角色关系
func (t *Transaction) WriteChild(area, id, child string) (err error) {
	if t.be_delete == true {
		return fmt.Errorf("drule[Transaction]WriteChild: This transaction has been deleted.")
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_WRITE)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]WriteChild: %v", err)
		return
	}
	rolec.lock.Lock()
	defer rolec.lock.Unlock()
	rolec.role.AddChild(child)
	rolec.be_change = true
	return
}

// 删除一个子角色关系
func (t *Transaction) DeleteChild(area, id, child string) (err error) {
	if t.be_delete == true {
		return fmt.Errorf("drule[Transaction]DeleteChild: This transaction has been deleted.")
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_WRITE)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]DeleteChild: %v", err)
		return
	}
	rolec.lock.Lock()
	defer rolec.lock.Unlock()
	rolec.role.DeleteChild(child)
	rolec.be_change = true
	return
}

// 是否含有这个子角色关系
func (t *Transaction) ExistChild(area, id, child string) (have bool, err error) {
	if t.be_delete == true {
		err = fmt.Errorf("drule[Transaction]ExistChild: This transaction has been deleted.")
		return
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_READ)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]ExistChild: %v", err)
		return
	}
	have = rolec.role.ExistChild(child)
	return
}

// 读取所有的朋友关系
func (t *Transaction) ReadFriends(area, id string) (friends map[string]roles.Status, err error) {
	if t.be_delete == true {
		err = fmt.Errorf("drule[Transaction]ReadFriends: This transaction has been deleted.")
		return
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_READ)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]ReadFriends: %v", err)
		return
	}
	friends = rolec.role.GetFriends()
	return
}

// 写入角色的所有朋友关系
func (t *Transaction) WriteFriends(area, id string, friends map[string]roles.Status) (err error) {
	if t.be_delete == true {
		err = fmt.Errorf("drule[Transaction]WriteFriends: This transaction has been deleted.")
		return
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_WRITE)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]WriteFriends: %v", err)
		return
	}
	rolec.role.SetFriends(friends)
	rolec.be_change = true
	return
}

// 重置所有朋友关系
func (t *Transaction) ResetFriends(area, id string) (err error) {
	friends := make(map[string]roles.Status)
	return t.WriteFriends(area, id, friends)
}

// 加入一个朋友关系，并绑定，已经有的关系将之修改绑定值。
//
// 这是WriteFriendStatus绑定状态的特例，也就是绑定位为0,绑定值为int64类型。
func (t *Transaction) WriteFriend(area, id, friend string, bind int64) (err error) {
	err = t.WriteFriendStatus(area, id, friend, 0, bind)
	return
}

// 设置朋友的状态
func (t *Transaction) WriteFriendStatus(area, id, friend string, bindbit int, value interface{}) (err error) {
	if t.be_delete == true {
		err = fmt.Errorf("drule[Transaction]WriteFriendStatus: This transaction has been deleted.")
		return
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_WRITE)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]WriteFriendStatus: %v", err)
		return
	}
	err = rolec.role.SetFriendStatus(friend, bindbit, value)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]WriteFriendStatus: %v", err)
	}
	rolec.be_change = true
	return
}

// 读取朋友的状态
func (t *Transaction) ReadFriendStatus(area, id, friend string, bindbit int, value interface{}) (have bool, err error) {
	if t.be_delete == true {
		err = fmt.Errorf("drule[Transaction]ReadFriendStatus: This transaction has been deleted.")
		return
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_READ)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]ReadFriendStatus: %v", err)
		return
	}
	have, err = rolec.role.GetFriendStatus(friend, bindbit, value)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]ReadFriendStatus: %v", err)
	}
	return
}

// 删除一个朋友关系，没有则忽略
func (t *Transaction) DeleteFriend(area, id, friend string) (err error) {
	if t.be_delete == true {
		err = fmt.Errorf("drule[Transaction]DeleteFriend: This transaction has been deleted.")
		return
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_WRITE)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]DeleteFriend: %v", err)
		return
	}
	rolec.role.DeleteFriend(friend)
	rolec.be_change = true
	return
}

// 创建一个空的上下文
func (t *Transaction) CreateContext(area, id, contextname string) (err error) {
	if t.be_delete == true {
		err = fmt.Errorf("drule[Transaction]CreateContext: This transaction has been deleted.")
		return
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_WRITE)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]CreateContext: %v", err)
		return
	}
	err = rolec.role.NewContext(contextname)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]CreateContext: %v", err)
	}
	rolec.be_change = true
	return
}

// 是否有这个上下文
func (t *Transaction) ExistContext(area, id, contextname string) (have bool, err error) {
	if t.be_delete == true {
		err = fmt.Errorf("drule[Transaction]ExistContext: This transaction has been deleted.")
		return
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_READ)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]ExistContext: %v", err)
		return
	}
	have = rolec.role.ExistContext(contextname)
	return
}

// 删除掉一个上下文
func (t *Transaction) DropContext(area, id, contextname string) (err error) {
	if t.be_delete == true {
		err = fmt.Errorf("drule[Transaction]DropContext: This transaction has been deleted.")
		return
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_WRITE)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]DropContext: %v", err)
		return
	}
	rolec.role.DelContext(contextname)
	rolec.be_change = true
	return
}

// 返回某个上下文的全部信息，如果没有这个上下文则have返回false
func (t *Transaction) ReadContext(area, id, contextname string) (context roles.Context, have bool, err error) {
	if t.be_delete == true {
		err = fmt.Errorf("drule[Transaction]ReadContext: This transaction has been deleted.")
		return
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_READ)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]ReadContext: %v", err)
		return
	}
	context, have = rolec.role.GetContext(contextname)
	return
}

// 删除一个上下文的绑定，upordown为roles包中的CONTEXT_UP或CONTEXT_DOWN，binderole是绑定的角色id
func (t *Transaction) DeleteContextBind(area, id, contextname string, upordown roles.ContextUpDown, bindrole string) (err error) {
	if t.be_delete == true {
		err = fmt.Errorf("drule[Transaction]DeleteContextBind: This transaction has been deleted.")
		return
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_WRITE)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]DeleteContextBind: %v", err)
		return
	}
	if upordown == roles.CONTEXT_UP {
		rolec.role.DelContextUp(contextname, bindrole)
	} else if upordown == roles.CONTEXT_DOWN {
		rolec.role.DelContextDown(contextname, bindrole)
	} else {
		err = fmt.Errorf("drule[Transaction]DeleteContextBind: Must CONTEXT_UP or CONTEXT_DOWN.")
	}
	rolec.be_change = true
	return
}

// 返回某个上下文中的同样绑定值的所有，upordown为roles中的CONTEXT_UP或CONTEXT_DOWN，如果给定的contextname不存在，则have返回false。
func (t *Transaction) ReadContextSameBind(area, id, contextname string, upordown roles.ContextUpDown, bind int64) (rolesid []string, have bool, err error) {
	if t.be_delete == true {
		err = fmt.Errorf("drule[Transaction]ReadContextSameBind: This transaction has been deleted.")
		return
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_READ)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]ReadContextSameBind: %v", err)
		return
	}
	if upordown == roles.CONTEXT_UP {
		rolesid, have = rolec.role.GetContextUpSameBind(contextname, bind)
	} else if upordown == roles.CONTEXT_DOWN {
		rolesid, have = rolec.role.GetContextDownSameBind(contextname, bind)
	} else {
		err = fmt.Errorf("drule[Transaction]ReadContextSameBind: Must CONTEXT_UP or CONTEXT_DOWN.")
	}
	return
}

// 返回所有上下文组的名称
func (t *Transaction) ReadContextsName(area, id string) (names []string, err error) {
	if t.be_delete == true {
		err = fmt.Errorf("drule[Transaction]ReadContextsName: This transaction has been deleted.")
		return
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_READ)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]ReadContextsName: %v", err)
		return
	}
	names = rolec.role.GetContextsName()
	return
}

// 设定上下文的状态属性，upordown为roles中的CONTEXT_UP或CONTEXT_DOWN
func (t *Transaction) WriteContextStatus(area, id, contextname string, upordown roles.ContextUpDown, bindroleid string, bindbit int, value interface{}) (err error) {
	if t.be_delete == true {
		err = fmt.Errorf("drule[Transaction]WriteContextStatus: This transaction has been deleted.")
		return
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_WRITE)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]WriteContextStatus: %v", err)
		return
	}
	err = rolec.role.SetContextStatus(contextname, upordown, bindroleid, bindbit, value)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]WriteContextStatus: %v", err)
	}
	rolec.be_change = true
	return
}

// 获取上下文的状态属性，upordown为roles.CONTEXT_UP或roles.CONTEXT_DOWN
func (t *Transaction) ReadContextStatus(area, id, contextname string, upordown roles.ContextUpDown, bindroleid string, bindbit int, value interface{}) (have bool, err error) {
	if t.be_delete == true {
		err = fmt.Errorf("drule[Transaction]ReadContextStatus: This transaction has been deleted.")
		return
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_READ)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]ReadContextStatus: %v", err)
		return
	}
	have, err = rolec.role.GetContextStatus(contextname, upordown, bindroleid, bindbit, value)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]ReadContextStatus: %v", err)
	}
	return
}

// 设定上下文
func (t *Transaction) WriteContexts(area, id string, contexts map[string]roles.Context) (err error) {
	if t.be_delete == true {
		err = fmt.Errorf("drule[Transaction]WriteContexts: This transaction has been deleted.")
		return
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_WRITE)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]WriteContexts: %v", err)
		return
	}
	rolec.role.SetContexts(contexts)
	rolec.be_change = true
	return
}

// 获得上下文
func (t *Transaction) ReadContexts(area, id string) (contexts map[string]roles.Context, err error) {
	if t.be_delete == true {
		err = fmt.Errorf("drule[Transaction]ReadContexts: This transaction has been deleted.")
		return
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_READ)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]ReadContexts: %v", err)
		return
	}
	contexts = rolec.role.GetContexts()
	return
}

// 重置上下文
func (t *Transaction) ResetContexts(area, id string) (err error) {
	contexts := make(map[string]roles.Context)
	return t.WriteContexts(area, id, contexts)
}

// 把data的数据装入role的name值下，如果找不到name，则返回错误
func (t *Transaction) WriteData(area, id, name string, data interface{}) (err error) {
	if t.be_delete == true {
		err = fmt.Errorf("drule[Transaction]WriteData: This transaction has been deleted.")
		return
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_WRITE)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]WriteData: %v", err)
		return
	}
	err = rolec.role.SetData(name, data)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]WriteData: %v", err)
	}
	rolec.be_change = true
	return
}

// 从Byte写入Data，这是一个内部的函数
func (t *Transaction) WriteDataFromByte(area, id, name string, data_b []byte) (err error) {
	if t.be_delete == true {
		err = fmt.Errorf("drule[Transaction]writeDataFromByte: This transaction has been deleted.")
		return
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_WRITE)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]writeDataFromByte: %v", err)
		return
	}
	err = rolec.role.SetDataFromByte(name, data_b)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]writeDataFromByte: %v", err)
	}
	rolec.be_change = true
	return
}

// 从角色中找到name的数据名并返回其数据
func (t *Transaction) ReadData(area, id, name string, data interface{}) (err error) {
	if t.be_delete == true {
		err = fmt.Errorf("drule[Transaction]ReadData: This transaction has been deleted.")
		return
	}
	t.tran_time = time.Now()

	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_READ)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]ReadData: %v", err)
		return
	}
	err = rolec.role.GetData(name, data)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]ReadData: %v", err)
	}
	return
}

func (t *Transaction) ReadDataToByte(area, id, name string) (data []byte, err error) {
	if t.be_delete == true {
		err = fmt.Errorf("drule[Transaction]ReadData: This transaction has been deleted.")
		return
	}
	t.tran_time = time.Now()
	rolec, _, err := t.getrole(area, id, TRAN_LOCK_MODE_READ)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]ReadData: %v", err)
		return
	}
	data, err = rolec.role.GetDataToByte(name)
	if err != nil {
		err = fmt.Errorf("drule[Transaction]ReadData: %v", err)
		return
	}
	return
}

// 返回事务ID
func (t *Transaction) TransactionId() (id string) {
	return t.unid
}

// 事务执行的处理
func (t *Transaction) Commit() (err error) {
	if t.be_delete == true {
		return fmt.Errorf("drule[Transaction]Commit: This transaction has been deleted.")
	}
	// 构造事务执行的处理信号
	commit_signal := &tranCommitSignal{
		tran_id:       t.unid,
		ask:           TRAN_COMMIT_ASK_COMMIT,
		return_handle: make(chan tranReturnHandle),
	}
	fmt.Println("Commit send 1")
	// 发送出去
	t.tran_commit_signal <- commit_signal
	fmt.Println("Commit send 2")
	// 开始等返回
	return_sigle := <-commit_signal.return_handle
	fmt.Println("Commit send 3")
	//fmt.Println("等到了返回：", t.unid)
	if return_sigle.Status != TRAN_RETURN_HANDLE_OK {
		return return_sigle.Error
	}
	return
}

// 事务的回滚处理
func (t *Transaction) Rollback() (err error) {
	if t.be_delete == true {
		return fmt.Errorf("drule[Transaction]Rollback: This transaction has been deleted.")
	}
	// 构造事务执行的处理信号
	rollback_signal := &tranCommitSignal{
		tran_id:       t.unid,
		ask:           TRAN_COMMIT_ASK_ROLLBACK,
		return_handle: make(chan tranReturnHandle),
	}
	// 发送信号
	t.tran_commit_signal <- rollback_signal
	// 开始等返回
	return_signle := <-rollback_signal.return_handle
	//fmt.Println("等到了返回：", t.unid)
	if return_signle.Status != TRAN_RETURN_HANDLE_OK {
		return return_signle.Error
	}
	return
}

// 输入将锁定的角色ID，让事务可以先尝试获得写权限，可重复使用，但每次只能针对一个area中的角色
func (t *Transaction) LockRole(area string, roleids ...string) (err error) {
	err = t.prepare(area, roleids)
	if err != nil {
		return fmt.Errorf("drule[Transaction]Prepare: %v", err)
	}
	return
}

func (t *Transaction) prepare(area string, roleids []string) (err error) {
	errall := make([]string, 0)
	for _, oneid := range roleids {
		_, _, errn := t.getrole(area, oneid, TRAN_LOCK_MODE_WRITE)
		if errn != nil {
			errall = append(errall, errn.Error())
		}
	}
	if len(errall) != 0 {
		errstr := strings.Join(errall, " | ")
		err = fmt.Errorf(errstr)
	}
	return
}

func (t *Transaction) getrole(area, id string, lockmode uint8) (rolec *roleCache, exist bool, err error) {
	t.lock.RLock()
	defer t.lock.RUnlock()
	exist = true
	var find bool
	cache_id := area + id
	rolec, find = t.tran_cache[cache_id]
	if find == true {
		rolec.tran_time = time.Now()
		return
	} else {
		rolec, exist, err = t.tran_service.getRole(t.unid, area, id, lockmode)
		if err != nil {
			return nil, false, err
		}
		if rolec.be_delete == TRAN_ROLE_BE_DELETE_COMMIT {
			return nil, false, fmt.Errorf("The role %v has already be deleted.", id)
		}
		if err == nil && lockmode == TRAN_LOCK_MODE_WRITE {
			t.tran_cache[cache_id] = rolec
		}
		rolec.tran_time = time.Now()
		return
	}
}
