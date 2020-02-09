package vault

import (
	"github.com/bzub/config-functions/cfssl"
	"github.com/bzub/config-functions/cfunc"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

const DefaultAppNameAnnotationValue = "vault-server"

const functionCMTemplate = `apiVersion: v1
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

// ConfigFunction implements kio.Filter and holds information used in Resource
// templates.
type ConfigFunction struct {
	cfunc.ConfigFunction `yaml:",inline"`

	// Data contains various options specific to this config function.
	Data Options

	// Hostnames are the names of the pods that will be created by the
	// StatefulSet. It is updated when the StatefulSet's `spec.replicas`
	// changes.
	Hostnames []string
}

// Options holds settings used in the config function.
type Options struct {
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

	// Generate Vault server Resources from templates.
	serverRs, err := cfunc.ParseTemplates(serverTemplates(), f)
	if err != nil {
		return nil, err
	}
	generatedRs = append(generatedRs, serverRs...)

	if f.Data.InitJobEnabled {
		// Generate init Job Resources from templates.
		initRs, err := cfunc.ParseTemplates(initJobTemplates(), f)
		if err != nil {
			return nil, err
		}
		generatedRs = append(generatedRs, initRs...)
	}

	if f.Data.UnsealJobEnabled {
		// Generate unseal Job Resources from templates.
		unsealRs, err := cfunc.ParseTemplates(unsealJobTemplates(), f)
		if err != nil {
			return nil, err
		}
		generatedRs = append(generatedRs, unsealRs...)
	}

	if f.Data.TLSGeneratorJobEnabled {
		// Create a cfssl function config from a ConfigMap template.
		cfsslFnCfg, err := cfunc.ParseTemplate("cfssl-cm", cfsslCMTemplate, f)
		if err != nil {
			return nil, err
		}

		// Create a cfssl filter.
		cfsslRW := &kio.ByteReadWriter{FunctionConfig: cfsslFnCfg}
		cfsslFunc := &cfssl.ConfigFunction{}
		cfsslFunc.RW = cfsslRW

		// Run the cfssl filter and use its Resources.
		cfsslRs, err := cfsslFunc.Filter([]*yaml.RNode{cfsslFnCfg})
		if err != nil {
			return nil, err
		}
		generatedRs = append(generatedRs, cfsslRs...)
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

	f.Hostnames, err = cfunc.GetStatefulSetHostnames(in, fnMeta.Name+"-server", fnMeta.Namespace)
	if err != nil {
		return err
	}

	// Set defaults.
	f.Data = Options{
		UnsealSecretName: fnMeta.Name + "-" + fnMeta.Namespace + "-unseal",
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
