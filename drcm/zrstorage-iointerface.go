// Copyright 2016-2017
// CoderG the 2016 project
// Insight 0+0 [ 洞悉 0+0 ]
// InDimensions Construct Source [ 忆黛蒙逝·建造源 ]
// Normal Fire Meditation Qin [ 火志溟 ] -> firemeditation@gmail.com
// Use of this source code is governed by GNU LGPL v3 license

package drcm

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/idcsource/Insight-0-0-lib/nst"
	"github.com/idcsource/Insight-0-0-lib/random"
	"github.com/idcsource/Insight-0-0-lib/roles"
)

// 从永久存储读出一个角色
func (z *ZrStorage) ReadRole(id string) (role roles.Roleer, err error) {
	// 如果启用了缓存，则启用全局的读锁。
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
	}
	// 查看缓存，如果缓存里有则从缓存里直接调用。
	rolec, find := z.rolesCache[id]
	if find == true {
		return rolec.role, nil
	}
	connmode, conn := z.findConn(id)
	if connmode == CONN_IS_LOCAL {
		// 如果是本地，就调用配套的hardstore的方法
		role, err = z.local_store.ReadRole(id)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]ReadRole: %v", err)
			return nil, err
		}
	} else {
		// 如果没有，因为是读取，所以就随机从一个slave中调用
		conncount := len(conn)
		connrandom := random.GetRandNum(conncount - 1)
		role, err = z.readRole(id, conn[connrandom])
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]ReadRole: %v", err)
			return nil, err
		}
	}
	// 如果开启了缓存，则存入缓存，并使其检查缓存
	if z.cacheMax >= 0 {
		z.rolesCache[id] = &oneRoleCache{
			lock: new(sync.RWMutex),
			role: role,
		}
		z.rolesCount++
	}
	if z.cacheMax > 0 {
		z.checkCacheNum()
	}
	return role, nil
}

// 本地的角色小读取，不带锁，但会加入缓存，并会检查缓存，返回的是oneRoleCache
func (z *ZrStorage) readRole_small(id string) (rolec *oneRoleCache, err error) {
	// 如果开启了缓存，就去找缓存
	if z.cacheMax >= 0 {
		if _, find := z.rolesCache[id]; find == true {
			return z.rolesCache[id], nil
		}
	}
	role, err := z.local_store.ReadRole(id)
	if err != nil {
		err = fmt.Errorf("drcm[ZrStorage]readRole_small: %v", err)
		return nil, err
	}
	// 如果开启了缓存，则存入缓存，并使其检查缓存
	if z.cacheMax >= 0 {
		z.rolesCache[id] = &oneRoleCache{
			lock: new(sync.RWMutex),
			role: role,
		}
		z.rolesCount++
	}
	if z.cacheMax > 0 {
		z.checkCacheNum()
	}
	return z.rolesCache[id], nil
}

// 从slave读取一个角色
//
//	--> OPERATE_READ_ROLE (前导)
//	<-- DATA_PLEASE (Net_SlaveReceipt回执)
//	--> 角色ID
//	<-- Net_RoleSendAndReceive (结构体，用Net_SlaveReceipt_Data封装)
func (z *ZrStorage) readRole(id string, slave *slaveIn) (role roles.Roleer, err error) {
	cprocess := slave.tcpconn.OpenProgress()
	defer cprocess.Close()
	slavereceipt, err := z.sendPrefixStat(cprocess, slave.code, OPERATE_READ_ROLE)
	if err != nil {
		return nil, err
	}
	// 如果获取到的DATA_PLEASE则说明认证已经通过
	if slavereceipt.DataStat != DATA_PLEASE {
		return nil, slavereceipt.Error
	}
	// 发送想要的id，并接收slave的返回
	slave_receipt_data, err := z.sendAndDecodeSlaveReceiptData(cprocess, []byte(id))
	if err != nil {
		return nil, err
	}
	if slave_receipt_data.DataStat != DATA_ALL_OK {
		return nil, slave_receipt_data.Error
	}
	// 解码Net_RoleSendAndReceive。
	rolegetstruct := Net_RoleSendAndReceive{}
	err = nst.BytesGobStruct(slave_receipt_data.Data, &rolegetstruct)
	if err != nil {
		return nil, err
	}
	// 合成出role来
	role, err = z.local_store.DecodeRole(rolegetstruct.RoleBody, rolegetstruct.RoleRela, rolegetstruct.RoleVer)
	return role, err
}

// 往永久存储写一个角色
func (z *ZrStorage) StoreRole(role roles.Roleer) (err error) {
	id := role.ReturnId()
	// 如果启用了缓存，则启用全局的读锁
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
		// 如果缓存里没有则加入缓存
		_, find := z.rolesCache[id]
		if find == false {
			z.rolesCache[id] = &oneRoleCache{
				lock: new(sync.RWMutex),
				role: role,
			}
			z.rolesCount++
		}
		// 如果缓存有个数要求，那么就检查个数要求
		if z.cacheMax > 0 {
			z.checkCacheNum()
		}
	}
	// 检查这个角色应该保存在哪里
	connmode, slaveconn := z.findConn(id)
	if connmode == CONN_IS_LOCAL {
		// 如果是本地保存
		err = z.local_store.StoreRole(role)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]StoreRole: %v", err)
		}
		return err
	} else {
		// 如果是slave保存
		err = z.storeRole(role, slaveconn)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]StoreRole: %v", err)
		}
		return err
	}
}

// 将角色保存到slave中，因为是保存所以需要将所有镜像同时保存
func (z *ZrStorage) storeRole(role roles.Roleer, conns []*slaveIn) (err error) {
	// 将角色编码，并生成传输所需要的Net_RoleSendAndReceive格式，并最终编码成为[]byte
	roleb, relab, verb, err := z.local_store.EncodeRole(role)
	if err != nil {
		return err
	}
	roleS := Net_RoleSendAndReceive{
		RoleBody: roleb,
		RoleRela: relab,
		RoleVer:  verb,
	}
	roleS_b, err := nst.StructGobBytes(roleS)
	if err != nil {
		return err
	}
	// 遍历slave的连接，如果slave出现错误就输出，继续下一个结点
	var errstring string
	for _, onec := range conns {
		err = z.storeRole_one(roleS_b, onec)
		if err != nil {
			errstring += fmt.Sprint(onec.name, ": ", err, " | ")
		}
	}
	if len(errstring) != 0 {
		return fmt.Errorf(errstring)
	}
	return nil
}

// 存储的一个slave连接
//
//	--> OPERATE_WRITE_ROLE (前导)
//	<-- DATA_PLEASE (slave回执)
//	--> Net_RoleSendAndReceive (结构体)
//	<-- DATA_ALL_OK (salve回执)
func (z *ZrStorage) storeRole_one(roleS_b []byte, onec *slaveIn) (err error) {
	cprocess := onec.tcpconn.OpenProgress()
	defer cprocess.Close()
	//发送前导
	slavereceipt, err := z.sendPrefixStat(cprocess, onec.code, OPERATE_WRITE_ROLE)
	if err != nil {
		return err
	}
	// 如果slave请求发送数据
	if slavereceipt.DataStat == DATA_PLEASE {
		srb, err := cprocess.SendAndReturn(roleS_b)
		if err != nil {
			return err
		}
		sr, err := z.decodeSlaveReceipt(srb)
		if err != nil {
			return err
		}
		if sr.DataStat != DATA_ALL_OK {
			return sr.Error
		}
		return nil
	} else {
		return slavereceipt.Error
	}
}

// 删除一个角色
func (z *ZrStorage) DeleteRole(id string) (err error) {
	// 如果启用了缓存，则启用全局的读锁
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
		// 检查自己的缓存里有没有这个家伙，如果有先删除之，因为是删除，所以就不触发缓存个数检查了
		_, find := z.rolesCache[id]
		if find == true {
			delete(z.rolesCache, id)
			z.rolesCount--
		}
	}
	// 检查这个角色应该保存在哪里
	connmode, slaveconn := z.findConn(id)
	if connmode == CONN_IS_LOCAL {
		// 如果这个角色在本地，那么就调用本地的删除之
		err = z.local_store.DeleteRole(id)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]DeleteRole: %v", err)
			return
		}
	} else {
		// 如果是slave
		err = z.deleteRole(id, slaveconn)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]DeleteRole: %v", err)
		}
		return err
	}
	return nil
}

// 向slave要求删除一个角色，需要将所有镜像同时删除，slave上不存在也是返回正常的
func (z *ZrStorage) deleteRole(id string, conns []*slaveIn) (err error) {
	// 遍历slave的连接，如果slave出现错误就输出，继续下一个结点
	var errstring string
	for _, onec := range conns {
		err = z.deleteRole_one(id, onec)
		if err != nil {
			errstring += fmt.Sprint(onec.name, ": ", err, " | ")
		}
	}
	if len(errstring) != 0 {
		return fmt.Errorf(errstring)
	}
	return nil
}

// 删除的一个slave链接
//
//	--> OPERATE_DEL_ROLE (前导)
//	<-- DATA_PLEASE (slave回执)
//	--> 角色ID
//	<-- DATA_ALL_OK (slave回执)
func (z *ZrStorage) deleteRole_one(id string, onec *slaveIn) (err error) {
	cprocess := onec.tcpconn.OpenProgress()
	defer cprocess.Close()
	//发送前导,OPERATE_DEL_ROLE
	slavereceipt, err := z.sendPrefixStat(cprocess, onec.code, OPERATE_DEL_ROLE)
	if err != nil {
		return err
	}
	// 如果slave请求发送数据
	if slavereceipt.DataStat == DATA_PLEASE {
		// 将id编码后发出去
		srb, err := cprocess.SendAndReturn([]byte(id))
		if err != nil {
			return err
		}
		// 解码返回值
		sr, err := z.decodeSlaveReceipt(srb)
		if err != nil {
			return err
		}
		if sr.DataStat != DATA_ALL_OK {
			return sr.Error
		}
		return nil
	} else {
		return slavereceipt.Error
	}
}

// 设置父角色
func (z *ZrStorage) WriteFather(id, father string) (err error) {
	// 如果启用了缓存，则启用全局的读锁
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
	}
	// 是否为本地
	connmode, slaveconn := z.findConn(id)
	if connmode == CONN_IS_LOCAL {
		// 如果为本地
		rolec, err := z.readRole_small(id)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]WriteFather: %v", err)
			return err
		}
		rolec.lock.Lock()
		defer rolec.lock.Unlock()
		rolec.role.SetFather(father)
		err = z.checkCacheStore(rolec.role)
		return err
	} else {
		// 如果是slave
		err = z.writeFather(id, father, slaveconn)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]WriteRole: %v", err)
		}
		return err
	}
}

// 发送slave设置角色的父角色
//
//	--> OPERATE_SET_FATHER (前导)
//	<-- DATA_PLEASE (slave回执)
//	--> Net_RoleFatherChange (结构)
//	<-- DATA_ALL_OK (slave回执)
func (z *ZrStorage) writeFather(id, father string, conns []*slaveIn) (err error) {
	// 构造要发送的信息
	sd := Net_RoleFatherChange{Id: id, Father: father}
	sdb, err := nst.StructGobBytes(sd)
	if err != nil {
		return err
	}
	// 遍历slave的连接，如果slave出现错误就输出，继续下一个结点
	var errstring string
	for _, onec := range conns {
		err = z.writeFather_one(sdb, onec)
		if err != nil {
			errstring += fmt.Sprint(onec.name, ": ", err, " | ")
		}
	}
	if len(errstring) != 0 {
		return fmt.Errorf(errstring)
	}
	return nil
}

// 发送slave设置角色的父角色——一个slave的
func (z *ZrStorage) writeFather_one(sdb []byte, onec *slaveIn) (err error) {
	cprocess := onec.tcpconn.OpenProgress()
	defer cprocess.Close()
	//发送前导，OPERATE_SET_FATHER
	slavereceipt, err := z.sendPrefixStat(cprocess, onec.code, OPERATE_SET_FATHER)
	if err != nil {
		return err
	}
	if slavereceipt.DataStat == DATA_PLEASE {
		sre, err := cprocess.SendAndReturn(sdb)
		if err != nil {
			return err
		}
		sr, err := z.decodeSlaveReceipt(sre)
		if err != nil {
			return err
		}
		if sr.DataStat != DATA_ALL_OK {
			return sr.Error
		}
		return nil
	} else {
		return slavereceipt.Error
	}
}

// 获取父角色的ID
func (z *ZrStorage) ReadFather(id string) (father string, err error) {
	// 如果启用了缓存，则启用全局的读锁
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
	}
	// 是否为本地
	connmode, slaveconn := z.findConn(id)
	if connmode == CONN_IS_LOCAL {
		// 如果是本地，则用ReadRole来读取
		rolec, err := z.readRole_small(id)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]ReadFather: %v", err)
			return "", err
		}
		// 给这个角色加读锁
		rolec.lock.RLock()
		defer rolec.lock.RUnlock()
		father = rolec.role.GetFather()
		return father, nil
	} else {
		// 如果不是本地，因为是读取，所以就随机从一个slave中调用
		conncount := len(slaveconn)
		connrandom := random.GetRandNum(conncount - 1)
		father, err = z.readFather(id, slaveconn[connrandom])
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]ReadFather: %v", err)
		}
		return
	}
}

// 一个slave的返回父亲
//
//	分配连接进程
//	--> OPERATE_GET_FATHER (前导)
//	<-- DATA_PLEASE (slave回执)
//	--> role's id (角色id的byte)
//	<-- father's id (父角色id的byte，Net_SlaveReceipt_Data封装)
func (z *ZrStorage) readFather(id string, conn *slaveIn) (father string, err error) {
	// 分配连接进程
	cprocess := conn.tcpconn.OpenProgress()
	defer cprocess.Close()
	// 发送前导词，OPERATE_GET_FATHER
	slavereceipt, err := z.sendPrefixStat(cprocess, conn.code, OPERATE_GET_FATHER)
	if err != nil {
		return "", err
	}
	if slavereceipt.DataStat == DATA_PLEASE {
		// 将自己的id发送出去
		slave_receipt_data, err := z.sendAndDecodeSlaveReceiptData(cprocess, []byte(id))
		if err != nil {
			return "", err
		}
		if slave_receipt_data.DataStat != DATA_ALL_OK {
			return "", slave_receipt_data.Error
		}
		father = string(slave_receipt_data.Data)
		return father, nil
	} else {
		return "", slavereceipt.Error
	}
}

// 重置父角色，这里只是调用WriteFather
func (z *ZrStorage) ResetFather(id string) error {
	return z.WriteFather(id, "")
}

// 读取角色的所有子角色名
func (z *ZrStorage) ReadChildren(id string) (children []string, err error) {
	// 如果启用了缓存，则启用全局读锁
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
	}
	// 是否为本地
	connmode, conn := z.findConn(id)
	if connmode == CONN_IS_LOCAL {
		// 本地的解决方案
		rolec, err := z.readRole_small(id)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]ReadChildren: %v", err)
			return children, err
		}
		// 给这个角色加读锁
		rolec.lock.RLock()
		defer rolec.lock.RUnlock()
		// 从角色获得信息
		children = rolec.role.GetChildren()
		return children, nil
	} else {
		// 如果需要来slave
		conn_count := len(conn)
		conn_random := random.GetRandNum(conn_count - 1)
		children, err = z.readChildren(id, conn[conn_random])
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]ReadChildren: %v", err)
			return children, err
		} else {
			return children, nil
		}
	}
}

// 从slave读出一个角色的children
//
//	分配连接进程
//	--> OPERATE_GET_CHILDREN (前导)
//	<-- DATA_PLEASE (slave回执)
//	--> role's id (角色的id)
//	<-- children's id ([]string，Net_SlaveReceipt_Data封装)
func (z *ZrStorage) readChildren(id string, conn *slaveIn) (children []string, err error) {
	// 分配连接进程
	cprocess := conn.tcpconn.OpenProgress()
	defer cprocess.Close()
	// 发送前导 OPERATE_GET_CHILDREN
	sr, err := z.sendPrefixStat(cprocess, conn.code, OPERATE_GET_CHILDREN)
	if err != nil {
		return
	}
	if sr.DataStat == DATA_PLEASE {
		// 发送要查询的id
		sr_data, err := z.sendAndDecodeSlaveReceiptData(cprocess, []byte(id))
		if err != nil {
			return children, err
		}
		if sr_data.DataStat != DATA_ALL_OK {
			return children, sr_data.Error
		}
		children = make([]string, 0)
		err = nst.BytesGobStruct(sr_data.Data, &children)
		return children, err
	} else {
		err = sr.Error
		return children, err
	}
}

// 写入角色的所有子角色名
func (z *ZrStorage) WriteChildren(id string, children []string) (err error) {
	// 如果启用了缓存，则启用全局的读锁
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
	}
	// 是否为本地
	mode, conn := z.findConn(id)
	if mode == CONN_IS_LOCAL {
		// 如果在本地
		rolec, err := z.readRole_small(id)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]WriteChildren: %v", err)
			return err
		}
		// 锁住这个角色
		rolec.lock.Lock()
		defer rolec.lock.Unlock()
		rolec.role.SetChildren(children)
		err = z.checkCacheStore(rolec.role)
		return err
	} else {
		// 如果在slave
		err = z.writeChildren(id, children, conn)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]WriteChildren: %v", err)
		}
		return err
	}
}

// 在slave上设置children
func (z *ZrStorage) writeChildren(id string, children []string, conns []*slaveIn) (err error) {
	// 构造要传输的信息
	role_children := Net_RoleAndChildren{
		Id:       id,
		Children: children,
	}
	children_b, err := nst.StructGobBytes(role_children)
	if err != nil {
		return err
	}
	// 遍历所有的连接
	var errstring string
	for _, onec := range conns {
		err = z.writeChildren_one(id, children_b, onec)
		if err != nil {
			errstring += fmt.Sprint(onec.name, ": ", err, " | ")
		}
	}
	if len(errstring) != 0 {
		return fmt.Errorf(errstring)
	} else {
		return nil
	}
}

// 向某一个slavev发送设置children的内容
//
//	分配连接
//	--> OPERATE_SET_CHILDREN (前导)
//	<-- DATA_PLEASE (slave回执)
//	--> Net_RoleAndChildren
//	<-- DATA_ALL_OK (slave回执)
func (z *ZrStorage) writeChildren_one(id string, children_b []byte, onec *slaveIn) (err error) {
	cprocess := onec.tcpconn.OpenProgress()
	defer cprocess.Close()
	// OPREATE_SET_CHILDREN 前导
	slave_receipt, err := z.sendPrefixStat(cprocess, onec.code, OPERATE_SET_CHILDREN)
	if err != nil {
		return err
	}
	if slave_receipt.DataStat != DATA_PLEASE {
		return slave_receipt.Error
	}
	// 发送children
	slave_receipt, err = z.sendAndDecodeSlaveReceipt(cprocess, children_b)
	if err != nil {
		return err
	}
	if slave_receipt.DataStat != DATA_ALL_OK {
		return slave_receipt.Error
	}
	return nil
}

// 重置角色的子角色关系，只是调用WriteCildren
func (z *ZrStorage) ResetChildren(id string) (err error) {
	children := make([]string, 0)
	return z.WriteChildren(id, children)
}

// 写入一个子角色关系
func (z *ZrStorage) WriteChild(id, child string) (err error) {
	// 如果启用了缓存，启用全局读锁
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
	}
	// 检查是否为本地
	mode, conn := z.findConn(id)
	if mode == CONN_IS_LOCAL {
		// 如果是本地，则读出或查看缓存什么的
		rolec, err := z.readRole_small(id)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]WriteChild: %v", err)
			return err
		}
		// 给这个role加锁
		rolec.lock.Lock()
		defer rolec.lock.Unlock()
		// 设置关系
		rolec.role.AddChild(child)
		err = z.checkCacheStore(rolec.role)
		return err
	} else {
		// 如果是slave的
		err = z.writeChild(id, child, conn)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]WriteChild: %v", err)
		}
		return err
	}
}

// 向slave发送添加child关系的命令
func (z *ZrStorage) writeChild(id, child string, conns []*slaveIn) (err error) {
	// 构建要发送的信息
	role_child := Net_RoleAndChild{
		Id:    id,
		Child: child,
	}
	role_child_b, err := nst.StructGobBytes(role_child)
	if err != nil {
		return err
	}
	//遍历slave连接
	var errstr string
	for _, onec := range conns {
		err = z.writeChild_one(role_child_b, onec)
		if err != nil {
			errstr += fmt.Sprint(onec.name, ": ", err, " | ")
		}
	}
	if len(errstr) != 0 {
		return fmt.Errorf(errstr)
	} else {
		return nil
	}
}

// 向其中一个slave发送添加child的命令
//
//	--> OPERATE_ADD_CHILD (前导)
//	<-- DATA_PLEASE (slave回执)
//	--> Net_RoleAndChild (结构体)
//	<-- DATA_ALL_OK (slave回执)
func (z *ZrStorage) writeChild_one(role_child_b []byte, onec *slaveIn) (err error) {
	// 分配连接
	cprocess := onec.tcpconn.OpenProgress()
	defer cprocess.Close()
	// 发送前导
	slave_reply, err := z.sendPrefixStat(cprocess, onec.code, OPERATE_ADD_CHILD)
	if err != nil {
		return err
	}
	if slave_reply.DataStat != DATA_PLEASE {
		return slave_reply.Error
	}
	slave_reply, err = z.sendAndDecodeSlaveReceipt(cprocess, role_child_b)
	if err != nil {
		return err
	}
	if slave_reply.DataStat != DATA_ALL_OK {
		return slave_reply.Error
	}
	return nil
}

// 从永久存储里删除一个子角色关系
func (z *ZrStorage) DeleteChild(id, child string) (err error) {
	// 如果启动了缓存，则启动全局锁
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
	}
	// 查询是否本地
	mode, conn := z.findConn(id)
	if mode == CONN_IS_LOCAL {
		// 如果是本地
		rolec, err := z.readRole_small(id)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]DeleteChild: %v", err)
			return err
		}
		rolec.lock.Lock()
		defer rolec.lock.Unlock()
		rolec.role.DeleteChild(child)
		return nil
	} else {
		// 如果是slave
		err = z.deleteChild(id, child, conn)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]DeleteChild: %v", err)
		}
		return err
	}
}

// 让slave删除一个child关系
func (z *ZrStorage) deleteChild(id, child string, conns []*slaveIn) (err error) {
	// 构造要发送的信息
	role_child := Net_RoleAndChild{
		Id:    id,
		Child: child,
	}
	role_child_b, err := nst.StructGobBytes(role_child)
	if err != nil {
		return err
	}
	// 遍历slave连接
	var errstr string
	for _, onec := range conns {
		err = z.deleteChild_one(role_child_b, onec)
		if err != nil {
			errstr += fmt.Sprint(onec.name, ": ", err, " | ")
		}
	}
	if len(errstr) != 0 {
		return fmt.Errorf(errstr)
	}
	return nil
}

// 对某一个slave发送删除child关系的请求
//
//	--> OPERATE_DEL_CHILD (前导)
//	<-- DATA_PLEASE (slave回执)
//	--> Net_RoleAndChild (结构体)
//	<-- DATA_ALL_OK (slave回执)
func (z *ZrStorage) deleteChild_one(role_child_b []byte, onec *slaveIn) (err error) {
	// 分配连接
	cprocess := onec.tcpconn.OpenProgress()
	defer cprocess.Close()
	// 发送前导
	slave_reply, err := z.sendPrefixStat(cprocess, onec.code, OPERATE_DEL_CHILD)
	if err != nil {
		return err
	}
	if slave_reply.DataStat != DATA_PLEASE {
		return slave_reply.Error
	}
	// 发送数据
	slave_reply, err = z.sendAndDecodeSlaveReceipt(cprocess, role_child_b)
	if err != nil {
		return err
	}
	if slave_reply.DataStat != DATA_ALL_OK {
		return slave_reply.Error
	}
	return nil
}

// 查询是否有这个子角色关系，如果有则返回true
func (z *ZrStorage) ExistChild(id, child string) (have bool, err error) {
	// 如果启动了缓存，则全局读锁
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
	}
	// 判断是否为本地
	mode, conn := z.findConn(id)
	if mode == CONN_IS_LOCAL {
		// 如果是本地
		rolec, err := z.readRole_small(id)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]ExistChild: %v", err)
			return false, err
		}
		// 给角色加读锁
		rolec.lock.RLock()
		defer rolec.lock.RUnlock()
		// 调用角色
		have = rolec.role.ExistChild(child)
		return have, nil
	} else {
		// 如果是远端，随机找个镜像出来
		conn_count := len(conn)
		conn_random := random.GetRandNum(conn_count - 1)
		have, err = z.existChild(id, child, conn[conn_random])
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]ExistChild: %v", err)
		}
		return have, err
	}
}

// 从slave中查看是否有那么一个child角色
//
//	--> OPERATE_EXIST_CHILD (前导)
//	<-- DATA_PLEASE (slave回执)
//	--> Net_RoleAndChild (结构体)
//	<-- DATA_RETURN_IS_TRUE 或 DATA_RETURN_IS_FALSE (slave回执)
func (z *ZrStorage) existChild(id, child string, conn *slaveIn) (have bool, err error) {
	// 分配进程
	cprocess := conn.tcpconn.OpenProgress()
	defer cprocess.Close()
	// 发送前导OPERATE_EXIST_CHILD
	slave_reply, err := z.sendPrefixStat(cprocess, conn.code, OPERATE_EXIST_CHILD)
	if err != nil {
		return false, err
	}
	if slave_reply.DataStat != DATA_PLEASE {
		return false, slave_reply.Error
	}
	// 创建要发送的结构体
	role_child := Net_RoleAndChild{
		Id:    id,
		Child: child,
	}
	role_child_b, err := nst.StructGobBytes(role_child)
	if err != nil {
		return false, err
	}
	// 向slave发送查询的结构体
	slave_reply, err = z.sendAndDecodeSlaveReceipt(cprocess, role_child_b)
	if err != nil {
		return false, err
	}
	if slave_reply.DataStat == DATA_RETURN_IS_TRUE {
		return true, nil
	} else if slave_reply.DataStat == DATA_RETURN_IS_FALSE {
		return false, nil
	} else {
		return false, slave_reply.Error
	}
}

// 读取id的所有朋友关系
func (z *ZrStorage) ReadFriends(id string) (status map[string]roles.Status, err error) {
	// 如果启用缓存，则全局读锁
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
	}
	// 是否本地
	mode, conn := z.findConn(id)
	if mode == CONN_IS_LOCAL {
		// 本地的处理
		rolec, err := z.readRole_small(id)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]ReadFriends: %v", err)
			return nil, err
		}
		rolec.lock.RLock()
		defer rolec.lock.RUnlock()
		// 从角色获取信息
		status := rolec.role.GetFriends()
		return status, nil
	} else {
		// slave上处理
		conn_count := len(conn)
		conn_random := random.GetRandNum(conn_count - 1)
		status, err = z.readFriends(id, conn[conn_random])
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]ReadFriends: %v", err)
		}
		return status, err
	}
}

// 从slave中读取一个角色的friends关系
//
//	--> OPERATE_GET_FRIENDS (前导词)
//	<-- DATA_PLEASE (slave回执)
//	--> role's id (角色ID)
//	<-- friends's status (map[string]roles.Status，Net_SlaveReceipt_Data封装)
func (z *ZrStorage) readFriends(id string, conn *slaveIn) (status map[string]roles.Status, err error) {
	// 分配连接
	cprocess := conn.tcpconn.OpenProgress()
	defer cprocess.Close()
	// 发送前导OPERATE_GET_FRIENDS
	slave_reply, err := z.sendPrefixStat(cprocess, conn.code, OPERATE_GET_FRIENDS)
	if err != nil {
		return nil, err
	}
	if slave_reply.DataStat != DATA_PLEASE {
		return nil, slave_reply.Error
	}
	// 发送角色的ID
	slave_reply_data, err := z.sendAndDecodeSlaveReceiptData(cprocess, []byte(id))
	if err != nil {
		return nil, err
	}
	if slave_reply_data.DataStat != DATA_ALL_OK {
		return nil, slave_reply_data.Error
	}
	// 解码status
	status = make(map[string]roles.Status)
	err = nst.BytesGobStruct(slave_reply_data.Data, &status)
	return status, err
}

// 写入角色的所有朋友关系
func (z *ZrStorage) WriteFriends(id string, friends map[string]roles.Status) (err error) {
	// 如果启用了缓存，则启用全局的读锁
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
	}
	// 是否为本地
	mode, conn := z.findConn(id)
	if mode == CONN_IS_LOCAL {
		// 如果为本地
		rolec, err := z.readRole_small(id)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]WriteFriend: %v", err)
			return err
		}
		// 加锁
		rolec.lock.Lock()
		defer rolec.lock.Unlock()
		// 调用角色的接口
		rolec.role.SetFriends(friends)
		err = z.checkCacheStore(rolec.role)
		return err
	} else {
		// 如果在slave
		err = z.writeFriends(id, friends, conn)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]WriteFriend: %v", err)
		}
		return err
	}
}

// 在slave上处理friends
func (z *ZrStorage) writeFriends(id string, friends map[string]roles.Status, conns []*slaveIn) (err error) {
	// 构造要传输的信息
	role_friends := Net_RoleAndFriends{
		Id:      id,
		Friends: friends,
	}
	friends_b, err := nst.StructGobBytes(role_friends)
	if err != nil {
		return err
	}
	// 遍历所有连接
	var errstr string
	for _, onec := range conns {
		err = z.writeFriends_one(id, friends_b, onec)
		if err != nil {
			errstr += fmt.Sprint(onec.name, ": ", err, " | ")
		}
	}
	if len(errstr) != 0 {
		return fmt.Errorf(errstr)
	} else {
		return nil
	}
}

// WriteFriends的向每一个slave发送
//
//	--> OPERATE_SET_FRIENDS (前导)
//	<-- DATA_PLEASE (slave回执)
//	--> Net_RoleAndFriends
//	<-- DATA_ALL_OK (slave回执)
func (z *ZrStorage) writeFriends_one(id string, friends_b []byte, onec *slaveIn) (err error) {
	cprocess := onec.tcpconn.OpenProgress()
	defer cprocess.Close()
	// 发送前导
	slave_receipt, err := z.sendPrefixStat(cprocess, onec.code, OPERATE_SET_FRIENDS)
	if err != nil {
		return err
	}
	if slave_receipt.DataStat != DATA_PLEASE {
		return slave_receipt.Error
	}
	// 发送Net_RoleAndFriends的byte
	slave_receipt, err = z.sendAndDecodeSlaveReceipt(cprocess, friends_b)
	if err != nil {
		return err
	}
	if slave_receipt.DataStat != DATA_ALL_OK {
		return slave_receipt.Error
	}
	return nil
}

// 重置角色的所有朋友关系，也就是发送一个空的朋友关系给WriteFriends
func (z *ZrStorage) ResetFriends(id string) (err error) {
	friends := make(map[string]roles.Status)
	return z.WriteFriends(id, friends)
}

// 加入一个朋友关系，并绑定，已经有的关系将之修改绑定值。
// 这是WriteFriendStatus绑定状态的特例，也就是绑定位为0,绑定值为int64类型。
func (z *ZrStorage) WriteFriend(id, friend string, bind int64) (err error) {
	err = z.WriteFriendStatus(id, friend, 0, bind)
	return
}

// 删除一个朋友关系，如果没有则忽略
func (z *ZrStorage) DeleteFriend(id, friend string) (err error) {
	// 如果启动缓存，启动全局读锁
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
	}
	// 是否为本地
	mode, conn := z.findConn(id)
	if mode == CONN_IS_LOCAL {
		// 如果是本地
		rolec, err := z.readRole_small(id)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]DeleteFriend: %v", err)
			return err
		}
		// 加锁
		rolec.lock.Lock()
		defer rolec.lock.Unlock()
		// 调用角色的接口
		rolec.role.DeleteFriend(friend)
		return nil
	} else {
		// 如果是slave
		err = z.deleteFriend(id, friend, conn)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]DeleteFriend: %v", err)
		}
		return err
	}
}

// 如果为slave的删除朋友关系
func (z *ZrStorage) deleteFriend(id, friend string, conns []*slaveIn) (err error) {
	// 构造要发送的信息
	role_friend := Net_RoleAndFriend{
		Id:     id,
		Friend: friend,
	}
	role_friend_b, err := nst.StructGobBytes(role_friend)
	if err != nil {
		return err
	}
	// 遍历连接
	var errstr string
	for _, onec := range conns {
		err = z.deleteFriend_one(role_friend_b, onec)
		if err != nil {
			errstr += fmt.Sprint(onec.name, ": ", err, " | ")
		}
	}
	if len(errstr) != 0 {
		return fmt.Errorf(errstr)
	} else {
		return nil
	}
}

// 一个slave的删除朋友关系
//
//	--> OPERATE_DEL_FRIEND (前导)
//	<-- DATA_PLEASE (slave回执)
//	--> Net_RoleAndFriend (结构体)
//	<-- DATA_ALL_OK (slave回执)
func (z *ZrStorage) deleteFriend_one(role_friend_b []byte, onec *slaveIn) (err error) {
	// 分配连接
	cprocess := onec.tcpconn.OpenProgress()
	defer cprocess.Close()
	// 发送前导
	slave_reply, err := z.sendPrefixStat(cprocess, onec.code, OPERATE_DEL_FRIEND)
	if err != nil {
		return err
	}
	if slave_reply.DataStat != DATA_PLEASE {
		return slave_reply.Error
	}
	// 发送数据
	slave_reply, err = z.sendAndDecodeSlaveReceipt(cprocess, role_friend_b)
	if err != nil {
		return err
	}
	if slave_reply.DataStat != DATA_ALL_OK {
		return slave_reply.Error
	}
	return nil
}

// 创建一个空的上下文，如果已经存在则忽略
func (z *ZrStorage) CreateContext(id, contextname string) (err error) {
	// 如果启用了缓存
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
	}
	// 检查是否为本地
	mode, conn := z.findConn(id)
	if mode == CONN_IS_LOCAL {
		// 如果是本地
		rolec, err := z.readRole_small(id)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]CreateContext: %v", err)
			return err
		}
		// 给角色加锁
		rolec.lock.Lock()
		defer rolec.lock.Unlock()
		// 调用角色接口
		rolec.role.NewContext(contextname)
		return nil
	} else {
		// 如果是slave
		err = z.createContext(id, contextname, conn)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]CreateContext: %v", err)
		}
		return err
	}
}

// 向slave要求创建上下文
func (z *ZrStorage) createContext(id, contextname string, conns []*slaveIn) (err error) {
	// 构建要发送的信息
	role_context := Net_RoleAndContext{
		Id:      id,
		Context: contextname,
	}
	role_context_b, err := nst.StructGobBytes(role_context)
	if err != nil {
		return err
	}
	// 遍历slave
	var errstr string
	for _, onec := range conns {
		err = z.createContext_one(role_context_b, onec)
		if err != nil {
			errstr += fmt.Sprint(onec.name, ": ", err, " | ")
		}
	}
	if len(errstr) != 0 {
		return fmt.Errorf(errstr)
	} else {
		return nil
	}
}

// 向某一个slave发送创建上下文的请求
//
//	--> OPERATE_ADD_CONTEXT (前导)
//	<-- DATA_PLEASE (slave回执)
//	--> Net_RoleAndContext (结构体)
//	<-- DATA_ALL_OK (slave回执)
func (z *ZrStorage) createContext_one(role_context_b []byte, onec *slaveIn) (err error) {
	// 分配连接
	cprocess := onec.tcpconn.OpenProgress()
	defer cprocess.Close()
	// 发送前导
	slave_reply, err := z.sendPrefixStat(cprocess, onec.code, OPERATE_ADD_CONTEXT)
	if err != nil {
		return err
	}
	if slave_reply.DataStat != DATA_PLEASE {
		return slave_reply.Error
	}
	slave_reply, err = z.sendAndDecodeSlaveReceipt(cprocess, role_context_b)
	if err != nil {
		return err
	}
	if slave_reply.DataStat != DATA_ALL_OK {
		return slave_reply.Error
	} else {
		return nil
	}
}

// 清除一个上下文，也就是删除
func (z *ZrStorage) DropContext(id, contextname string) (err error) {
	// 如果启动了缓存，则全局读锁
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
	}
	// 是否为本地
	mode, conn := z.findConn(id)
	if mode == CONN_IS_LOCAL {
		// 本地
		rolec, err := z.readRole_small(id)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]DropContext: %v", err)
			return err
		}
		rolec.lock.Lock()
		defer rolec.lock.Unlock()
		rolec.role.DelContext(contextname)
		return nil
	} else {
		// slave
		err = z.dropContext(id, contextname, conn)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]DropContext: %v", err)
		}
		return err
	}
}

// 让slave清除上下文
func (z *ZrStorage) dropContext(id, contextname string, conns []*slaveIn) (err error) {
	// 构造要发送的信息
	role_context := Net_RoleAndContext{
		Id:      id,
		Context: contextname,
	}
	role_context_b, err := nst.StructGobBytes(role_context)
	if err != nil {
		return err
	}
	// 遍历slave
	var errstr string
	for _, onec := range conns {
		err = z.dropContext_one(role_context_b, onec)
		if err != nil {
			errstr += fmt.Sprint(onec.name, ": ", err, " | ")
		}
	}
	if len(errstr) != 0 {
		return fmt.Errorf(errstr)
	} else {
		return nil
	}
}

// 向某一个slave发送drop上下文的请求
//
//	--> OPERATE_DROP_CONTEXT (前导)
//	<-- DATA_PLEASE (slave回执)
//	--> Net_RoleAndContext (结构体)
//	<-- DATA_ALL_OK (slave回执)
func (z *ZrStorage) dropContext_one(role_context_b []byte, onec *slaveIn) (err error) {
	// 分配连接
	cprocess := onec.tcpconn.OpenProgress()
	defer cprocess.Close()
	// 发送前导
	slave_reply, err := z.sendPrefixStat(cprocess, onec.code, OPERATE_DROP_CONTEXT)
	if err != nil {
		return err
	}
	if slave_reply.DataStat != DATA_PLEASE {
		return slave_reply.Error
	}
	slave_reply, err = z.sendAndDecodeSlaveReceipt(cprocess, role_context_b)
	if err != nil {
		return err
	}
	if slave_reply.DataStat != DATA_ALL_OK {
		return slave_reply.Error
	} else {
		return nil
	}
}

// 返回某个上下文的全部信息，如果没有这个上下文则have返回false
func (z *ZrStorage) ReadContext(id, contextname string) (context roles.Context, have bool, err error) {
	// 如果启动了缓存
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
	}
	// 是否本地
	mode, conn := z.findConn(id)
	if mode == CONN_IS_LOCAL {
		// 本地
		rolec, err := z.readRole_small(id)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]ReadContext: %v", err)
			return context, false, err
		}
		// 加锁
		rolec.lock.RLock()
		defer rolec.lock.RUnlock()
		// 读取
		context, have = rolec.role.GetContext(contextname)
		return context, have, nil
	} else {
		// slave，随即获取一个连接
		conn_count := len(conn)
		conn_random := random.GetRandNum(conn_count - 1)
		context, have, err = z.readContext(id, contextname, conn[conn_random])
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]ReadContext: %v", err)
		}
		return context, have, err
	}
}

// slave上的readContext
//
//	--> OPERATE_READ_CONTEXT (前导)
//	<-- DATA_PLEASE (slave回执)
//	--> Net_RoleAndContext (结构体)
//	<-- context (roles.Context，Net_SlaveReceipt_Data封装)
func (z *ZrStorage) readContext(id, contextname string, conn *slaveIn) (context roles.Context, have bool, err error) {
	have = false
	// 分配连接
	cprocess := conn.tcpconn.OpenProgress()
	defer cprocess.Close()
	// 前导
	slave_reply, err := z.sendPrefixStat(cprocess, conn.code, OPERATE_READ_CONTEXT)
	if err != nil {
		return
	}
	if slave_reply.DataStat != DATA_PLEASE {
		err = slave_reply.Error
		return
	}
	// 构造要发送的结构体
	role_context := Net_RoleAndContext{
		Id:      id,
		Context: contextname,
	}
	role_context_b, err := nst.StructGobBytes(role_context)
	if err != nil {
		return
	}
	slave_reply_data, err := z.sendAndDecodeSlaveReceiptData(cprocess, role_context_b)
	if err != nil {
		return
	}
	// 看看slave是没有找到还是其他错误
	if slave_reply_data.DataStat != DATA_ALL_OK {
		if slave_reply_data.DataStat == DATA_RETURN_IS_FALSE {
			return context, false, nil
		} else {
			return context, false, slave_reply_data.Error
		}
	}
	err = nst.BytesGobStruct(slave_reply_data.Data, &context)
	if err != nil {
		return
	}
	return context, true, nil
}

// 清除一个上下文的绑定，upordown为roles包中的CONTEXT_UP或CONTEXT_DOWN，binderole是绑定的角色id
func (z *ZrStorage) DeleteContextBind(id, contextname string, upordown uint8, bindrole string) (err error) {
	// 是否有缓存
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
	}
	// 查看本地否
	mode, conn := z.findConn(id)
	if mode == CONN_IS_LOCAL {
		// 如果在本地，就读出来
		rolec, err := z.readRole_small(id)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]DeleteContextBind: %v", err)
			return err
		}
		// 锁定角色
		rolec.lock.Lock()
		defer rolec.lock.Unlock()
		// 调用角色接口
		if upordown == roles.CONTEXT_UP {
			rolec.role.DelContextUp(contextname, bindrole)
		} else if upordown == roles.CONTEXT_DOWN {
			rolec.role.DelContextDown(contextname, bindrole)
		} else {
			return fmt.Errorf("drcm[ZrStorage]DeleteContextBind: Must CONTEXT_UP or CONTEXT_DOWN.")
		}
		return nil
	} else {
		// 如果slave
		err = z.delContextBind(id, contextname, upordown, bindrole, conn)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]DeleteContextBind: %v", err)
		}
		return err
	}
}

// slave清除一个上下文的绑定
func (z *ZrStorage) delContextBind(id, contextname string, upordown uint8, bindrole string, conns []*slaveIn) (err error) {
	// 构造传输的信息
	role_context := Net_RoleAndContext{
		Id:       id,
		Context:  contextname,
		UpOrDown: upordown,
		BindRole: bindrole,
	}
	role_context_b, err := nst.StructGobBytes(role_context)
	if err != nil {
		return err
	}
	// 遍历连接
	var errstr string
	for _, onec := range conns {
		err = z.delContextBind_one(role_context_b, onec)
		if err != nil {
			errstr += fmt.Sprint(onec.name, ": ", err, " | ")
		}
	}
	if len(errstr) != 0 {
		return fmt.Errorf(errstr)
	}
	return nil
}

// 一个的slave清除一个上下文的绑定
//
//	--> OPERATE_DEL_CONTEXT_BIND (前导)
//	<-- DATA_PLEASE (slave回执)
//	--> Net_RoleAndContext ([]byte)
//	<-- DATA_ALL_OK (slave回执)
func (z *ZrStorage) delContextBind_one(role_context_b []byte, onec *slaveIn) (err error) {
	// 分配连接
	cprocess := onec.tcpconn.OpenProgress()
	defer cprocess.Close()
	// 前导
	slave_reply, err := z.sendPrefixStat(cprocess, onec.code, OPERATE_DEL_CONTEXT_BIND)
	if err != nil {
		return err
	}
	if slave_reply.DataStat != DATA_PLEASE {
		return slave_reply.Error
	}
	// 发送数据
	slave_reply, err = z.sendAndDecodeSlaveReceipt(cprocess, role_context_b)
	if err != nil {
		return err
	}
	if slave_reply.DataStat != DATA_ALL_OK {
		return slave_reply.Error
	}
	return nil
}

// 返回某个上下文中的同样绑定值的所有，upordown为roles中的CONTEXT_UP或CONTEXT_DOWN，如果给定的contextname不存在，则have返回false。
func (z *ZrStorage) ReadContextSameBind(id, contextname string, upordown uint8, bind int64) (rolesid []string, have bool, err error) {
	// 如果启动了缓存
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
	}
	// 是否为本地
	mode, conn := z.findConn(id)
	if mode == CONN_IS_LOCAL {
		// 本地的解决方案
		rolec, err := z.readRole_small(id)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]ReadContextSameBind: %v", err)
			return nil, false, err
		}
		// 加读锁
		rolec.lock.RLock()
		defer rolec.lock.RUnlock()
		// 从角色获得信息
		if upordown == roles.CONTEXT_UP {
			rolesid, have = rolec.role.GetContextUpSameBind(contextname, bind)
		} else {
			rolesid, have = rolec.role.GetContextDownSameBind(contextname, bind)
		}
		return rolesid, have, nil
	} else {
		// slave的解决方案
		conn_count := len(conn)
		conn_random := random.GetRandNum(conn_count - 1)
		rolesid, have, err = z.readContextSameBind(id, contextname, upordown, bind, conn[conn_random])
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]ReadContextSameBind: %v", err)
		}
		return rolesid, have, err
	}
}

// 从一个slave读取某个上下文中的同样绑定值的所有
//
//	--> OPERATE_SAME_BIND_CONTEXT (前导)
//	<-- DATA_PLEASE (slave 回执)
//	--> Net_RoleAndContext_Data (结构体)
//	<-- rolesid []string ([]byte数据，Net_SlaveReceipt_Data封装)
func (z *ZrStorage) readContextSameBind(id, contextname string, upordown uint8, bind int64, conn *slaveIn) (rolesid []string, have bool, err error) {
	// 构造发出的信息
	contextsamebind := Net_RoleAndContext_Data{
		Id:       id,
		Context:  contextname,
		UpOrDown: upordown,
		Single:   1,
		Bit:      0,
		Int:      bind,
	}
	contextsamebind_b, err := nst.StructGobBytes(contextsamebind)
	if err != nil {
		return nil, false, err
	}
	// 分配连接
	cprocess := conn.tcpconn.OpenProgress()
	defer cprocess.Close()
	// 发送前导
	slave_receipt, err := z.sendPrefixStat(cprocess, conn.code, OPERATE_SAME_BIND_CONTEXT)
	if err != nil {
		return nil, false, err
	}
	// 查看回执
	if slave_receipt.DataStat != DATA_PLEASE {
		return nil, false, slave_receipt.Error
	}
	// 发送结构
	slave_receipt_data, err := z.sendAndDecodeSlaveReceiptData(cprocess, contextsamebind_b)
	// 查看回执
	if slave_receipt_data.DataStat == DATA_RETURN_IS_FALSE {
		// 这是如果没有找到的解决方法
		return nil, false, slave_receipt_data.Error
	}
	if slave_receipt_data.DataStat != DATA_ALL_OK {
		// 这是不期望的发送
		return nil, false, slave_receipt_data.Error
	}
	rolesid = make([]string, 0)
	err = nst.BytesGobStruct(slave_receipt_data.Data, &rolesid)
	if err != nil {
		return nil, false, err
	}
	return rolesid, true, nil
}

// 返回所有上下文组的名称
func (z *ZrStorage) ReadContextsName(id string) (names []string, err error) {
	// 如果启用了缓存，则启用全局读锁
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
	}
	// 是否为本地
	mode, conn := z.findConn(id)
	if mode == CONN_IS_LOCAL {
		// 本地
		rolec, err := z.readRole_small(id)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]ReadContextsName: %v", err)
			return nil, err
		}
		// 角色加读锁
		rolec.lock.RLock()
		defer rolec.lock.RUnlock()
		// 角色的接口
		names = rolec.role.GetContextsName()
		return names, nil
	} else {
		// slave
		conn_count := len(conn)
		conn_random := random.GetRandNum(conn_count - 1)
		names, err = z.readContextsName(id, conn[conn_random])
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]ReadContextsName: %v", err)
		}
		return names, err
	}
}

// slave上的返回所有上下文组的名称
//
//	--> OPERATE_GET_CONTEXTS_NAME (前导)
//	<-- DATA_PLEASE (slave回执)
//	--> role's id
//	<-- names (slave回执带数据体)
func (z *ZrStorage) readContextsName(id string, conn *slaveIn) (names []string, err error) {
	// 分配连接
	cprocess := conn.tcpconn.OpenProgress()
	defer cprocess.Close()
	// 发送前导
	slave_reply, err := z.sendPrefixStat(cprocess, conn.code, OPERATE_GET_CONTEXTS_NAME)
	if err != nil {
		return
	}
	if slave_reply.DataStat != DATA_PLEASE {
		return nil, slave_reply.Error
	}
	// 发送id，并接收带数据体的slave回执
	slave_reply_data, err := z.sendAndDecodeSlaveReceiptData(cprocess, []byte(id))
	if err != nil {
		return
	}
	if slave_reply_data.DataStat != DATA_ALL_OK {
		return nil, slave_reply_data.Error
	}
	names = make([]string, 0)
	err = nst.BytesGobStruct(slave_reply_data.Data, &names)
	return
}

// 设置朋友的状态属性
func (z *ZrStorage) WriteFriendStatus(id, friend string, bindbit int, value interface{}) (err error) {
	// 如果开启缓存
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
	}
	// 检查是否为本地
	mode, conn := z.findConn(id)
	if mode == CONN_IS_LOCAL {
		// 如果是本地
		rolec, err := z.readRole_small(id)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]WriteFriendStatus: %v", err)
			return err
		}
		// 给role加锁
		rolec.lock.Lock()
		defer rolec.lock.Unlock()
		// 设定状态
		err = rolec.role.SetFriendStatus(friend, bindbit, value)
		if err != nil {
			return err
		}
		err = z.checkCacheStore(rolec.role)
		return err
	} else {
		// 如果是slave
		err = z.writeFriendStatus(conn, id, friend, bindbit, value)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]WriteFriendStatus: %v", err)
		}
		return err
	}
}

// slave的设置朋友的状态属性
func (z *ZrStorage) writeFriendStatus(conns []*slaveIn, id, friend string, bindbit int, value interface{}) (err error) {
	// 构建要发送的信息
	statustype := z.statusValueType(value)
	if statustype == 0 {
		return fmt.Errorf("The value's type not int64, float64 or complex128.")
	}
	role_friend := Net_RoleAndFriend{
		Id:     id,
		Friend: friend,
		Single: statustype,
		Bit:    bindbit,
	}
	switch statustype {
	case roles.STATUS_VALUE_TYPE_INT:
		role_friend.Int = value.(int64)
	case roles.STATUS_VALUE_TYPE_FLOAT:
		role_friend.Float = value.(float64)
	case roles.STATUS_VALUE_TYPE_COMPLEX:
		role_friend.Complex = value.(complex128)
	default:
		role_friend.Single = roles.STATUS_VALUE_TYPE_NULL
	}
	role_friend_b, err := nst.StructGobBytes(role_friend)
	if err != nil {
		return nil
	}
	// 遍历连接
	var errstr string
	for _, onec := range conns {
		err = z.writeFriendStatus_one(role_friend_b, onec)
		if err != nil {
			errstr += fmt.Sprint(onec.name, ": ", err, " | ")
		}
	}
	if len(errstr) != 0 {
		return fmt.Errorf(errstr)
	} else {
		return nil
	}
}

// 一个slave的设置朋友的状态属性
// 	--> OPERATE_SET_FRIEND_STATUS (前导词)
//	<-- DATA_PLEASE (slave回执)
//	--> Net_RoleAndFriend (结构体)
//	<-- DATA_ALL_OK (slave回执)
func (z *ZrStorage) writeFriendStatus_one(role_friend_b []byte, onec *slaveIn) (err error) {
	// 分配连接
	cprocess := onec.tcpconn.OpenProgress()
	defer cprocess.Close()
	// 发送前导
	slave_reply, err := z.sendPrefixStat(cprocess, onec.code, OPERATE_SET_FRIEND_STATUS)
	if err != nil {
		return err
	}
	if slave_reply.DataStat != DATA_PLEASE {
		return slave_reply.Error
	}
	slave_reply, err = z.sendAndDecodeSlaveReceipt(cprocess, role_friend_b)
	if err != nil {
		return err
	}
	if slave_reply.DataStat != DATA_ALL_OK {
		return slave_reply.Error
	}
	return nil
}

// 获取朋友的状态属性
func (z *ZrStorage) ReadFriendStatus(id, friend string, bindbit int, value interface{}) (err error) {
	// 如果启动了缓存
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
	}
	// 是否为本地
	mode, conn := z.findConn(id)
	if mode == CONN_IS_LOCAL {
		// 本地
		rolec, err := z.readRole_small(id)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]ReadFriendStatus: %v", err)
			return err
		}
		// 加读锁
		rolec.lock.RLock()
		defer rolec.lock.RUnlock()
		// 获取
		err = rolec.role.GetFriendStatus(friend, bindbit, value)
		return err
	} else {
		// slave
		conn_count := len(conn)
		conn_random := random.GetRandNum(conn_count - 1)
		err = z.readFriendStatus(conn[conn_random], id, friend, bindbit, value)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]ReadFriendStatus: %v", err)
		}
		return err
	}
}

// slave获取朋友的状态属性
//
//	--> OPERATE_GET_FRIEND_STATUS (前导)
//	<-- DATA_PLEASE (slave回执)
//	--> Net_RoleAndFriend (结构体)
//	<-- Net_RoleAndFriend带上value (slave回执带数据体)
func (z *ZrStorage) readFriendStatus(conn *slaveIn, id, friend string, bindbit int, value interface{}) (err error) {
	// 看看要什么类型的值
	valuetype := z.statusValueType(value)
	if valuetype == roles.STATUS_VALUE_TYPE_NULL {
		return fmt.Errorf("The value's type not int64, float64 or complex128.")
	}
	// 构造查询结构
	role_friend := Net_RoleAndFriend{
		Id:     id,
		Friend: friend,
		Single: valuetype,
		Bit:    bindbit,
	}
	role_friend_b, err := nst.StructGobBytes(role_friend)
	if err != nil {
		return err
	}
	// 分配连接
	cprocess := conn.tcpconn.OpenProgress()
	defer cprocess.Close()
	// 发送前导
	slave_reply, err := z.sendPrefixStat(cprocess, conn.code, OPERATE_GET_FRIEND_STATUS)
	if err != nil {
		return err
	}
	if slave_reply.DataStat != DATA_PLEASE {
		return slave_reply.Error
	}
	// 发送要查询的结构，并接收带数据体的slave回执
	slave_reply_data, err := z.sendAndDecodeSlaveReceiptData(cprocess, role_friend_b)
	if err != nil {
		return err
	}
	if slave_reply_data.DataStat != DATA_ALL_OK {
		return slave_reply_data.Error
	}
	err = nst.BytesGobStruct(slave_reply_data.Data, &role_friend)
	if err != nil {
		return err
	}
	switch role_friend.Single {
	case roles.STATUS_VALUE_TYPE_INT:
		value = &role_friend.Int
	case roles.STATUS_VALUE_TYPE_FLOAT:
		value = &role_friend.Float
	case roles.STATUS_VALUE_TYPE_COMPLEX:
		value = &role_friend.Complex
	default:
		return fmt.Errorf("The value's type not int64, float64 or complex128.")
	}
	return nil
}

// 设定上下文的状态属性，upordown为roles中的CONTEXT_UP或CONTEXT_DOWN
func (z *ZrStorage) WriteContextStatus(id, contextname string, upordown uint8, bindroleid string, bindbit int, value interface{}) (err error) {
	// 如果开启缓存
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
	}
	// 检查是否为本地
	mode, conn := z.findConn(id)
	if mode == CONN_IS_LOCAL {
		// 如果是本地
		rolec, err := z.readRole_small(id)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]WriteContextStatus: %v", err)
			return err
		}
		// 加锁
		rolec.lock.Lock()
		defer rolec.lock.Unlock()
		// 设定状态
		err = rolec.role.SetContextStatus(contextname, upordown, bindroleid, bindbit, value)
		if err != nil {
			return err
		}
		err = z.checkCacheStore(rolec.role)
		return err
	} else {
		// 如果是slave
		err = z.writeContextStatus(conn, id, contextname, upordown, bindroleid, bindbit, value)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]WriteContextStatus: %v", err)
		}
		return err
	}
}

// slave的设定上下文的状态属性
func (z *ZrStorage) writeContextStatus(conns []*slaveIn, id, contextname string, upordown uint8, bindroleid string, bindbit int, value interface{}) (err error) {
	// 构建要发送的信息
	statustype := z.statusValueType(value)
	if statustype == 0 {
		return fmt.Errorf("The value's type not int64, float64 or complex128.")
	}
	role_context := Net_RoleAndContext_Data{
		Id:       id,
		Context:  contextname,
		UpOrDown: upordown,
		BindRole: bindroleid,
		Single:   statustype,
		Bit:      bindbit,
	}
	switch statustype {
	case roles.STATUS_VALUE_TYPE_INT:
		role_context.Int = value.(int64)
	case roles.STATUS_VALUE_TYPE_FLOAT:
		role_context.Float = value.(float64)
	case roles.STATUS_VALUE_TYPE_COMPLEX:
		role_context.Complex = value.(complex128)
	}
	role_context_b, err := nst.StructGobBytes(role_context)
	if err != nil {
		return nil
	}
	// 遍历连接
	var errstr string
	for _, onec := range conns {
		err = z.writeContextStatus_one(role_context_b, onec)
		if err != nil {
			errstr += fmt.Sprint(onec.name, ": ", err, " | ")
		}
	}
	if len(errstr) != 0 {
		return fmt.Errorf(errstr)
	} else {
		return nil
	}
}

// 一个slave的设定上下文的状态属性
//
//	--> OPERATE_SET_CONTEXT_STATUS (前导)
//	<-- DATA_PLEASE (slave回执)
//	--> Net_RoleAndContext_Data (结构体)
//	<-- DATA_ALL_OK (slave回执)
func (z *ZrStorage) writeContextStatus_one(role_context_b []byte, onec *slaveIn) (err error) {
	// 分配连接
	cprocess := onec.tcpconn.OpenProgress()
	defer cprocess.Close()
	// 发送前导
	slave_reply, err := z.sendPrefixStat(cprocess, onec.code, OPERATE_SET_CONTEXT_STATUS)
	if err != nil {
		return err
	}
	if slave_reply.DataStat != DATA_PLEASE {
		return slave_reply.Error
	}
	slave_reply, err = z.sendAndDecodeSlaveReceipt(cprocess, role_context_b)
	if err != nil {
		return err
	}
	if slave_reply.DataStat != DATA_ALL_OK {
		return slave_reply.Error
	}
	return nil
}

// 获取上下文的状态属性，upordown为roles.CONTEXT_UP或roles.CONTEXT_DOWN
func (z *ZrStorage) ReadContextStatus(id, contextname string, upordown uint8, bindroleid string, bindbit int, value interface{}) (err error) {
	// 如果启动了缓存
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
	}
	// 是否为本地
	mode, conn := z.findConn(id)
	if mode == CONN_IS_LOCAL {
		// 本地
		rolec, err := z.readRole_small(id)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]ReadContextStatus: %v", err)
			return err
		}
		// 加读锁
		rolec.lock.RLock()
		defer rolec.lock.RUnlock()
		// 获取
		err = rolec.role.GetContextStatus(contextname, upordown, bindroleid, bindbit, value)
		return err
	} else {
		// slave
		conn_count := len(conn)
		conn_random := random.GetRandNum(conn_count - 1)
		err = z.readContextStatus(conn[conn_random], id, contextname, upordown, bindroleid, bindbit, value)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]ReadContextStatus: %v", err)
		}
		return err
	}
}

// slave获取上下文的状态属性
//
//	--> OPERATE_GET_CONTEXT_STATUS (前导)
//	<-- DATA_PLEASE (slave回执)
//	--> Net_RoleAndContext_Data (结构体)
//	<-- value (slave回执带数据体)
func (z *ZrStorage) readContextStatus(conn *slaveIn, id, contextname string, upordown uint8, bindroleid string, bindbit int, value interface{}) (err error) {
	// 看看要什么类型的值
	valuetype := z.statusValueType(value)
	if valuetype == roles.STATUS_VALUE_TYPE_NULL {
		return fmt.Errorf("The value's type not int64, float64 or complex128.")
	}
	// 构造查询结构
	role_context := Net_RoleAndContext_Data{
		Id:       id,
		Context:  contextname,
		UpOrDown: upordown,
		BindRole: bindroleid,
		Single:   valuetype,
		Bit:      bindbit,
	}
	role_context_b, err := nst.StructGobBytes(role_context)
	if err != nil {
		return err
	}
	// 分配连接
	cprocess := conn.tcpconn.OpenProgress()
	defer cprocess.Close()
	// 发送前导
	slave_reply, err := z.sendPrefixStat(cprocess, conn.code, OPERATE_GET_CONTEXT_STATUS)
	if err != nil {
		return err
	}
	if slave_reply.DataStat != DATA_PLEASE {
		return slave_reply.Error
	}
	// 发送要查询的结构，并接收带数据体的slave回执
	slave_reply_data, err := z.sendAndDecodeSlaveReceiptData(cprocess, role_context_b)
	if err != nil {
		return err
	}
	if slave_reply_data.DataStat != DATA_ALL_OK {
		return slave_reply_data.Error
	}
	err = nst.BytesGobStruct(slave_reply_data.Data, value)
	return err
}

// 设定上下文
func (z *ZrStorage) WriteContexts(id string, contexts map[string]roles.Context) (err error) {
	// 如果启动了缓存
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
	}
	// 是否为本地
	mode, conn := z.findConn(id)
	if mode == CONN_IS_LOCAL {
		// 如果为本地
		rolec, err := z.readRole_small(id)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]WriteContexts: %v", err)
			return err
		}
		// 加锁
		rolec.lock.Lock()
		defer rolec.lock.Unlock()
		// 调用角色的接口
		rolec.role.SetContexts(contexts)
		err = z.checkCacheStore(rolec.role)
		return err
	} else {
		// 如果是slave
		err = z.writeContexts(id, contexts, conn)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]WriteContexts: %v", err)
		}
		return err
	}
}

// 在slave上设定上下文
func (z *ZrStorage) writeContexts(id string, contexts map[string]roles.Context, conns []*slaveIn) (err error) {
	// 构建要传输的信息
	contexts_b, err := nst.StructGobBytes(contexts)
	if err != nil {
		return err
	}
	// 遍历链接
	var errstr string
	for _, onec := range conns {
		err = z.writeContexts_one(id, contexts_b, onec)
		if err != nil {
			errstr += fmt.Sprint(onec.name, ": ", err, " | ")
		}
	}
	if len(errstr) != 0 {
		return fmt.Errorf(errstr)
	} else {
		return nil
	}
}

// 在slave上设定上下文_一个
//
//	--> OPERATE_SET_CONTEXTS (前导)
//	<-- DATA_PLEASE (slave回执)
//	--> role's id
//	<-- DATA_PLEASE (slave回执)
//	--> map[string]roles.Context ([]byte)
//	<-- DATA_ALL_OK (slave回执)
func (z *ZrStorage) writeContexts_one(id string, contexts_b []byte, onec *slaveIn) (err error) {
	// 分配连接进程
	cprocess := onec.tcpconn.OpenProgress()
	defer cprocess.Close()
	// 发送前导
	slave_receipt, err := z.sendPrefixStat(cprocess, onec.code, OPERATE_SET_CONTEXTS)
	if err != nil {
		return err
	}
	if slave_receipt.DataStat != DATA_PLEASE {
		return slave_receipt.Error
	}
	// 发送角色的id
	slave_receipt, err = z.sendAndDecodeSlaveReceipt(cprocess, []byte(id))
	if err != nil {
		return err
	}
	if slave_receipt.DataStat != DATA_PLEASE {
		return slave_receipt.Error
	}
	// 发送数据
	slave_receipt, err = z.sendAndDecodeSlaveReceipt(cprocess, contexts_b)
	if err != nil {
		return err
	}
	if slave_receipt.DataStat != DATA_ALL_OK {
		return slave_receipt.Error
	}
	return nil
}

// 获取上下文
func (z *ZrStorage) ReadContexts(id string) (contexts map[string]roles.Context, err error) {
	// 如果启动了缓存
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
	}
	// 查看是否在本地
	connmode, conn := z.findConn(id)
	if connmode == CONN_IS_LOCAL {
		// 本地
		rolec, err := z.readRole_small(id)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]ReadContexts: %v", err)
			return contexts, err
		}
		// 加读锁
		rolec.lock.RLock()
		defer rolec.lock.RUnlock()
		// 从角色获取信息
		contexts = rolec.role.GetContexts()
		return contexts, nil
	} else {
		// 如果slave
		conn_count := len(conn)
		conn_random := random.GetRandNum(conn_count - 1)
		contexts, err = z.readContexts(id, conn[conn_random])
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]ReadContexts: %v", err)
		}
		return contexts, err
	}
}

// Slave的获取上下文
//
//	分配连接进程
//	--> OPERATE_GET_CONTEXTS (前导)
//	<-- DATA_PLEASE (slave回执)
//	--> role's id (角色ID)
//	<-- contexts (byte，Net_SlaveReceipt_Data封装)
func (z *ZrStorage) readContexts(id string, conn *slaveIn) (contexts map[string]roles.Context, err error) {
	// 分配连接
	cprocess := conn.tcpconn.OpenProgress()
	defer cprocess.Close()
	// 发送前导词
	sr, err := z.sendPrefixStat(cprocess, conn.code, OPERATE_GET_CONTEXTS)
	if err != nil {
		return
	}
	if sr.DataStat != DATA_PLEASE {
		return nil, sr.Error

	}
	// 发送角色ID
	sr_data, err := z.sendAndDecodeSlaveReceiptData(cprocess, []byte(id))
	if err != nil {
		return nil, err
	}
	if sr_data.DataStat != DATA_ALL_OK {
		return nil, sr_data.Error
	}
	// 解码
	contexts = make(map[string]roles.Context)
	err = nst.BytesGobStruct(sr_data.Data, &contexts)
	return contexts, err
}

// 重置上下文，实际也就是利用WriteContexts发一个空的过去
func (z *ZrStorage) ResetContexts(id string) (err error) {
	contexts := make(map[string]roles.Context)
	return z.WriteContexts(id, contexts)
}

// 把data的数据装入role的name值下，如果找不到name，则返回错误
func (z *ZrStorage) WriteData(id, name string, data interface{}) (err error) {
	// 如果开启缓存
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
	}
	// 是否为本地
	mode, conn := z.findConn(id)
	if mode == CONN_IS_LOCAL {
		// 如果本地
		rolec, err := z.readRole_small(id)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]WriteData: %v", err)
			return err
		}
		// 加锁
		rolec.lock.Lock()
		defer rolec.lock.Unlock()
		defer func() {
			// 拦截反射的恐慌
			if e := recover(); e != nil {
				err = fmt.Errorf("drcm[ZrStorage]WriteData: %v", e)
			}
		}()
		// 开始反射的那些乱七八遭
		rv := reflect.Indirect(reflect.ValueOf(rolec.role)).FieldByName(name)
		rv_type := rv.Type()
		dv := reflect.Indirect(reflect.ValueOf(data))
		dv_type := dv.Type()
		if rv_type != dv_type {
			err = fmt.Errorf("drcm[ZrStorage]WriteData: The data type %v not assignable to type %v.", dv_type, rv_type)
			return err
		}
		if rv.CanSet() != true {
			err = fmt.Errorf("drcm[ZrStroage]WriteData: The data type %v not be set.", dv_type)
			return err
		}
		rv.Set(dv)
		rolec.role.SetDataChanged()
		err = z.checkCacheStore(rolec.role)
		return err
	} else {
		// 如果是在slave
		err = z.writeData(id, name, data, conn)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]WriteData: %v", err)
		}
		return err
	}
}

// slave把data的数据装入role的name值下
func (z *ZrStorage) writeData(id, name string, data interface{}, conns []*slaveIn) (err error) {
	// 构造要传输的信息
	data_b, err := nst.StructGobBytes(data)
	if err != nil {
		return err
	}
	trans := Net_RoleData_Data{
		Id:   id,
		Name: name,
		Data: data_b,
	}
	trans_b, err := nst.StructGobBytes(trans)
	if err != nil {
		return err
	}
	// 遍历连接
	var errstr string
	for _, onec := range conns {
		err = z.writeData_one(id, trans_b, onec)
		if err != nil {
			errstr += fmt.Sprint(onec.name, ": ", err, " | ")
		}
	}
	if len(errstr) != 0 {
		return fmt.Errorf(errstr)
	} else {
		return nil
	}
}

// slave把data的数据装入role的name值下，一个的
//
//	--> OPERATE_SET_DATA (前导)
//	<-- DATA_PLEASE (slave回执)
//	--> Net_RoleData_Data
//	<-- DATA_ALL_OK (slave回执)
func (z *ZrStorage) writeData_one(id string, trans_b []byte, onec *slaveIn) (err error) {
	// 分配连接
	cprocess := onec.tcpconn.OpenProgress()
	defer cprocess.Close()
	// 发送前导
	slave_receipt, err := z.sendPrefixStat(cprocess, onec.code, OPERATE_SET_DATA)
	if err != nil {
		return err
	}
	if slave_receipt.DataStat != DATA_PLEASE {
		return slave_receipt.Error
	}
	//发送数据体
	slave_receipt, err = z.sendAndDecodeSlaveReceipt(cprocess, trans_b)
	if err != nil {
		return err
	}
	if slave_receipt.DataStat != DATA_ALL_OK {
		return slave_receipt.Error
	}
	return nil
}

// 从角色中知道name的数据名并返回其数据
func (z *ZrStorage) ReadData(id, name string, data interface{}) (err error) {
	// 如果启用缓存
	if z.cacheMax >= 0 {
		z.lock.RLock()
		defer z.lock.RUnlock()
	}
	// 是否本地
	mode, conn := z.findConn(id)
	if mode == CONN_IS_LOCAL {
		// 本地
		rolec, err := z.readRole_small(id)
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]ReadData: %v", err)
			return err
		}
		rolec.lock.RLock()
		defer rolec.lock.RUnlock()
		// 开始获取信息了
		rv := reflect.Indirect(reflect.ValueOf(rolec.role)).FieldByName(name)
		rv_type := rv.Type()
		dv := reflect.Indirect(reflect.ValueOf(data))
		dv_type := dv.Type()
		if rv_type != dv_type {
			err = fmt.Errorf("drcm[ZrStorage]WriteData: The data type %v not assignable to type %v.", dv_type, rv_type)
			return err
		}
		if rv.CanSet() != true {
			err = fmt.Errorf("drcm[ZrStroage]WriteData: The data type %v not be set.", dv_type)
			return err
		}
		dv.Set(rv)
		return nil
	} else {
		// slave
		conn_count := len(conn)
		conn_random := random.GetRandNum(conn_count - 1)
		err = z.readData(id, name, data, conn[conn_random])
		if err != nil {
			err = fmt.Errorf("drcm[ZrStorage]ReadData: %v", err)
		}
		return err
	}
}

// slave的从角色中知道name的数据名并返回其数据
//
//	--> OPERATE_GET_DATA (前导词)
//	<-- DATA_PLEASE (slave回执)
//	--> Net_RoleData_Data
//	<-- Net_RoleData_Data (跟随DATA_ALL_OK)
func (z *ZrStorage) readData(id, name string, data interface{}, conn *slaveIn) (err error) {
	// 分配连接
	cprocess := conn.tcpconn.OpenProgress()
	defer cprocess.Close()
	// 构建发送的数据
	trans := Net_RoleData_Data{
		Id:   id,
		Name: name,
	}
	trans_b, err := nst.StructGobBytes(trans)
	if err != nil {
		return err
	}
	// 发送前导
	slave_receipt, err := z.sendPrefixStat(cprocess, conn.code, OPERATE_GET_DATA)
	if err != nil {
		return err
	}
	if slave_receipt.DataStat != DATA_PLEASE {
		return slave_receipt.Error
	}
	// 发送数据体并接收
	slave_receipt_data, err := z.sendAndDecodeSlaveReceiptData(cprocess, trans_b)
	if err != nil {
		return err
	}
	if slave_receipt_data.DataStat != DATA_ALL_OK {
		return slave_receipt_data.Error
	}
	err = nst.BytesGobStruct(slave_receipt_data.Data, data)
	return err
}

// 查看连接是哪个，id为角色的id，connmode来自CONN_IS_*
func (z *ZrStorage) findConn(id string) (connmode uint8, conn []*slaveIn) {
	// 如果模式为own，则直接返回本地
	if z.dmode == DMODE_OWN {
		connmode = CONN_IS_LOCAL
		return
	}

	// 找到第一个首字母。
	theChar := string(id[0])
	// slave池中有没有
	conn, find := z.slaves[theChar]
	if find == false {
		// 如果在slave池里没有找到，那么就默认为本地存储
		connmode = CONN_IS_LOCAL
		return
	} else {
		connmode = CONN_IS_SLAVE
		return
	}
}

// 从[]byte解码SlaveReceipt
func (z *ZrStorage) decodeSlaveReceipt(b []byte) (receipt Net_SlaveReceipt, err error) {
	return DecodeSlaveReceipt(b)
}

// 从[]byte解码SlaveReceipt带数据体
func (z *ZrStorage) decodeSlaveReceiptData(b []byte) (receipt Net_SlaveReceipt_Data, err error) {
	return DecodeSlaveReceiptData(b)
}

// 发送数据并解码返回的SlaveReceipt
func (z *ZrStorage) sendAndDecodeSlaveReceipt(cprocess *nst.ProgressData, data []byte) (receipt Net_SlaveReceipt, err error) {
	return SendAndDecodeSlaveReceipt(cprocess, data)
}

// 发送数据并解码返回的SlaveReceipt_Data
func (z *ZrStorage) sendAndDecodeSlaveReceiptData(cprocess *nst.ProgressData, data []byte) (receipt Net_SlaveReceipt_Data, err error) {
	return SendAndDecodeSlaveReceiptData(cprocess, data)
}

// 向slave发送前导状态，也就是身份验证码和要操作的状态，并获取slave是否可以继续传输的要求
func (z *ZrStorage) sendPrefixStat(process *nst.ProgressData, code string, operate int) (receipt Net_SlaveReceipt, err error) {
	return SendPrefixStat(process, code, operate)
}

// 查看是否被标记删除，标记删除则返回true。
func (z *ZrStorage) checkDel(id string) bool {
	del := z.checkDelById(id)
	if del == true {
		return del
	}
	rolec, find := z.rolesCache[id]
	if find == false {
		return true
	} else {
		del = rolec.role.ReturnDelete()
		return del
	}
}

func (z *ZrStorage) checkDelById(id string) bool {
	for _, v := range z.deleteCache {
		if v == id {
			return true
		}
	}
	return false
}

// 判断friend或context的状态的类型，types：1为int，2为float，3为complex
func (z *ZrStorage) statusValueType(value interface{}) (types uint8) {
	valuer := reflect.Indirect(reflect.ValueOf(value))
	vname := valuer.Type().String()
	switch vname {
	case "int64":
		return roles.STATUS_VALUE_TYPE_INT
	case "float64":
		return roles.STATUS_VALUE_TYPE_FLOAT
	case "complex128":
		return roles.STATUS_VALUE_TYPE_COMPLEX
	default:
		return roles.STATUS_VALUE_TYPE_NULL
	}
}

// 检查如果没有开启缓存，那就直接进行保存
func (z *ZrStorage) checkCacheStore(role roles.Roleer) (err error) {
	if z.cacheMax <= 0 {
		err = z.local_store.StoreRole(role)
		return err
	} else {
		return nil
	}
}
