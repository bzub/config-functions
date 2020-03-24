package consul

import (
	"github.com/bzub/config-functions/cfunc"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

const DefaultAppNameAnnotationValue = "consul-server"

const functionCMTemplate = `apiVersion: v1
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
  backup_cron_job_enabled: "{{ .Data.BackupCronJobEnabled }}"
  backup_secret_name: "{{ .Data.BackupSecretName }}"
  restore_secret_name: "{{ .Data.RestoreSecretName }}"
  tls_generator_job_enabled: "{{ .Data.TLSGeneratorJobEnabled }}"
  gossip_key_generator_job_enabled: "{{ .Data.GossipKeyGeneratorJobEnabled }}"
  acl_bootstrap_secret_name: "{{ .Data.ACLBootstrapSecretName }}"
  tls_server_secret_name: "{{ .Data.TLSServerSecretName }}"
  tls_ca_secret_name: "{{ .Data.TLSCASecretName }}"
  tls_cli_secret_name: "{{ .Data.TLSCLISecretName }}"
  tls_client_secret_name: "{{ .Data.TLSClientSecretName }}"
  gossip_secret_name: "{{ .Data.GossipSecretName }}"
`

// ConfigFunction implements kio.Filter and holds information used in Resource
// templates.
type ConfigFunction struct {
	cfunc.ConfigFunction `yaml:",inline"`

	// Data contains various options specific to this config function.
	Data Options
}

// Options holds settings used in the config function.
type Options struct {
	// ACLBootstrapJobEnabled creates a Job which executes `consul acl
	// bootstrap` on a new Consul cluster, and stores the bootstrap token
	// information in a Secret.
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

	// BackupCronJobEnabled adds a CronJob that runs `consul snapshot save`
	// on the cluster periodically.
	//
	// https://www.consul.io/docs/commands/snapshot/save.html
	BackupCronJobEnabled bool `json:"backup_cron_job_enabled"`

	// TLSGeneratorJobEnabled creates a Job which generates TLS assets for
	// Consul communication, and stores them in Secrets.
	//
	// https://learn.hashicorp.com/consul/security-networking/certificates
	TLSGeneratorJobEnabled bool `yaml:"tls_generator_job_enabled"`

	// GossipKeyGeneratorJobEnabled creates a Job which generates a Consul
	// gossip encryption key Secret.
	//
	// https://learn.hashicorp.com/consul/security-networking/agent-encryption
	GossipKeyGeneratorJobEnabled bool `yaml:"gossip_key_generator_job_enabled"`

	// ACLBootstrapSecretName is the name of the Secret used to hold Consul
	// cluster ACL bootstrap information.
	ACLBootstrapSecretName string `yaml:"acl_bootstrap_secret_name"`

	// BackupSecretName is the name of the Secret used to hold a backup of
	// the Consul k8s secrets and database.
	BackupSecretName string `json:"backup_secret_name"`

	// RestoreSecretName is the name of the Secret that restore Jobs will
	// look for to restore from backups.
	RestoreSecretName string `json:"restore_secret_name"`

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

// casiConfig holds information used to patch workload Resources with a sidecar
// continer.
type casiConfig struct {
	// PatchTarget contains Resource metadata from a workload to be
	// patched.
	PatchTarget yaml.ResourceMeta

	// FunctionConfig contains information used to configure the Consul
	// agent sidecar.
	*ConfigFunction
}

// Filter generates Resources.
func (f *ConfigFunction) Filter(in []*yaml.RNode) ([]*yaml.RNode, error) {
	// Workaround single line style of function config.
	if err := cfunc.FixStyles(in...); err != nil {
		return nil, err
	}

	// Get data for templates.
	if err := f.syncData(in); err != nil {
		return nil, err
	}

	// Generate a ConfigMap from the function config.
	fnConfigMap, err := cfunc.ParseTemplate("function-cm", functionCMTemplate, f)
	if err != nil {
		return nil, err
	}

	// Start building our generated Resource slice.
	generatedRs := []*yaml.RNode{fnConfigMap}

	// Generate Consul server Resources from templates.
	serverRs, err := cfunc.ParseTemplates(serverTemplates(), f)
	if err != nil {
		return nil, err
	}
	generatedRs = append(generatedRs, serverRs...)

	if f.Data.GossipKeyGeneratorJobEnabled {
		// Generate gossip Resouces from templates.
		gossipRs, err := cfunc.ParseTemplates(gossipTemplates(), f)
		if err != nil {
			return nil, err
		}
		generatedRs = append(generatedRs, gossipRs...)
	}

	if f.Data.TLSGeneratorJobEnabled {
		// Generate agent TLS Resources from templates.
		tlsRs, err := cfunc.ParseTemplates(tlsTemplates(), f)
		if err != nil {
			return nil, err
		}
		generatedRs = append(generatedRs, tlsRs...)
	}

	if f.Data.ACLBootstrapJobEnabled {
		// Generate ACL bootstrap Resources from templates.
		aclRs, err := cfunc.ParseTemplates(aclJobTemplates(), f)
		if err != nil {
			return nil, err
		}
		generatedRs = append(generatedRs, aclRs...)
	}

	if f.Data.AgentSidecarInjectorEnabled {
		// Generate sidecar patch Resources for workloads that call for
		// them from input.
		sidecarRs, err := f.sidecarPatches(in)
		if err != nil {
			return nil, err
		}
		generatedRs = append(generatedRs, sidecarRs...)
	}

	if f.Data.BackupCronJobEnabled {
		// Generate backup CronJob Resources from templates.
		backupRs, err := cfunc.ParseTemplates(backupCronJobTemplates(), f)
		if err != nil {
			return nil, err
		}
		generatedRs = append(generatedRs, backupRs...)
	}

	// Return the generated resources + patches + input.
	return append(generatedRs, in...), nil
}

// syncData populates a struct with information needed for Resource templates.
func (f *ConfigFunction) syncData(in []*yaml.RNode) error {
	if err := f.SyncMetadata(DefaultAppNameAnnotationValue); err != nil {
		return err
	}

	fnMeta, err := f.RW.FunctionConfig.GetMeta()
	if err != nil {
		return err
	}

	// Set defaults.
	f.Data = Options{
		ACLBootstrapSecretName: fnMeta.Name + "-" + fnMeta.Namespace + "-acl",
		TLSServerSecretName:    fnMeta.Name + "-" + fnMeta.Namespace + "-tls-server",
		TLSCASecretName:        fnMeta.Name + "-" + fnMeta.Namespace + "-tls-ca",
		TLSCLISecretName:       fnMeta.Name + "-" + fnMeta.Namespace + "-tls-cli",
		TLSClientSecretName:    fnMeta.Name + "-" + fnMeta.Namespace + "-tls-client",
		GossipSecretName:       fnMeta.Name + "-" + fnMeta.Namespace + "-gossip",
		BackupSecretName:       fnMeta.Name + "-" + fnMeta.Namespace + "-backup",
		RestoreSecretName:      fnMeta.Name + "-" + fnMeta.Namespace + "-restore",
	}

	// Populate function data from config.
	if err := yaml.Unmarshal([]byte(f.RW.FunctionConfig.MustString()), f); err != nil {
		return err
	}

	return nil
}

// UnmarshalYAML implements yaml.Unmarshaler. It ensures all values from
// a ConfigMap's KV data can be converted into relevant Go types.
func (d *Options) UnmarshalYAML(node *yaml.Node) error {
	var key, value string
	for i := range node.Content {
		if key == "" {
			key = node.Content[i].Value
			continue
		}
		value = node.Content[i].Value

		// Convert KV string values into associated Options types.
		switch {
		case key == "acl_bootstrap_job_enabled" && value == "true":
			d.ACLBootstrapJobEnabled = true
		case key == "agent_sidecar_injector_enabled" && value == "true":
			d.AgentSidecarInjectorEnabled = true
		case key == "backup_cron_job_enabled" && value == "true":
			d.BackupCronJobEnabled = true
		case key == "tls_generator_job_enabled" && value == "true":
			d.TLSGeneratorJobEnabled = true
		case key == "gossip_key_generator_job_enabled" && value == "true":
			d.GossipKeyGeneratorJobEnabled = true
		case key == "acl_bootstrap_secret_name":
			d.ACLBootstrapSecretName = value
		case key == "backup_secret_name":
			d.BackupSecretName = value
		case key == "restore_secret_name":
			d.RestoreSecretName = value
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
