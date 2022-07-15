package scheduler

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/VividCortex/ewma"
	"github.com/clstb/ipfaas/pkg/messages"
)

type Latency struct {
	NodeId       string
	FunctionName string
	Value        int64
}

type heatbeatWithExpiry struct {
	messages.Heartbeat
	expiresAt time.Time
}

type Scheduler struct {
	latencies           *sync.Map
	heartbeats          *sync.Map
	nodeIdsByFunction   *sync.Map
	outstandingRequests *sync.Map
}

func New(
	latencyCh <-chan Latency,
	heartbeatCh <-chan messages.Heartbeat,
) *Scheduler {
	s := &Scheduler{
		latencies:           &sync.Map{},
		heartbeats:          &sync.Map{},
		nodeIdsByFunction:   &sync.Map{},
		outstandingRequests: &sync.Map{},
	}
	go func() {
		ewmas := map[string]ewma.MovingAverage{}
		for latency := range latencyCh {
			key := latency.NodeId + "." + latency.FunctionName
			avg, ok := ewmas[key]
			if !ok {
				avg = ewma.NewMovingAverage()
			}
			avg.Add(float64(latency.Value))
			ewmas[key] = avg
			s.latencies.Store(key, avg.Value())

			v, ok := s.outstandingRequests.Load(key)
			if ok {
				outstandingRequest := v.(uint)
				s.outstandingRequests.Store(key, outstandingRequest-1)
			}
		}
	}()
	go func() {
		for heartbeat := range heartbeatCh {
			s.heartbeats.Store(heartbeat.NodeId, heatbeatWithExpiry{
				Heartbeat: heartbeat,
				expiresAt: time.Now().Add(10 * time.Second),
			})
		}
	}()
	go func() {
		t := time.NewTicker(3 * time.Second)
		for range t.C {
			m := map[string][]string{}
			s.heartbeats.Range(func(key, value interface{}) bool {
				heartbeat := value.(heatbeatWithExpiry)
				if heartbeat.expiresAt.Before(time.Now()) {
					return true
				}
				for _, function := range heartbeat.Functions {
					m[function] = append(m[function], heartbeat.NodeId)
				}
				return true
			})
			for k, v := range m {
				s.nodeIdsByFunction.Store(k, v)
			}
		}
	}()

	return s
}

func (s *Scheduler) Schedule(functionName string) (string, error) {
	v, ok := s.nodeIdsByFunction.Load(functionName)
	if !ok {
		return "", fmt.Errorf("function not found")
	}

	nodeIds := v.([]string)
	var nodeId string
	var outstandingRequests uint
	load := math.MaxFloat64
	for _, id := range nodeIds {
		key := id + "." + functionName
		v, ok := s.latencies.Load(key)
		if !ok {
			nodeId = id
			break
		}
		avg := v.(float64)

		v, ok = s.outstandingRequests.Load(key)
		if !ok {
			nodeId = id
			break
		}
		requests := v.(uint)

		l := float64(requests) * avg
		if load > l {
			nodeId = id
			load = l
			outstandingRequests = requests
		}
	}
	go func() {
		s.outstandingRequests.Store(nodeId+"."+functionName, outstandingRequests+1)
	}()

	return nodeId, nil
}
