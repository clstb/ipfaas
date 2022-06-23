package messages

import (
	"github.com/valyala/fasthttp"
)

type Heartbeat struct {
	NodeId    string
	UsedMEM   float64
	UsedCPU   float64
	Functions []string
}

type FunctionResponse struct {
	FunctionName string
	Data         []byte
	Header       fasthttp.ResponseHeader
	RequestId    string
	IsCID        bool
}

type FunctionRequest struct {
	FunctionName string
	Data         []byte
	Params       string
	Query        string
	NodeId       string
	RequestId    string
	IsCID        bool
	PublishIPFS  bool
}
