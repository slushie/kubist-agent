package main

import (
	"fmt"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"github.com/slushie/kubist-agent/couchdb"
	"k8s.io/client-go/dynamic"
	"github.com/slushie/kubist-agent/kubernetes"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"strings"
	"strconv"
)

var Resources = []schema.GroupVersionResource{
	{"", "v1", "pods"},
}

var ch = make(chan cache.Delta)
var Watchers = NewChannelAggregator(ch)

func RunAgent(db *couchdb.Database, pool dynamic.ClientPool) {
	for _, gvr := range Resources {
		client, err := pool.ClientForGroupVersionResource(gvr)
		if  err != nil {
			panic(err.Error())
		}

		rw := kubernetes.NewResourceWatcher(client, gvr.Resource, "")
		Watchers.Add(rw.Watch())
	}

	fmt.Println("watching resources...")

	for {
		select {
		case delta := <- ch:
			applyDelta(db, delta)
		}
	}

	fmt.Println("bye felicia")
}

func applyDelta(db *couchdb.Database, delta cache.Delta) {
	object := delta.Object.(*unstructured.Unstructured)
	rv := object.GetResourceVersion()

	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(object)
	if err != nil {
		panic(err.Error())
	}

	id := object.GetKind() + key
	fmt.Printf("[%s] %s rv=%s", delta.Type, id, rv)

	action := strings.ToUpper(string(delta.Type))
	switch delta.Type {
	case cache.Added:
		if doc, err := db.GetOrNil(id); err != nil {
			panic(err.Error())
		} else if doc != nil {
			docObject := &unstructured.Unstructured{Object: doc}
			docRv := docObject.GetResourceVersion()
			if docRv != rv {
				fmt.Printf("[!] ADD %s: conflict resourceVersion %#v != %#v\n", id, rv, docRv)
			} else {
				fmt.Printf("[!] ADD %s: existing resourceVersion %#v\n", id, docRv)
			}
			break // nothing to write
		}

		object.Object["_id"] = id
		_, err := db.Put(id, object.Object)
		if status := err.(*couchdb.StatusObject); status != nil {
			fmt.Printf("[!] ADD %s: put %s\n", id, status.Status)
		} else if err != nil {
			panic(err.Error())
		}

	case cache.Updated, cache.Sync:
		put := object.DeepCopy().Object
		put["_id"] = id

		if doc, err := db.GetOrNil(id); err != nil {
			panic(err.Error())
		} else if doc == nil {
			fmt.Printf("[~] %s %s: new document\n", action, id)
		} else {
			put["_rev"] = doc["_rev"]

			docObject := &unstructured.Unstructured{Object: doc}
			docRv := docObject.GetResourceVersion()
			if parseRv(rv) < parseRv(docRv) {
				fmt.Printf("[!] %s %s: conflict resourceVersion %#v < %#v\n", action, id, rv, docRv)
				break // old version, don't overwrite
			} else if rv == docRv {
				break // same version, don't overwrite
			}
		}

		_, err = db.Put(id, put)
		if status := err.(*couchdb.StatusObject); status != nil {
			fmt.Printf("[!] %s %s: put %s\n", action, id, status.Status)
		} else if err != nil {
			panic(err.Error())
		}

	case cache.Deleted:
		if doc, err := db.GetOrNil(id); err != nil {
			panic(err.Error())
		} else if doc != nil {
			if _, err := db.Delete(doc); err != nil {
				fmt.Printf("DELETE %s: %s\n", id, err.Error())
			}
		}

	default:
		panic("what what in the butt")
	}
}

func parseRv(rv string) int {
	if i, err := strconv.Atoi(rv); err != nil {
		panic(err.Error())
	} else {
		return i
	}
}