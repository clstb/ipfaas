package main

import "github.com/clstb/ipfs-connector/pkg/message"

func fanOut(n int, in <-chan message.Message) []<-chan message.Message {
	var outs []<-chan message.Message
	var internalOuts []chan message.Message

	for i := 0; i < n; i++ {
		out := make(chan message.Message)
		outs = append(outs, out)
		internalOuts = append(internalOuts, out)
	}

	go func() {
		for msg := range in {
			for _, out := range internalOuts {
				out <- msg
			}
		}
		for _, out := range internalOuts {
			close(out)
		}
	}()

	return outs
}
