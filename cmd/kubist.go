package cmd

import (
	"bufio"
	"fmt"
	"github.com/slushie/kubist-agent/couchdb"
	"github.com/slushie/kubist-agent/kubernetes"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh/terminal"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"strings"
	"syscall"
)

var rootCmd = &cobra.Command{
	Use:   "kubist-agent",
	Args:  cobra.NoArgs,
	Short: "Kubist Agent reflects Kubernetes resources to CouchDB",
	Long: `Kubist Agent reflects Kubernetes resources to CouchDB.

This daemon can run in-cluster or on your workstation. Because it 
inherits most global flags from kubectl, all of the agent's Kubernetes 
server configuration can be done through command-line flags.

Environment variables and configuration files may be used to configure 
this daemon. Environment variables are noted in the --help output below,
but generally follow the pattern --some-option => SOME_OPTION. Configuration
is read from kubist.json in the current directory, ~/.config, or /etc/kubist,
in that order.
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
var DefaultResources = []map[string]interface{}{
	{"group": "", "version": "v1", "resource": "pods"},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(loadConfig)

	// only overridden from the config file
	viper.SetDefault("resources", DefaultResources)

	rootCmd.Flags().Bool(
		"recreate-database",
		false,
		"Drop and recreate the CouchDB database. "+
			"WARNING: This may break replication",
	)

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
		DefaultCouchDbUrl,
		"Base URL for CouchDB [COUCHDB_URL]",
	)

	rootCmd.Flags().StringP(
		"couchdb-username",
		"U",
		"",
		"Username for CouchDB authentication [COUCHDB_USERNAME]",
	)

	rootCmd.Flags().StringP(
		"couchdb-password",
		"P",
		"",
		"Password for CouchDB authentication [COUCHDB_PASSWORD]",
	)

	rootCmd.Flags().BoolP(
		"couchdb-read-password",
		"p",
		false,
		"Read CouchDB password from stdin",
	)

	// import kubernetes flags
	flagNames := clientcmd.RecommendedConfigOverrideFlags("kube-")
	clientcmd.BindOverrideFlags(
		overrides,
		rootCmd.Flags(),
		flagNames,
	)
}

func loadConfig() {
	viper.SetConfigName("kubist")
	viper.AddConfigPath("/etc/kubist")
	viper.AddConfigPath("$HOME/.config")
	viper.AddConfigPath(".")

	if err := viper.BindPFlags(rootCmd.Flags()); err != nil {
		panic("binding flags: " + err.Error())
	}

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
}

func execute(cmd *cobra.Command, _ []string) {
	err := viper.ReadInConfig()
	if err == nil {
		fmt.Println("[~] Read config from " + viper.ConfigFileUsed())
	} else if _, notFound := err.(viper.ConfigFileNotFoundError); !notFound {
		panic("reading config: " + err.Error())
	}

	pool := createKubernetesClient(cmd)
	cc := createCouchDbClient(cmd)

	host, err := os.Hostname()
	if err != nil {
		panic(err.Error())
	}

	name := strings.Replace("kubist/"+host, ".", "_", -1)
	name = strings.ToLower(name)

	db := cc.Database(name)

	var exists bool
	recreateDatabase := viper.GetBool("recreate-database")
	if exists, err = db.Exists(); err != nil {
		panic(err.Error())
	} else if exists && recreateDatabase {
		fmt.Println("[+] Dropping database " + name)
		if err = db.Drop(); err != nil {
			panic(err.Error())
		}
	}

	if !exists || recreateDatabase {
		fmt.Println("[+] Creating database " + name)
		if err = db.Create(); err != nil {
			panic(err.Error())
		}
	}

	resources := make([]schema.GroupVersionResource, 0, 10)

	for _, in := range viper.Get("resources").([]interface{}) {
		r := in.(map[string]interface{})
		// group can be nil for core resources
		var group string
		if g, exists := r["group"]; exists {
			group = g.(string)
		}

		gvr := schema.GroupVersionResource{
			Group:    group,
			Version:  r["version"].(string),
			Resource: r["resource"].(string),
		}
		resources = append(resources, gvr)
	}

	namespace := viper.GetString("kube-namespace")

	nsDesc := "namespace " + namespace
	if namespace == "" {
		nsDesc = "all namespaces"
	}
	fmt.Printf("[+] Reflecting %+v in %s to database %#v\n",
		resources, nsDesc, name)

	agent := NewKubistAgent(db, pool, resources, namespace)
	agent.Run()
}

func createCouchDbClient(_ *cobra.Command) *couchdb.Client {
	url := viper.GetString("couchdb-url")
	username := viper.GetString("couchdb-username")
	password := viper.GetString("couchdb-password")

	if viper.GetBool("couchdb-read-password") {
		var err error
		password, err = promptForPassword("CouchDB password")
		if err != nil {
			panic(err.Error())
		}
	}

	auth := &couchdb.Auth{username, password}
	cc, err := couchdb.NewClient(url, auth)
	if err != nil {
		panic(err.Error())
	}

	return cc
}

func createKubernetesClient(_ *cobra.Command) dynamic.ClientPool {
	var (
		err        error
		kubeConfig *rest.Config
	)

	if viper.GetBool("in-cluster") {
		kubeConfig, err = rest.InClusterConfig()
		if err != nil {
			panic("in-cluster config failed: " + err.Error())
		}
	} else {
		path := viper.GetString("kubeconfig")
		kubeConfig, err = kubernetes.NewClientConfig(path, overrides)
		if err != nil {
			panic(fmt.Sprintf("kubeconfig %#v failed: %s",
				path, err.Error()))
		}
	}

	pool := dynamic.NewDynamicClientPool(kubeConfig)

	return pool
}

func promptForPassword(prompt string) (string, error) {
	stdin := syscall.Stdin
	if terminal.IsTerminal(stdin) {
		os.Stdin.WriteString("[?] " + prompt + ": ")
		password, err := terminal.ReadPassword(stdin)
		os.Stdin.WriteString("\n")
		if err != nil {
			return "", err
		} else {
			return string(password), nil
		}
	} else {
		fmt.Println("[?] Not a tty. Reading " + prompt + " from stdin")

		reader := bufio.NewReader(os.Stdin)
		password, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		} else {
			return password, nil
		}
	}
}
