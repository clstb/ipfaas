package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/clstb/ipfaas/pkg/messages"
	"github.com/clstb/ipfaas/pkg/scheduler"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/ipfs/go-cid"
	icore "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/valyala/fasthttp"
	"github.com/vmihailenco/msgpack/v5"
)

func (s *Server) handleFunctionRequest(msg icore.PubSubMessage) error {
	ctx := context.Background()

	functionRequest := messages.FunctionRequest{}
	if err := msgpack.Unmarshal(msg.Data(), &functionRequest); err != nil {
		return fmt.Errorf("parsing message: %w", err)
	}

	if functionRequest.NodeId != s.ipfs.NodeId {
		return nil
	}

	name := strings.TrimSuffix(msg.Topics()[0], "_requests")

	addr, ok := s.resolver.Resolve(name)
	if !ok {
		return fmt.Errorf("resolving function: %s", name)
	}

	url, err := url.Parse(addr)
	if err != nil {
		return fmt.Errorf("parsing function address: %w", err)
	}
	url.Path = functionRequest.Params
	url.RawQuery = functionRequest.Query

	if functionRequest.IsCID {
		cid, err := cid.Decode(string(functionRequest.Data))
		if err != nil {
			return fmt.Errorf("casting cid: %w", err)
		}

		r, err := s.ipfs.Block().Get(ctx, path.IpfsPath(cid))
		if err != nil {
			return fmt.Errorf("getting block: %w", err)
		}

		functionRequest.Data, err = io.ReadAll(r)
		if err != nil {
			return fmt.Errorf("reading block: %w", err)
		}
	}

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	req.SetRequestURI(url.String())
	req.SetBody(functionRequest.Data)

	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(res)

	if err := s.client.Do(req, res); err != nil {
		return fmt.Errorf("calling function: %s: %w", name, err)
	}

	functionResponse := messages.FunctionResponse{
		FunctionName: functionRequest.FunctionName,
		Data:         res.Body(),
		Header:       res.Header,
		RequestId:    functionRequest.RequestId,
	}

	if functionRequest.PublishIPFS {
		block, err := s.ipfs.Block().Put(ctx, bytes.NewReader(functionResponse.Data))
		if err != nil {
			return fmt.Errorf("putting block: %w", err)
		}

		functionResponse.Data = []byte(block.Path().Cid().String())
		functionResponse.IsCID = true
	}

	b, err := msgpack.Marshal(&res)
	if err != nil {
		return fmt.Errorf("marshalling message: %w", err)
	}

	if err := s.ipfs.PubSub().Publish(
		ctx,
		name+"_responses",
		b,
	); err != nil {
		return fmt.Errorf("publishing message: %w", err)
	}

	return nil
}

func (s *Server) FunctionHandler() fiber.Handler {
	offload := func(functionName, requestId, nodeId string, c *fiber.Ctx) error {
		ch := make(chan messages.FunctionResponse)
		headers := c.GetReqHeaders()
		_, isCID := headers["Ipfaas-Is-Cid"]
		_, publishIpfs := headers["Ipfaas-Publish-Ipfs"]

		req := messages.FunctionRequest{
			FunctionName: functionName,
			Data:         c.Body(),
			Params:       c.Params("params"),
			Query:        string(c.Request().URI().QueryString()),
			NodeId:       nodeId,
			RequestId:    requestId,
			IsCID:        isCID,
			PublishIPFS:  publishIpfs,
		}

		b, err := msgpack.Marshal(&req)
		if err != nil {
			return fmt.Errorf("marshalling message: %w", err)
		}

		s.offloads.Store(requestId, ch)
		defer s.offloads.Delete(requestId)

		go func() {
			s.latencyCh <- scheduler.Latency{
				NodeId:       s.ipfs.NodeId,
				FunctionName: functionName,
				RequestId:    requestId,
			}
		}()
		defer func() {
			s.latencyCh <- scheduler.Latency{
				NodeId:       s.ipfs.NodeId,
				FunctionName: functionName,
				RequestId:    requestId,
				Done:         true,
			}
		}()

		if err := s.ipfs.PubSub().Publish(
			c.Context(),
			functionName+"_requests",
			b,
		); err != nil {
			return fmt.Errorf("publishing message: %w", err)
		}

		res := <-ch

		c.Response().Header = res.Header
		return c.Send(res.Data)
	}
	handle := func(functionName, requestId string, c *fiber.Ctx) error {
		addr, ok := s.resolver.Resolve(functionName)
		if !ok {
			return fmt.Errorf("resolving function")
		}

		c.Request().SetRequestURI(addr)

		go func() {
			s.latencyCh <- scheduler.Latency{
				NodeId:       s.ipfs.NodeId,
				FunctionName: functionName,
				RequestId:    requestId,
			}
		}()
		defer func() {
			s.latencyCh <- scheduler.Latency{
				NodeId:       s.ipfs.NodeId,
				FunctionName: functionName,
				RequestId:    requestId,
				Done:         true,
			}
		}()
		if err := s.client.Do(c.Request(), c.Response()); err != nil {
			return fmt.Errorf("calling function: %s: %w", functionName, err)
		}

		return nil
	}

	return func(c *fiber.Ctx) error {
		requestId := utils.UUIDv4()

		functionName := c.Params("name")
		if functionName == "" {
			return fmt.Errorf("Provide function name in the request path")
		}

		nodeId, err := s.scheduler.Schedule(functionName)
		if err != nil {
			return fmt.Errorf("scheduling: %w", err)
		}

		if nodeId == s.ipfs.NodeId {
			return handle(functionName, requestId, c)
		}
		return offload(functionName, requestId, nodeId, c)
	}
}
