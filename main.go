package main

import (
	"github.com/slushie/kubist-agent/kubernetes"
	"github.com/slushie/kubist-agent/couchdb"
	"k8s.io/client-go/dynamic"
	"os"
	"fmt"
	"strings"
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

	name := strings.Replace("kubist/" + host, ".", "_", -1)
	name = strings.ToLower(name)

	db := cc.Database(name)
	if exists, err := db.Exists(); err != nil {
		panic(err.Error())
	} else if !exists {
		fmt.Println("[ ] Creating database at " + name)
		if err = db.Create(); err != nil {
			panic(err.Error())
		}
	}

	RunAgent(db, pool)
}
