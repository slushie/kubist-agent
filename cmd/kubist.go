package cmd

import (
	"k8s.io/client-go/dynamic"
	"github.com/slushie/kubist-agent/couchdb"
	"os"
	"strings"
	"fmt"
	"github.com/slushie/kubist-agent/kubernetes"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/rest"
)

var rootCmd = &cobra.Command{
	Use:   "kubist-agent",
	Args:  cobra.NoArgs,
	Short: "Kubist Agent reflects Kubernetes resources to CouchDB",
	Long: `Kubist Agent reflects Kubernetes resources to CouchDB.

This daemon can run in-cluster or on your workstation. Because it 
inherits most global flags from kubectl, all of the agent's Kubernetes 
server configuration can be done through command-line flags.
`,
	Example: `  kubist-agent --context minikube
Connect to the "minikube" context in your ~/.kube/config file.

  kubist-agent --in-cluster --couchdb-url http://couchdb.kubist:5984/
Connect to Kubernetes from a Pod within the cluster, and to the CouchDB 
service in the "kubist" namespace of the current cluster.
`,
	Run: execute,
}

var overrides = &clientcmd.ConfigOverrides{}
var DefaultCouchDbUrl = "http://localhost:5984"

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringP(
		"kubeconfig",
		"f",
		"",
		"Path to your Kubeconfig [KUBECONFIG]",
	)

	rootCmd.Flags().BoolP(
		"in-cluster",
		"C",
		false,
		"Look for in-cluster configuration. Does not load a kubeconfig",
	)

	rootCmd.Flags().StringP(
		"couchdb-url",
		"u",
		"http://localhost:5984/",
		"Base URL for CouchDB [COUCHDB_URL]",
	)

	rootCmd.Flags().StringP(
		"couchdb-username",
		"U",
		os.Getenv("COUCHDB_USERNAME"),
		"Username for CouchDB authentication [COUCHDB_USERNAME]",
	)

	rootCmd.Flags().StringP(
		"couchdb-password",
		"P",
		os.Getenv("COUCHDB_PASSWORD"),
		"Password for CouchDB authentication [COUCHDB_PASSWORD]",
	)

	initKubernetesOverrides()
}

func initKubernetesOverrides() {
	flagNames := clientcmd.RecommendedConfigOverrideFlags("kube-")
	clientcmd.BindOverrideFlags(
		overrides,
		rootCmd.Flags(),
		flagNames,
	)
}

func execute(cmd *cobra.Command, _ []string) {
	pool := createKubernetesClient(cmd)
	cc := createCouchDbClient(cmd)

	host, err := os.Hostname()
	if err != nil {
		panic(err.Error())
	}

	name := strings.Replace("kubist/"+host, ".", "_", -1)
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

	fmt.Printf("[+] Reflecting to DB %#v\n", name)
	RunAgent(db, pool)
}

func createCouchDbClient(cmd *cobra.Command) *couchdb.Client {
	var url = DefaultCouchDbUrl

	if flag := cmd.Flag("couchdb-url"); flag.Changed {
		url = flag.Value.String()
	} else if env, exists := os.LookupEnv("COUCHDB_URL"); exists {
		url = env
	}

	username, err := cmd.Flags().GetString("couchdb-username")
	if err != nil {
		panic(err.Error())
	}

	password, err := cmd.Flags().GetString("couchdb-password")
	if err != nil {
		panic(err.Error())
	}

	auth := &couchdb.Auth{username, password}
	cc, err := couchdb.NewClient(url, auth)
	if err != nil {
		panic(err.Error())
	}

	return cc
}

func createKubernetesClient(cmd *cobra.Command) dynamic.ClientPool {
	var kubeConfig *rest.Config

	if inCluster, err := cmd.Flags().GetBool("in-cluster"); err != nil {
		panic("in-cluster: " + err.Error())
	} else if inCluster {
		kubeConfig, err = rest.InClusterConfig()
		if err != nil {
			panic("in-cluster config: " + err.Error())
		}
	} else {
		var path string
		if flag := cmd.Flag("kubeconfig"); flag.Changed {
			path = flag.Value.String()
		} else if env, exists := os.LookupEnv("KUBECONFIG"); exists {
			path = env
		}

		kubeConfig, err = kubernetes.NewClientConfig(path, overrides)
		if err != nil {
			panic(err.Error())
		}
	}

	pool := dynamic.NewDynamicClientPool(kubeConfig)

	return pool
}
