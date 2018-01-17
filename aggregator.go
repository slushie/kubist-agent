package main

import (
	"sync"
	"errors"
	"k8s.io/client-go/tools/cache"
)

type ChannelAggregator struct {
	wg *sync.WaitGroup
	stop chan struct{}
	out chan<- cache.Delta
}

func NewChannelAggregator(out chan<- cache.Delta) *ChannelAggregator {
	return &ChannelAggregator{wg:&sync.WaitGroup{}, stop:make(chan struct{}), out:out}
}

func (ca *ChannelAggregator) Add(ch <-chan cache.Delta) error {
	select {
	case <-ca.stop:
		return errors.New("can't add channel to stopped aggregator")
	default:
	}

	ca.wg.Add(1)
	go func() {
		defer ca.wg.Done()
		for {
			select {
			case <- ca.stop:
				return
			case v := <- ch:
				ca.out <- v
			}
		}
	}()
	
	return nil
}

func (ca *ChannelAggregator) Wait() {
	ca.wg.Wait()
}

func (ca *ChannelAggregator) Stop() {
	close(ca.stop)
}