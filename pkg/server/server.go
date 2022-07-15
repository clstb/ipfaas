package server

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/clstb/ipfaas/pkg/ipfs"
	"github.com/clstb/ipfaas/pkg/messages"
	"github.com/clstb/ipfaas/pkg/resolver"
	"github.com/clstb/ipfaas/pkg/scheduler"
	"github.com/containerd/containerd"
	"github.com/containerd/go-cni"
	"github.com/gofiber/fiber/v2"
	icore "github.com/ipfs/interface-go-ipfs-core"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/valyala/fasthttp"
	"github.com/vmihailenco/msgpack/v5"
	"go.uber.org/zap"
)

type Server struct {
	*fiber.App
	scheduler *scheduler.Scheduler
	resolver  *resolver.Resolver
	ipfs      *ipfs.IPFS
	client    *fasthttp.Client
	offloads  *sync.Map
	latencyCh chan<- scheduler.Latency

	containerd *containerd.Client
	cni        cni.CNI
	logger     *zap.Logger
}

func New(
	ctx context.Context,
	logger *zap.Logger,
	containerd *containerd.Client,
	cni cni.CNI,
	repository string,
) (*Server, error) {
	heartbeatCh := make(chan messages.Heartbeat)
	latencyCh := make(chan scheduler.Latency)

	scheduler := scheduler.New(
		latencyCh,
		heartbeatCh,
	)

	ipfs, err := ipfs.New(ctx, logger, repository)
	if err != nil {
		return nil, err
	}

	s := &Server{
		App:        fiber.New(),
		scheduler:  scheduler,
		resolver:   resolver.New(containerd),
		ipfs:       ipfs,
		client:     &fasthttp.Client{},
		offloads:   &sync.Map{},
		latencyCh:  latencyCh,
		containerd: containerd,
		cni:        cni,
		logger:     logger,
	}

	if err := s.ipfs.Subscribe(ctx, "heartbeats"); err != nil {
		return nil, err
	}

	go func() {
		for msg := range ipfs.Messages() {
			topic := msg.Topics()[0]
			var err error
			switch {
			case strings.HasSuffix(topic, "_responses"):
				if msg.From().String() == ipfs.NodeId {
					continue
				}

				functionResponse := messages.FunctionResponse{}
				if err = msgpack.Unmarshal(msg.Data(), &functionResponse); err != nil {
					continue
				}

				v, ok := s.offloads.Load(functionResponse.RequestId)
				if !ok {
					continue
				}
				ch := v.(chan messages.FunctionResponse)
				ch <- functionResponse
			case strings.HasSuffix(topic, "_requests"):
				if msg.From().String() == ipfs.NodeId {
					continue
				}

				functionRequest := messages.FunctionRequest{}
				if err := msgpack.Unmarshal(msg.Data(), &functionRequest); err != nil {
					continue
				}

				if functionRequest.NodeId != s.ipfs.NodeId {
					continue
				}

				go s.handleFunctionRequest(functionRequest)
			case topic == "heartbeats":
				go func(msg icore.PubSubMessage) {
					heartbeat := messages.Heartbeat{}
					if err = msgpack.Unmarshal(msg.Data(), &heartbeat); err != nil {
						return
					}

					if msg.From().String() == ipfs.NodeId {
						for _, function := range heartbeat.Functions {
							err = s.ipfs.Subscribe(ctx, function+"_requests")
							err = s.ipfs.Subscribe(ctx, function+"_responses")
						}
					}
					heartbeatCh <- heartbeat
				}(msg)
			}
			if err != nil {
				logger.Error("handling message", zap.Error(err))
			}
		}
	}()
	go func() {
		t := time.NewTicker(3 * time.Second)
		for range t.C {
			mem, err := mem.VirtualMemory()
			if err != nil {
				s.logger.Error("getting mem metric", zap.Error(err))
				continue
			}

			cpu, err := cpu.Percent(time.Second*5, false)
			if err != nil {
				s.logger.Error("getting cpu metric", zap.Error(err))
				continue
			}

			var functions []string
			s.resolver.FunctionURLs.Range(func(key, value interface{}) bool {
				function := value.(*resolver.Function)
				if function.ExpiresAt.Before(time.Now()) {
					return true
				}
				functions = append(functions, function.Name)
				return true
			})

			heartbeat := messages.Heartbeat{
				NodeId:    s.ipfs.NodeId,
				UsedMEM:   mem.UsedPercent,
				UsedCPU:   cpu[0],
				Functions: functions,
			}

			b, err := msgpack.Marshal(&heartbeat)
			if err != nil {
				s.logger.Error("marshalling message", zap.Error(err))
				continue
			}

			if err := s.ipfs.PubSub().Publish(ctx, "heartbeats", b); err != nil {
				s.logger.Error("publishing message", zap.Error(err))
			}
		}
	}()

	s.routes()

	return s, nil
}
