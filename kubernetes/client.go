package kubernetes

import (
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func NewClientConfig(path string, overrides *clientcmd.ConfigOverrides) (*rest.Config, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.ExplicitPath = path

	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
	return loader.ClientConfig()
}
