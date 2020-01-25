package vault

import (
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

var functionCMTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Name }}
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
data:
  init_enabled: "{{ .Data.InitEnabled }}"
  unseal_enabled: "{{ .Data.UnsealEnabled }}"
  generate_tls_enabled: "{{ .Data.GenerateTLSEnabled }}"
  unseal_secret_name: "{{ .Data.UnsealSecretName }}"
`

// FunctionConfig holds information used in Resource templates. It is a Go
// representation of a Kubernetes ConfigMap Resource.
type FunctionConfig struct {
	// ObjectMeta contains Resource metadata to use in templates.
	yaml.ObjectMeta `yaml:"metadata"`

	Data FunctionData
}

// FunctionData holds settings used in the config function.
type FunctionData struct {
	// InitEnabled creates a Job which performs "vault operator init" on a
	// new Vault cluster, and stores unseal keys in a Secret.
	InitEnabled bool `yaml:"init_enabled"`

	// UnsealEnabled creates a Job which performs "vault operator unseal"
	// on a Vault cluster.
	UnsealEnabled bool `yaml:"unseal_enabled"`

	// GenerateTLSEnabled creates Jobs which generate TLS assets for
	// communication with Vault.
	GenerateTLSEnabled bool `yaml:"generate_tls_enabled"`

	// UnsealSecretName is the name of the Secret used to hold unseal key
	// shares.
	UnsealSecretName string `yaml:"unseal_secret_name"`
}

// UnmarshalYAML implements yaml.Unmarshaler. It ensures all values from
// a ConfigMap's KV data can be converted into relevant Go types.
func (d *FunctionData) UnmarshalYAML(node *yaml.Node) error {
	var key, value string
	for i := range node.Content {
		if key == "" {
			key = node.Content[i].Value
			continue
		}
		value = node.Content[i].Value

		// Convert KV string values into associated FunctionData types.
		switch {
		case key == "init_enabled" && value == "true":
			d.InitEnabled = true
		case key == "unseal_enabled" && value == "true":
			d.UnsealEnabled = true
		case key == "generate_tls_enabled" && value == "true":
			d.GenerateTLSEnabled = true
		case key == "unseal_secret_name":
			d.UnsealSecretName = value
		}

		key = ""
	}

	return nil
}
