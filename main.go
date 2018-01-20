package main

import (
	"github.com/slushie/kubist-agent/kubernetes"
	"github.com/slushie/kubist-agent/couchdb"
	"k8s.io/client-go/dynamic"
	"os"
)

func main() {
	kubeConfig, err := kubernetes.NewClientConfig("", nil)
	if err != nil {
		panic(err.Error())
	}

	pool := dynamic.NewDynamicClientPool(kubeConfig)

	cc, err := couchdb.NewClient(
		"http://localhost:5984/",
		&couchdb.Auth{"admin", "admin"},
	)

	host, err := os.Hostname()
	if err != nil {
		panic(err.Error())
	}

	db := cc.Database("kubist-agent/" + host)

	RunAgent(db, pool)
}
