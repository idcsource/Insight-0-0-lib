// Copyright 2016-2017
// CoderG the 2016 project
// Insight 0+0 [ 洞悉 0+0 ]
// InDimensions Construct Source [ 忆黛蒙逝·建造源 ] -> idcsource@gmail.com
// Stephen Fire Meditation Qin [ 火志溟 ] -> firemeditation@gmail.com
// Use of this source code is governed by GNU LGPL v3 license

package dspiders

import (
	"fmt"

	"github.com/idcsource/insight00-lib/cpool"
	"github.com/idcsource/insight00-lib/iendecode"
	"github.com/idcsource/insight00-lib/nst2"
	"github.com/idcsource/insight00-lib/roles"
)

// The network transport handle.
//
// Order by nst.ConnExecer interface.
//
// It operate url crawl queue's request, the page's storage, the words's index and others.
type NetTransportHandle struct {
	urlCrawlQueue  *UrlCrawlQueue           // The url crawl queue
	pageProcess    map[string]*PagesProcess // The page process
	wordProcess    *WordsIndexProcess       // The word index process
	identityConfig *cpool.Section           // The indentity code's config, the *cpool.Section is name = code
	closed         bool                     // if closed it will true
	// TODO : Others
}

func NewNetTransportHandle(u *UrlCrawlQueue, p map[string]*PagesProcess, w *WordsIndexProcess, i *cpool.Section) (n *NetTransportHandle) {
	return &NetTransportHandle{
		urlCrawlQueue:  u,
		pageProcess:    p,
		wordProcess:    w,
		identityConfig: i,
		closed:         true,
	}
}

func (n *NetTransportHandle) Start() {
	n.wordProcess.Start()
	for key := range n.pageProcess {
		n.pageProcess[key].Start()
	}
	n.closed = false
}

func (n *NetTransportHandle) Close() {
	n.wordProcess.Close()
	for key := range n.pageProcess {
		n.pageProcess[key].Close()
	}
	n.closed = true
}

func (n *NetTransportHandle) NSTexec(ce *nst2.ConnExec) (stat nst2.SendStat, err error) {
	// get the crawl machine send
	c_send_b, err := ce.GetData()
	if err != nil {
		stat = nst2.SEND_STAT_NOT_OK
		return
	}
	// if close
	if n.closed == true {
		return nst2.SEND_STAT_NOT_OK, n.sendReceipt(ce, NET_DATA_ERROR, []byte("The service was closed."))
	}
	// decode
	c_send := NetTransportData{}
	err = iendecode.BytesGobStruct(c_send_b, &c_send)
	if err != nil {
		return nst2.SEND_STAT_NOT_OK, n.sendReceipt(ce, NET_DATA_ERROR, []byte("The NetTransportData decode error: "+err.Error()))
	}
	// check the identity
	code, err := n.identityConfig.GetConfig(c_send.Name)
	if err != nil {
		return nst2.SEND_STAT_NOT_OK, n.sendReceipt(ce, NET_DATA_ERROR, []byte("Can not found the identity."))
	}
	if code != c_send.Code {
		return nst2.SEND_STAT_NOT_OK, n.sendReceipt(ce, NET_DATA_ERROR, []byte("Can not found the identity."))
	}
	// get the operator name
	switch c_send.Operate {
	case NET_TRANSPORT_OPERATE_URL_CRAWL_QUEUE_ADD:
		err = n.addUrlToCrawlQueue(ce, &c_send)
	case NET_TRANSPORT_OPERATE_URL_CRAWL_QUEUE_GET:
		err = n.getUrlFromCrawlQueue(ce, &c_send)
	case NET_TRANSPORT_OPERATE_SEND_PAGE_DATA:
		err = n.thePageData(ce, &c_send)
	case NET_TRANSPORT_OPERATE_SEND_MEDIA_DATA:
		err = n.theMediaData(ce, &c_send)
	default:
		return nst2.SEND_STAT_NOT_OK, n.sendReceipt(ce, NET_DATA_ERROR, []byte("Unspecified operation."))
	}
	if err != nil {
		stat = nst2.SEND_STAT_NOT_OK
	} else {
		stat = nst2.SEND_STAT_OK
	}
	return
}

func (n *NetTransportHandle) addUrlToCrawlQueue(ce *nst2.ConnExec, c_send *NetTransportData) (err error) {
	// decode data, the type is []UrlBasic
	var urls []UrlBasic
	err = iendecode.BytesGobStruct(c_send.Data, &urls)
	if err != nil {
		return n.sendReceipt(ce, NET_DATA_ERROR, []byte("The []UrlBasic decode error: "+err.Error()))
	}
	if _, have := n.pageProcess[c_send.SiteName]; have == true {
		err = n.pageProcess[c_send.SiteName].AddUrls(urls)
		if err != nil {
			return n.sendReceipt(ce, NET_DATA_ERROR, []byte(err.Error()))
		} else {
			return n.sendReceipt(ce, NET_DATA_STATUS_OK, nil)
		}
	} else {
		return n.sendReceipt(ce, NET_DATA_ERROR, []byte("Nonexistent site name."))
	}
	return
}

func (n *NetTransportHandle) getUrlFromCrawlQueue(ce *nst2.ConnExec, c_send *NetTransportData) (err error) {
	url, err := n.urlCrawlQueue.Get()
	if err != nil {
		fmt.Println(err)
		return n.sendReceipt(ce, NET_DATA_ERROR, []byte(err.Error()))
	}
	url_b, err := iendecode.StructGobBytes(url)
	if err != nil {
		fmt.Println(err)
		return n.sendReceipt(ce, NET_DATA_ERROR, []byte("The UrlBasic decode error: "+err.Error()))
	}
	err = n.sendReceipt(ce, NET_DATA_STATUS_OK, url_b)
	if err != nil {
		fmt.Println(err)
	}
	return
}

func (n *NetTransportHandle) thePageData(ce *nst2.ConnExec, c_send *NetTransportData) (err error) {
	// decode data, the type is PageData's middle data
	mid := roles.RoleMiddleData{}
	err = iendecode.BytesGobStruct(c_send.Data, &mid)
	if err != nil {
		return n.sendReceipt(ce, NET_DATA_ERROR, []byte("The RoleMiddleData decode error: "+err.Error()))
	}
	// decode RoleMiddleData to Role
	pagedata := &PageData{}
	err = roles.DecodeMiddleToRole(mid, pagedata)
	if err != nil {
		return n.sendReceipt(ce, NET_DATA_ERROR, []byte("The PageData decode error: "+err.Error()))
	}
	// send to
	if _, have := n.pageProcess[c_send.SiteName]; have == true {
		err = n.pageProcess[c_send.SiteName].AddPage(pagedata, c_send.Status)
		if err != nil {
			return n.sendReceipt(ce, NET_DATA_ERROR, []byte(err.Error()))
		} else {
			return n.sendReceipt(ce, NET_DATA_STATUS_OK, nil)
		}
	} else {
		return n.sendReceipt(ce, NET_DATA_ERROR, []byte("Nonexistent site name."))
	}

	return
}

func (n *NetTransportHandle) theMediaData(ce *nst2.ConnExec, c_send *NetTransportData) (err error) {
	return
}

func (n *NetTransportHandle) sendReceipt(ce *nst2.ConnExec, status NetDataStatus, data []byte) (err error) {
	n_send := NetTransportDataRe{
		Status: status,
		Data:   data,
	}
	n_send_b, err := iendecode.StructGobBytes(n_send)
	if err != nil {
		return
	}
	err = ce.SendData(n_send_b)
	return
}
