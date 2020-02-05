package etcd

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
  tls_generator_job_enabled: "{{ .Data.TLSGeneratorJobEnabled }}"
  tls_server_secret_name: "{{ .Data.TLSServerSecretName }}"
  tls_ca_secret_name: "{{ .Data.TLSCASecretName }}"
  tls_root_client_secret_name: "{{ .Data.TLSRootClientSecretName }}"
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
	// - `app.kubernetes.io/name` (Defaults to `etcd-server`.)
	// - `app.kubernetes.io/instance` (Default is the value of `metadata.name`)
	yaml.ObjectMeta `yaml:"metadata"`

	// Data contains various options specific to this config function.
	Data FunctionData
}

// FunctionData holds settings used in the config function.
type FunctionData struct {
	// TLSGeneratorJobEnabled creates Jobs which generate TLS assets for
	// communication with Etcd.
	TLSGeneratorJobEnabled bool `yaml:"tls_generator_job_enabled"`

	// TLSServerSecretName is the name of the Secret used to hold Etcd
	// server TLS assets.
	TLSServerSecretName string `yaml:"tls_server_secret_name"`

	// TLSCASecretName is the name of the Secret used to hold Etcd CA TLS
	// assets.
	TLSCASecretName string `yaml:"tls_ca_secret_name"`

	// TLSRootClientSecretName is the name of the Secret used to hold Etcd
	// root user TLS assets.
	TLSRootClientSecretName string `yaml:"tls_root_client_secret_name"`

	// InitialCluster is used to configure etcd's `initial-cluster`
	// setting. By default the function detects the number of StatefulSet
	// replicas from an existing Resource config to generate this value.
	//
	// InitialCluster is exposed in the server ConfigMap as
	// `ETCD_INITIAL_CLUSTER` rather than the function ConfigMap.
	//
	// https://github.com/etcd-io/etcd/blob/master/Documentation/op-guide/configuration.md#--initial-cluster
	InitialCluster string

	// Hostnames are the names of the pods that will be created by the
	// StatefulSet. It is updated when the StatefulSet's `spec.replicas`
	// changes.
	//
	// Hostnames is only used in Go, and not exposed via YAML config(s).
	Hostnames []string
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
		case key == "tls_generator_job_enabled" && value == "true":
			d.TLSGeneratorJobEnabled = true
		case key == "tls_server_secret_name":
			d.TLSServerSecretName = value
		case key == "tls_ca_secret_name":
			d.TLSCASecretName = value
		case key == "tls_root_client_secret_name":
			d.TLSRootClientSecretName = value
		}

		key = ""
	}

	return nil
}
