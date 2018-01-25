# kubist-agent

Kubist Agent reflects Kubernetes resources to CouchDB.

## Usage

```
$ kubist-agent --help
Kubist Agent reflects Kubernetes resources to CouchDB.

This daemon can run in-cluster or on your workstation. Because it
inherits most global flags from kubectl, all of the agent's Kubernetes
server configuration can be done through command-line flags.

Environment variables and configuration files may be used to configure
this daemon. Environment variables are noted in the --help output below,
but generally follow the pattern --some-option => SOME_OPTION. Configuration
is read from kubist.json in the current directory, ~/.config, or /etc/kubist,
in that order.

Usage:
  kubist-agent [flags]

Examples:
  kubist-agent --context minikube
Connect to the "minikube" context in your ~/.kube/config file.

  kubist-agent --in-cluster --couchdb-url http://couchdb.kubist:5984/
Connect to Kubernetes from a Pod within the cluster, and to the CouchDB
service in the "kubist" namespace of the current cluster.


Flags:
  -P, --couchdb-password string             Password for CouchDB authentication [COUCHDB_PASSWORD]
  -p, --couchdb-read-password               Read CouchDB password from stdin
  -u, --couchdb-url string                  Base URL for CouchDB [COUCHDB_URL] (default "http://localhost:5984")
  -U, --couchdb-username string             Username for CouchDB authentication [COUCHDB_USERNAME]
  -h, --help                                help for kubist-agent
  -C, --in-cluster                          Look for in-cluster configuration. Does not load a kubeconfig
      --kube-as string                      Username to impersonate for the operation
      --kube-as-group stringArray           Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --kube-certificate-authority string   Path to a cert file for the certificate authority
      --kube-client-certificate string      Path to a client certificate file for TLS
      --kube-client-key string              Path to a client key file for TLS
      --kube-cluster string                 The name of the kubeconfig cluster to use
      --kube-context string                 The name of the kubeconfig context to use
      --kube-insecure-skip-tls-verify       If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
  -n, --kube-namespace string               If present, the namespace scope for this CLI request
      --kube-password string                Password for basic authentication to the API server
      --kube-request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
      --kube-server string                  The address and port of the Kubernetes API server
      --kube-token string                   Bearer token for authentication to the API server
      --kube-user string                    The name of the kubeconfig user to use
      --kube-username string                Username for basic authentication to the API server
  -f, --kubeconfig string                   Path to your Kubeconfig [KUBECONFIG]
      --recreate-database                   Drop and recreate the CouchDB database. WARNING: This may break replication
```