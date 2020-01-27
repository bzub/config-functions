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
  init_job_enabled: "{{ .Data.InitJobEnabled }}"
  unseal_job_enabled: "{{ .Data.UnsealJobEnabled }}"
  tls_generator_job_enabled: "{{ .Data.TLSGeneratorJobEnabled }}"
  unseal_secret_name: "{{ .Data.UnsealSecretName }}"
`

// FunctionConfig holds information used in Resource templates. It is a Go
// representation of a Kubernetes ConfigMap Resource.
type FunctionConfig struct {
	// ObjectMeta contains Resource metadata to use in templates.
	//
	// The following information from the function config are applied to
	// all Resource configs the function manages/generates:
	// - `metadata.name` (Used as a prefix for Resource names.)
	// - `metadata.namespace`
	//
	// In addition, the function sets the following labels on Resource
	// configs:
	// - `app.kubernetes.io/name` (Defaults to `vault-server`.)
	// - `app.kubernetes.io/instance` (Default is the value of `metadata.name`)
	yaml.ObjectMeta `yaml:"metadata"`

	// Data contains various options specific to this config function.
	Data FunctionData
}

// FunctionData holds settings used in the config function.
type FunctionData struct {
	// InitJobEnabled creates a Job which performs "vault operator init" on
	// a new Vault cluster, and stores unseal keys in a Secret.
	InitJobEnabled bool `yaml:"init_job_enabled"`

	// UnsealJobEnabled creates a Job which performs "vault operator unseal"
	// on a Vault cluster.
	UnsealJobEnabled bool `yaml:"unseal_job_enabled"`

	// TLSGeneratorJobEnabled creates Jobs which generate TLS assets for
	// communication with Vault.
	TLSGeneratorJobEnabled bool `yaml:"tls_generator_job_enabled"`

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
		case key == "init_job_enabled" && value == "true":
			d.InitJobEnabled = true
		case key == "unseal_job_enabled" && value == "true":
			d.UnsealJobEnabled = true
		case key == "tls_generator_job_enabled" && value == "true":
			d.TLSGeneratorJobEnabled = true
		case key == "unseal_secret_name":
			d.UnsealSecretName = value
		}

		key = ""
	}

	return nil
}
