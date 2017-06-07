// Copyright 2016-2017
// CoderG the 2016 project
// Insight 0+0 [ 洞悉 0+0 ]
// InDimensions Construct Source [ 忆黛蒙逝·建造源 ] -> idcsource@gmail.com
// Stephen Fire Meditation Qin [ 火志溟 ] -> firemeditation@gmail.com
// Use of this source code is governed by GNU LGPL v3 license

package dspiders

import (
	"time"

	"github.com/idcsource/Insight-0-0-lib/roles"
)

// The Url basic information for url queue channel
type UrlBasic struct {
	Domain string // the url belonged which domain
	Url    string // the url self
	Text   string // the url's name
	Hash   string // the url page last version hash
	Ver    uint64 // the url page last version
	Filter bool   // The this is a list url, the bool is true, it will just get all the url but not store the page data.
}

// The Url's queue channel
type UrlChannel chan UrlBasic

// One words index request
type WordsIndexRequest struct {
	Url        string         // The index queue url
	Domain     string         // The Url domain
	Type       WordsIndexType // the index type is what
	PageData   *PageData      // the page data
	AroundLink *AroundLink    // the around link
}

// One page's data.
type PageData struct {
	roles.Role
	Url         string    // The complete link address
	Ver         uint64    // The page version
	UpTime      time.Time // The update time
	UpInterval  int64     // The update time interval(will wait UpInterval second to next update.)
	Domain      string    // The domain name.
	Spider      string    // The spider machine's name.
	KeyWords    []string  // The key words, from the html's header meta name=keywords
	HeaderTitle string    // The page's title, from <header><title></title></header>
	BodyContent string    // The page's body content, from <body></body>, and is all text
	Hash        string    // The page body content's(the field BodyConent) sha1 hash signature
}

// One media's data, for example css, image ...
type MediaData struct {
	roles.Role
	Url        string    // The complete link address
	Ver        uint64    // The page version
	UpTime     time.Time // The update time
	UpInterval int64     // The update time interval(will wait UpInterval second to next update.)
	Domain     string    // The domain name.
	Spider     string    // The spider machine's name.
	MediaType  int       // The media's type
	MediaName  string    // The Media's name
	DataSaved  bool      // If the data already be saved.
	DataBody   []byte    // The media's data body.
	Hash       string    // sha1 hash signature
}

// The link which not in the domain.
type AroundLink struct {
	roles.Role
	Url  string // the link address
	Text string // the link text
}

// The sentences location index in one page
type SentencesIndex struct {
	Text  string // the text
	Index uint64 // the index location
}

// One page's all sentences and words location index
type PageSentences struct {
	roles.Role
	Url       string              // The complete link address
	Ver       uint64              // the page version
	Sentences []SentencesIndex    // the sentences index
	Words     map[string][]uint64 // the words index [word's text][]the index localtion
}

// The data which transport in network
type NetTransportData struct {
	Name    string              // The sender's name
	Code    string              // The sender's identity code
	Operate NetTransportOperate // The operate code
	Status  NetDataStatus       // The data status
	Domain  string              // The damain if it be need
	Data    []byte              // The data body, it can be PageData, UrlBasic and so on.
}
