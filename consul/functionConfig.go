package main

import (
	"strconv"

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
  replicas: "{{ .Data.Replicas }}"
  agent_tls_enabled: "{{ .Data.AgentTLSEnabled }}"
  gossip_enabled: "{{ .Data.GossipEnabled }}"
  acl_bootstrap_enabled: "{{ .Data.ACLBootstrapEnabled }}"
  agent_sidecar_injector_enabled: "{{ .Data.AgentSidecarInjectorEnabled }}"
  agent_tls_server_secret_name: "{{ .Data.AgentTLSServerSecretName }}"
  agent_tls_ca_secret_name: "{{ .Data.AgentTLSCASecretName }}"
  agent_tls_cli_secret_name: "{{ .Data.AgentTLSCLISecretName }}"
  gossip_secret_name: "{{ .Data.GossipSecretName }}"
  acl_bootstrap_secret_name: "{{ .Data.ACLBootstrapSecretName }}"
`

// functionConfig holds information used in Resource templates.
type functionConfig struct {
	// ObjectMeta contains Resource metadata to use in templates.
	yaml.ObjectMeta `yaml:"metadata"`

	Data functionData
}

type functionData struct {
	// AgentTLSEnabled creates a Job which populates a Secret with Consul
	// agent TLS assests, and configures a Consul StatefulSet to use said
	// Secret.
	AgentTLSEnabled bool `yaml:"agent_tls_enabled,omitempty"`

	// GossipEnabled creates a Job which creates a Consul gossip encryption
	// key Secret, and configures a Consul StatefulSet to use said
	// key/Secret.
	GossipEnabled bool `yaml:"gossip_enabled,omitempty"`

	// ACLBootstrapEnabled creates a Job (and associated resources) which
	// executes `consul acl bootstrap` on a new Consul cluster, and stores
	// the bootstrap token information in a Secret.
	ACLBootstrapEnabled bool `yaml:"acl_bootstrap_enabled,omitempty"`

	// AgentSidecarInjectorEnabled adds a Consul Agent sidecar container to
	// workload configs that contain the
	// `config.bzub.dev/consul-agent-sidecar-injector` annotation with a
	// value that targets the desired Consul server instance.
	AgentSidecarInjectorEnabled bool `yaml:"agent_sidecar_injector_enabled,omitempty"`

	// AgentTLSServerSecretName is the name of the Secret used to hold
	// Consul server TLS assets.
	AgentTLSServerSecretName string `yaml:"agent_tls_server_secret_name,omitempty"`

	// AgentTLSCASecretName is the name of the Secret used to hold
	// Consul CA certificates.
	AgentTLSCASecretName string `yaml:"agent_tls_ca_secret_name,omitempty"`

	// AgentTLSCLISecretName is the name of the Secret used to hold
	// Consul CLI TLS assets.
	AgentTLSCLISecretName string `yaml:"agent_tls_cli_secret_name,omitempty"`

	// GossipSecretName is the name of the Secret used to hold the Consul
	// gossip encryption key/config.
	GossipSecretName string `yaml:"gossip_secret_name,omitempty"`

	// ACLBootstrapSecretName is the name of the Secret used to hold Consul
	// cluster ACL bootstrap information.
	ACLBootstrapSecretName string `yaml:"acl_bootstrap_secret_name,omitempty"`

	// Replicas is the number of configured Consul server replicas. Used
	// for other options like --bootstrap-expect.
	Replicas int `yaml:"replicas,omitempty"`
}

func (d *functionData) UnmarshalYAML(node *yaml.Node) error {
	var err error
	var key, value string
	for i := range node.Content {
		if key == "" {
			key = node.Content[i].Value
			continue
		}
		value = node.Content[i].Value

		// Convert KV string values into associated functionData types.
		switch {
		case key == "agent_tls_enabled" && value == "true":
			d.AgentTLSEnabled = true
		case key == "gossip_enabled" && value == "true":
			d.GossipEnabled = true
		case key == "acl_bootstrap_enabled" && value == "true":
			d.ACLBootstrapEnabled = true
		case key == "agent_sidecar_injector_enabled" && value == "true":
			d.AgentSidecarInjectorEnabled = true
		case key == "agent_tls_server_secret_name":
			d.AgentTLSServerSecretName = value
		case key == "agent_tls_ca_secret_name":
			d.AgentTLSCASecretName = value
		case key == "agent_tls_cli_secret_name":
			d.AgentTLSCLISecretName = value
		case key == "gossip_secret_name":
			d.GossipSecretName = value
		case key == "acl_bootstrap_secret_name":
			d.ACLBootstrapSecretName = value
		case key == "replicas":
			d.Replicas, err = strconv.Atoi(value)
			if err != nil {
				return err
			}
		}

		key = ""
	}

	return nil
}
