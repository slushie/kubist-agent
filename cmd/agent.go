package cmd

import (
	"fmt"
	"github.com/slushie/kubist-agent/couchdb"
	"github.com/slushie/kubist-agent/kubernetes"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"strconv"
	"strings"
)

type KubistAgent struct {
	ch        chan cache.Delta
	db        couchdb.DatabaseInterface
	pool      dynamic.ClientPool
	Resources []schema.GroupVersionResource
	Namespace string

	Watchers *ChannelAggregator
	PoolSize int
}

var DefaultPoolSize = 10

func NewKubistAgent(
	db couchdb.DatabaseInterface,
	pool dynamic.ClientPool,
	resources []schema.GroupVersionResource,
	namespace string,
) *KubistAgent {
	var ch = make(chan cache.Delta)

	return &KubistAgent{
		ch:        ch,
		db:        db,
		pool:      pool,
		Resources: resources,
		Namespace: namespace,
		Watchers:  NewChannelAggregator(ch),
		PoolSize:  DefaultPoolSize,
	}
}

func (ka *KubistAgent) Run() {
	for _, gvr := range ka.Resources {
		client, err := ka.pool.ClientForGroupVersionResource(gvr)
		if err != nil {
			panic(err.Error())
		}

		rw := kubernetes.NewResourceWatcher(client, gvr.Resource, ka.Namespace)
		ka.Watchers.Add(rw.Watch())
	}

	for i := 0; i < ka.PoolSize; i += 1 {
		go func () {
            for delta := range ka.ch {
                ka.applyDelta(delta)
            }
		}()
	}

	fmt.Println("bye felicia")
}

func (ka *KubistAgent) Stop() {
    ka.Watchers.Stop()
}

func (ka *KubistAgent) applyDelta(delta cache.Delta) {
	rsrc := delta.Object.(*unstructured.Unstructured)
	rv := rsrc.GetResourceVersion()

	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(rsrc)
	if err != nil {
		panic(err.Error())
	}

	id := rsrc.GetKind() + "/" + key
	fmt.Printf("[%s] %s rv=%s\n", delta.Type, id, rv)

	action := strings.ToUpper(string(delta.Type))
	switch delta.Type {
	case cache.Added:
		if doc, err := ka.db.GetOrNil(id); err != nil {
			panic(err.Error())
		} else if doc != nil {
			docObject := &unstructured.Unstructured{Object: doc.Body}
			docRv := docObject.GetResourceVersion()
			if docRv != rv {
				fmt.Printf("[!] ADD %s: conflict resourceVersion %#v != %#v\n", id, rv, docRv)
			} else {
				fmt.Printf("[!] ADD %s: existing resourceVersion %#v\n", id, docRv)
			}
			break // nothing to write
		}

		rsrc.Object["_id"] = id
		_, err := ka.db.Put(id, rsrc.Object)
		if status, ok := err.(*couchdb.StatusObject); ok {
			fmt.Printf("[!] ADD %s: put %s\n", id, status.Status)
		} else if err != nil {
			panic(err.Error())
		}

	case cache.Updated, cache.Sync:
		put := rsrc.DeepCopy().Object
		put["_id"] = id

		if doc, err := ka.db.GetOrNil(id); err != nil {
			panic(err.Error())
		} else if doc == nil {
			fmt.Printf("[~] %s %s: new document\n", action, id)
		} else {
			put["_rev"] = doc.Body["_rev"]

			docObject := &unstructured.Unstructured{Object: doc.Body}
			docRv := docObject.GetResourceVersion()
			if parseRv(rv) < parseRv(docRv) {
				fmt.Printf("[!] %s %s: conflict resourceVersion %#v < %#v\n", action, id, rv, docRv)
				break // old version, don't overwrite
			} else if rv == docRv {
				break // same version, don't overwrite
			}
		}

		_, err = ka.db.Put(id, put)
		if status, ok := err.(*couchdb.StatusObject); ok {
			fmt.Printf("[!] %s %s: put %s\n", action, id, status.Status)
		} else if err != nil {
			panic(err.Error())
		}

	case cache.Deleted:
		if doc, err := ka.db.GetOrNil(id); err != nil {
			panic(err.Error())
		} else if doc != nil {
			if _, err := ka.db.Delete(doc.Body); err != nil {
				fmt.Printf("[!] DELETE %s: %s\n", id, err.Error())
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
