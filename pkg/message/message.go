package message

import (
	"bytes"
	"encoding/json"

	"github.com/clstb/ipfs-connector/pkg/openfaas"
	ipfs "github.com/ipfs/go-ipfs-api"
	"github.com/openfaas/connector-sdk/types"
)

type (
	MessageType int
	Message     struct {
		Type       MessageType `json:"type"`
		Topic      string
		SubMessage interface{} `json:"sub_message"`
	}
	FunctionDeployMessage   openfaas.Function
	FunctionInvokeMessage   []byte
	FunctionResponseMessage types.InvokerResponse
	IPFSMessage             *ipfs.Message
)

func (m FunctionInvokeMessage) Bytes() []byte {
	return []byte(m)
}

const (
	FunctionDeploy = iota
	FunctionInvoke
	FunctionResponse
	IPFS
)

var messageTypeToString = map[MessageType]string{
	FunctionDeploy:   "function_deploy",
	FunctionInvoke:   "function_invoke",
	FunctionResponse: "function_response",
	IPFS:             "ipfs",
}

var messageTypeToID = map[string]MessageType{
	"function_deploy":   FunctionDeploy,
	"function_invoke":   FunctionInvoke,
	"function_response": FunctionResponse,
	"ipfs":              IPFS,
}

func (t MessageType) String() string {
	return messageTypeToString[t]
}

// MarshalJSON marshals the MessageType as a quoted json string
func (s MessageType) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(messageTypeToString[s])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmashals a quoted json string to the MessageType
func (s *MessageType) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	// Note that if the string cannot be found then it will be set to the zero value, 'Created' in this case.
	*s = messageTypeToID[j]
	return nil
}
