package kubernetes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	r "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	client "k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
)

func init() {
	// panic on list/watch errors
	r.ErrorHandlers = append(r.ErrorHandlers, func(err error) {
		panic(err.Error())
	})
}

type ResourceWatcher struct {
	ctr  cache.Controller
	stop chan struct{}
	ch   chan cache.Delta
}

func NewResourceWatcher(
	c client.Interface,
	resourceName string,
	namespace string,
) *ResourceWatcher {
	ns := true
	if namespace == "" {
		ns = false
	}

	rc := c.Resource(&metav1.APIResource{
		Name:       resourceName,
		Namespaced: ns,
	}, namespace)

	lw := &cache.ListWatch{
		ListFunc: func(o metav1.ListOptions) (runtime.Object, error) {
			return rc.List(o)
		},
		WatchFunc: func(o metav1.ListOptions) (watch.Interface, error) {
			return rc.Watch(o)
		},
	}

	ch := make(chan cache.Delta)
	fifo := cache.NewDeltaFIFO(cache.MetaNamespaceKeyFunc, nil, nil)

	ctr := cache.New(&cache.Config{
		Queue:         fifo,
		ListerWatcher: lw,
		Process: func(o interface{}) error {
			for _, d := range o.(cache.Deltas) {
				ch <- d
			}

			return nil
		},
	})

	stop := make(chan struct{})
	return &ResourceWatcher{ctr: ctr, stop: stop, ch: ch}
}

func (rw *ResourceWatcher) Watch() <-chan cache.Delta {
	go rw.ctr.Run(rw.stop)
	return rw.ch
}

func (rw *ResourceWatcher) Stop() {
	var s struct{}
	rw.stop <- s
}
