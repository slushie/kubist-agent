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
	"golang.org/x/crypto/ssh/terminal"
	"syscall"
	"bufio"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
var DefaultResources = []schema.GroupVersionResource{
	{"", "v1", "pods"},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(loadConfig)

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

	// import kubernetes flags
	flagNames := clientcmd.RecommendedConfigOverrideFlags("kube-")
	clientcmd.BindOverrideFlags(
		overrides,
		rootCmd.Flags(),
		flagNames,
	)

	viper.SetDefault("resources", DefaultResources)
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

	err := viper.ReadInConfig()
	if err == nil {
		fmt.Println("[~] Read config from " + viper.ConfigFileUsed())
	} else if _, notFound := err.(viper.ConfigFileNotFoundError); !notFound {
		panic("reading config: " + err.Error())
	}
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
		fmt.Println("[~] Creating database at " + name)
		if err = db.Create(); err != nil {
			panic(err.Error())
		}
	}

	resources := viper.Get("resources").([]schema.GroupVersionResource)

	fmt.Printf("[+] Reflecting %#v to DB %#v\n", resources, name)
	RunAgent(db, pool, resources)
}

func createCouchDbClient(cmd *cobra.Command) *couchdb.Client {
	url := viper.GetString("couchdb-url")
	username := viper.GetString("couchdb-username")

	var password string
	if viper.IsSet("couchdb-password") {
		password = viper.GetString("couchdb-password")
	} else if username != "" {
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

func promptForPassword(prompt string) (string, error) {
	stdin := syscall.Stdin
	if terminal.IsTerminal(stdin) {
		os.Stdin.WriteString(prompt + ": ")
		password, err := terminal.ReadPassword(stdin)
		os.Stdin.WriteString("\n")
		if err != nil {
			return "", err
		} else {
			return string(password), nil
		}
	} else {
		fmt.Println("[~] Not a tty. Reading " + prompt + " from stdin")

		reader := bufio.NewReader(os.Stdin)
		password, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		} else {
			return password, nil
		}
	}
}