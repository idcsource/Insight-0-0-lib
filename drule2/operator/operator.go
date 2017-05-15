// Copyright 2016-2017
// CoderG the 2016 project
// Insight 0+0 [ 洞悉 0+0 ]
// InDimensions Construct Source [ 忆黛蒙逝·建造源 ]
// Stephen Fire Meditation Qin [ 火志溟 ] -> firemeditation@gmail.com
// Use of this source code is governed by GNU LGPL v3 license

package operator

import (
	"fmt"
	"sync"
	"time"

	"github.com/idcsource/Insight-0-0-lib/drule2/trule"
	"github.com/idcsource/Insight-0-0-lib/iendecode"
	"github.com/idcsource/Insight-0-0-lib/ilogs"
	"github.com/idcsource/Insight-0-0-lib/nst"
)

// 创建一个操作者，自己的名字，远程地址，连接数，用户名，密码，日志
func NewOperator(selfname string, addr string, conn_num int, username, password string, log *ilogs.Logs) (o *Operator, err error) {
	drule_conn, err := nst.NewTcpClient(addr, conn_num, log)
	if err != nil {
		err = fmt.Errorf("operator[Operator]NewOperator: %v", err)
		return
	}
	drule := &druleInfo{
		name:     addr,
		username: username,
		password: password,
		tcpconn:  drule_conn,
	}

	if err != nil {
		err = fmt.Errorf("operator[Operator]NewOperator: %v", err)
		return
	}
	operatorS := &operatorService{
		tran_signal: make(chan tranService, 10),
	}
	o = &Operator{
		selfname:        selfname,
		drule:           drule,
		service:         operatorS,
		transaction:     make(map[string]*OTransaction),
		login:           false,
		logs:            log,
		runstatus:       OPERATOR_RUN_RUNNING,
		closeing_signal: make(chan bool),
		closed_signal:   make(chan bool),
		tran_wait:       &sync.WaitGroup{},
	}
	// 自动登陆
	err = o.autoLogin()
	if err != nil {
		return
	}
	// 事务信号监控
	go o.transactionSignalHandle()
	// 关闭信号处理
	go o.closeSignalHandle()
	return
}

// 创建一个操作者，并使用加密连接。自己的名字，远程地址，连接数，用户名，密码，日志
func NewOperatorTLS(selfname string, addr string, conn_num int, username, password string, log *ilogs.Logs) (o *Operator, err error) {
	drule_conn, err := nst.NewTcpClient(addr, conn_num, log)
	if err != nil {
		err = fmt.Errorf("operator[Operator]NewOperator: %v", err)
		return
	}
	err = nst.TcpClientTLS(drule_conn)
	if err != nil {
		err = fmt.Errorf("operator[Operator]NewOperator: %v", err)
		return
	}
	drule := &druleInfo{
		name:     addr,
		username: username,
		password: password,
		tcpconn:  drule_conn,
	}

	if err != nil {
		err = fmt.Errorf("operator[Operator]NewOperator: %v", err)
		return
	}
	operatorS := &operatorService{
		tran_signal: make(chan tranService, 10),
	}
	o = &Operator{
		selfname:        selfname,
		drule:           drule,
		service:         operatorS,
		transaction:     make(map[string]*OTransaction),
		login:           false,
		logs:            log,
		runstatus:       OPERATOR_RUN_RUNNING,
		closeing_signal: make(chan bool),
		closed_signal:   make(chan bool),
		tran_wait:       &sync.WaitGroup{},
	}
	// 自动登陆
	err = o.autoLogin()
	if err != nil {
		return
	}
	// 事务信号监控
	go o.transactionSignalHandle()
	// 关闭信号处理
	go o.closeSignalHandle()
	return
}

// 关闭
func (o *Operator) Close() {
	o.runstatus = OPERATOR_RUN_CLOSEING
	o.closeing_signal <- true
	// 开始等closed_signal
	<-o.closed_signal
	o.runstatus = OPERATOR_RUN_CLOSED
	o.drule.tcpconn.Close()
	o.drule.tcpconn = nil
	o.drule = nil
}

func (o *Operator) closeSignalHandle() {
	// 等待暂停中信号
	<-o.closeing_signal
	// 等待waiting的信号
	o.tran_wait.Wait()
	// 发送已经暂停信号
	o.closed_signal <- true
}

// 事务信号监控
func (o *Operator) transactionSignalHandle() {
	go o.tranTimeOutMonitor()
	for {
		if o.runstatus == OPERATOR_RUN_CLOSED {
			return
		}
		tran_signal := <-o.service.tran_signal
		if o.runstatus == OPERATOR_RUN_CLOSED {
			return
		}
		switch tran_signal.askfor {
		case TRANSACTION_ASKFOR_KEEPLIVE:
			if _, find := o.transaction[tran_signal.unid]; find == true {
				o.transaction[tran_signal.unid].activetime = time.Now()
			}
		case TRANSACTION_ASKFOR_END:
			if _, find := o.transaction[tran_signal.unid]; find == true {
				delete(o.transaction, tran_signal.unid)
				o.tran_wait.Done()
			}
		}
	}
}

// 事务超时处理
func (o *Operator) tranTimeOutMonitor() {
	for {
		if o.runstatus == OPERATOR_RUN_CLOSED {
			return
		}
		time.Sleep(time.Duration(trule.TRAN_TIME_OUT_CHECK) * time.Second)
		if o.runstatus == OPERATOR_RUN_CLOSED {
			return
		}
		for key, _ := range o.transaction {
			if o.transaction[key].activetime.Unix()+trule.TRAN_TIME_OUT < time.Now().Unix() {
				o.transaction[key].Commit()
			}
		}
	}
}

// 写登陆
func (o *Operator) autoLogin() (err error) {
	login := O_DRuleUser{
		UserName: o.drule.username,
		//Password: random.GetSha1Sum(o.drule.password),
		Password: o.drule.password,
	}
	// 编码
	login_b, err := iendecode.StructGobBytes(login)
	if err != nil {
		return
	}
	// 发送
	cprocess := o.drule.tcpconn.OpenProgress()
	defer cprocess.Close()
	drule_return, err := o.operatorSend(cprocess, "", "", OPERATE_ZONE_MANAGE, OPERATE_USER_LOGIN, login_b)
	if err != nil {
		return
	}
	if drule_return.DataStat != DATA_ALL_OK {
		return fmt.Errorf(drule_return.Error)
	}
	// 解码
	err = iendecode.BytesGobStruct(login_b, &login)
	if err != nil {
		return
	}
	o.login = true
	o.drule.unid = login.Unid

	// 开始监控自动登陆
	go o.autoKeepLife()

	return
}

// 自动续命
func (o *Operator) autoKeepLife() {
	for {
		time.Sleep(time.Duration(USER_ADD_LIFE) * time.Second)
		err := o.keepLifeOnec()
		if err != nil {
			o.login = false
			return
		}
	}
}

func (o *Operator) keepLifeOnec() (err error) {
	// 发送
	cprocess := o.drule.tcpconn.OpenProgress()
	defer cprocess.Close()
	drule_return, err := o.operatorSend(cprocess, "", "", OPERATE_ZONE_MANAGE, OPERATE_USER_ADD_LIFE, nil)
	if err != nil {
		return
	}
	if drule_return.DataStat != DATA_ALL_OK {
		return fmt.Errorf(drule_return.Error)
	}
	return
}

func (o *Operator) operatorSend(process *nst.ProgressData, areaid, roleid string, oz OperateZone, operate OperatorType, data []byte) (receipt O_DRuleReceipt, err error) {
	//	if o.login == false {
	//		err = fmt.Errorf("Not login to the DRule server.")
	//		return
	//	}
	thestat := O_OperatorSend{
		OperatorName:  o.selfname,
		OperateZone:   oz,
		Operate:       operate,
		TransactionId: "",
		InTransaction: false,
		RoleId:        roleid,
		AreaId:        areaid,
		User:          o.drule.username,
		Unid:          o.drule.unid,
		Data:          data,
	}
	statbyte, err := iendecode.StructGobBytes(thestat)
	if err != nil {
		return
	}
	rdata, err := process.SendAndReturn(statbyte)
	if err != nil {
		return
	}
	receipt = O_DRuleReceipt{}
	err = iendecode.BytesGobStruct(rdata, &receipt)
	return
}
