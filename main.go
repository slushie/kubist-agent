package main

import (
	"fmt"
	k "github.com/slushie/kubist-agent/kubernetes"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	//"sync"
	"k8s.io/client-go/tools/cache"
	//"flag"
	"encoding/json"
	"bytes"
	//"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var Resources = []schema.GroupVersionResource{
	{"", "v1", "pods"},
}

var ch = make(chan cache.Delta)
var Watchers = NewChannelAggregator(ch)

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
		Watchers.Add(rw.Watch())
	}

	fmt.Println("watching resources...")

	for {
		select {
		case delta := <- ch:
			if u := delta.Object.(*unstructured.Unstructured); u != nil {
				fmt.Printf("[%s] %s: %s/%s\n", delta.Type, u.GetKind(), u.GetNamespace(), u.GetName())
			} else {
				fmt.Printf("[%s] Unknown %T\n", delta.Type, delta.Object)
			}
		}
	}

	fmt.Println("done")
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