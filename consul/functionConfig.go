package consul

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
  acl_bootstrap_job_enabled: "{{ .Data.ACLBootstrapJobEnabled }}"
  agent_sidecar_injector_enabled: "{{ .Data.AgentSidecarInjectorEnabled }}"
  tls_generator_job_enabled: "{{ .Data.TLSGeneratorJobEnabled }}"
  gossip_key_generator_job_enabled: "{{ .Data.GossipKeyGeneratorJobEnabled }}"
  acl_bootstrap_secret_name: "{{ .Data.ACLBootstrapSecretName }}"
  tls_server_secret_name: "{{ .Data.TLSServerSecretName }}"
  tls_ca_secret_name: "{{ .Data.TLSCASecretName }}"
  tls_cli_secret_name: "{{ .Data.TLSCLISecretName }}"
  tls_client_secret_name: "{{ .Data.TLSClientSecretName }}"
  gossip_secret_name: "{{ .Data.GossipSecretName }}"
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
	// - `app.kubernetes.io/name` (Defaults to `consul-server`.)
	// - `app.kubernetes.io/instance` (Default is the value of `metadata.name`)
	yaml.ObjectMeta `yaml:"metadata"`

	// Data contains various options specific to this config function.
	Data FunctionData
}

// FunctionData holds settings used in the config function.
type FunctionData struct {
	// ACLBootstrapJobEnabled creates a Job (and associated resources)
	// which executes `consul acl bootstrap` on a new Consul cluster, and
	// stores the bootstrap token information in a Secret.
	//
	// https://learn.hashicorp.com/consul/day-0/acl-guide
	ACLBootstrapJobEnabled bool `yaml:"acl_bootstrap_job_enabled"`

	// AgentSidecarInjectorEnabled adds a Consul Agent sidecar container to
	// workload configs that contain the
	// `config.bzub.dev/consul-agent-sidecar-injector` annotation with a
	// value that targets the desired Consul server instance.
	//
	// https://www.consul.io/docs/agent/basics.html
	AgentSidecarInjectorEnabled bool `yaml:"agent_sidecar_injector_enabled"`

	// TLSGeneratorJobEnabled creates a Job which populates a Secret with
	// Consul agent TLS assests, and configures a Consul StatefulSet to use
	// said Secret.
	//
	// https://learn.hashicorp.com/consul/security-networking/certificates
	TLSGeneratorJobEnabled bool `yaml:"tls_generator_job_enabled"`

	// GossipKeyGeneratorJobEnabled creates a Job which creates a Consul
	// gossip encryption key Secret, and configures a Consul StatefulSet to
	// use said key/Secret.
	//
	// https://learn.hashicorp.com/consul/security-networking/agent-encryption
	GossipKeyGeneratorJobEnabled bool `yaml:"gossip_key_generator_job_enabled"`

	// ACLBootstrapSecretName is the name of the Secret used to hold Consul
	// cluster ACL bootstrap information.
	ACLBootstrapSecretName string `yaml:"acl_bootstrap_secret_name"`

	// TLSServerSecretName is the name of the Secret used to hold Consul
	// server TLS assets.
	TLSServerSecretName string `yaml:"tls_server_secret_name"`

	// TLSCASecretName is the name of the Secret used to hold Consul CA
	// certificates.
	TLSCASecretName string `yaml:"tls_ca_secret_name"`

	// TLSCLISecretName is the name of the Secret used to hold Consul CLI
	// TLS assets.
	TLSCLISecretName string `yaml:"tls_cli_secret_name"`

	// TLSClientSecretName is the name of the Secret used to hold Consul
	// Client TLS assets.
	TLSClientSecretName string `yaml:"tls_client_secret_name"`

	// GossipSecretName is the name of the Secret used to hold the Consul
	// gossip encryption key/config.
	GossipSecretName string `yaml:"gossip_secret_name"`
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
		case key == "acl_bootstrap_job_enabled" && value == "true":
			d.ACLBootstrapJobEnabled = true
		case key == "agent_sidecar_injector_enabled" && value == "true":
			d.AgentSidecarInjectorEnabled = true
		case key == "tls_generator_job_enabled" && value == "true":
			d.TLSGeneratorJobEnabled = true
		case key == "gossip_key_generator_job_enabled" && value == "true":
			d.GossipKeyGeneratorJobEnabled = true
		case key == "acl_bootstrap_secret_name":
			d.ACLBootstrapSecretName = value
		case key == "tls_server_secret_name":
			d.TLSServerSecretName = value
		case key == "tls_ca_secret_name":
			d.TLSCASecretName = value
		case key == "tls_cli_secret_name":
			d.TLSCLISecretName = value
		case key == "tls_client_secret_name":
			d.TLSClientSecretName = value
		case key == "gossip_secret_name":
			d.GossipSecretName = value
		}

		key = ""
	}

	return nil
}

// casiConfig holds information used to patch workload Resources with a sidecar
// continer.
type casiConfig struct {
	// PatchTarget contains Resource metadata from a workload to be
	// patched.
	PatchTarget yaml.ResourceMeta

	// FunctionConfig contains information used to configure the Consul
	// agent sidecar.
	*FunctionConfig
}
