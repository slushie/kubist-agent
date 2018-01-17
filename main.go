package main

import (
	"fmt"
	k "github.com/slushie/kubist-agent/kubernetes"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sync"
	"k8s.io/client-go/tools/cache"
	//"flag"
	"encoding/json"
	"bytes"
)

var Resources = []schema.GroupVersionResource{
	{"", "v1", "pods"},
}

var Watchers = sync.WaitGroup{}

func main() {
	//flag.Set("logtostderr", "true")
	//flag.Set("v", "9")

	config, err := k.NewClientConfig("", nil)
	if err != nil {
		panic(err.Error())
	}

	pool := dynamic.NewDynamicClientPool(config)

	for _, gvr := range Resources {
		client, err := pool.ClientForGroupVersionResource(gvr)
		if  err != nil {
			panic(err.Error())
		}

		rw := k.NewResourceWatcher(client, gvr.Resource, "")
		AddWatch(rw)
	}

	fmt.Println("watching resources...")
	Watchers.Wait()
	fmt.Println("done")
}

func AddWatch(watcher *k.ResourceWatcher) {
	Watchers.Add(1)
	go func(ch <-chan cache.Delta) {
		defer Watchers.Done()
		for {
			select {
			// TODO select on a stop channel
			case delta := <-ch:
				//data, _ := ToJson(delta)
				fmt.Printf("new delta: %s %T\n",
					delta.Type, delta.Object)
			}
		}
	}(watcher.Watch())
}

func ToJson(o interface{}) ([]byte, error) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "  ")

	if err := enc.Encode(o); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}