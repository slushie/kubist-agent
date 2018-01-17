package kubernetes

import (
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/rest"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"github.com/spf13/pflag"
)

func NewClientConfig(path string, flags *pflag.FlagSet) (*rest.Config, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.ExplicitPath = path

	overrides := &clientcmd.ConfigOverrides{}

	if flags != nil {
		flagNames := clientcmd.RecommendedConfigOverrideFlags("")
		clientcmd.BindOverrideFlags(
			overrides,
			flags,
			flagNames,
		)
	}

	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
	return loader.ClientConfig()
}
