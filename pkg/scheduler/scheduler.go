package scheduler

import (
	"fmt"
	"math/rand"
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
	latencies         *sync.Map
	heartbeats        *sync.Map
	nodeIdsByFunction *sync.Map
	inflightRequests  *sync.Map
}

func New(
	latencyCh <-chan Latency,
	heartbeatCh <-chan messages.Heartbeat,
) *Scheduler {
	rand.Seed(time.Now().Unix())
	s := &Scheduler{
		latencies:         &sync.Map{},
		heartbeats:        &sync.Map{},
		nodeIdsByFunction: &sync.Map{},
		inflightRequests:  &sync.Map{},
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

			v, ok := s.inflightRequests.Load(key)
			if ok {
				s.inflightRequests.Store(key, v.(int)-1)
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
			s.inflightRequests.Range(func(key, value interface{}) bool {
				fmt.Println(key, value)
				return true
			})
			s.latencies.Range(func(key, value interface{}) bool {
				fmt.Println(key, value)
				return true
			})
			for k, v := range m {
				s.nodeIdsByFunction.Store(k, v)
			}
		}
	}()

	return s
}

func (s *Scheduler) calcLoad(nodeId, functionName string) (float64, int) {
	key := nodeId + "." + functionName
	var avg float64
	v, ok := s.latencies.Load(key)
	if !ok {
		avg = 0
	} else {
		avg = v.(float64)
	}

	var requests int
	v, ok = s.inflightRequests.Load(key)
	if !ok {
		requests = 0
	} else {
		requests = v.(int)
	}

	return avg * float64(requests), requests
}

func (s *Scheduler) Schedule(functionName string) (string, error) {
	v, ok := s.nodeIdsByFunction.Load(functionName)
	if !ok {
		return "", fmt.Errorf("function not found")
	}

	nodeIds := v.([]string)
	if len(nodeIds) == 1 {
		return nodeIds[0], nil
	}

	firstNode := nodeIds[rand.Intn(len(nodeIds))]
	secondNode := nodeIds[rand.Intn(len(nodeIds))]
	for secondNode == firstNode {
		secondNode = nodeIds[rand.Intn(len(nodeIds))]
	}

	firstLoad, firstRequests := s.calcLoad(firstNode, functionName)
	secondLoad, secondRequests := s.calcLoad(secondNode, functionName)

	var nodeId string
	var requests int
	if firstLoad < secondLoad {
		nodeId = firstNode
		requests = firstRequests
	} else {
		nodeId = secondNode
		requests = secondRequests
	}

	s.inflightRequests.Store(nodeId+"."+functionName, requests+1)
	return nodeId, nil
}
