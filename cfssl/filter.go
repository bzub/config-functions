package cfssl

import (
	"github.com/bzub/config-functions/cfunc"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

const DefaultAppNameAnnotationValue = "cfssl"

const functionCMTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Name }}
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
data:
  secret_name: "{{ .Data.SecretName }}"
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
	// SecretName is the name of the Secret used to hold generated certs
	// and keys. Defaults to the metadata.name of the function config.
	SecretName string `yaml:"secret_name"`
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

	// Generate cfssl Job Resources from templates.
	jobRs, err := cfunc.ParseTemplates(cfsslJobTemplates(), f)
	if err != nil {
		return nil, err
	}
	generatedRs = append(generatedRs, jobRs...)

	// Return the generated resources + patches + input.
	return append(generatedRs, in...), nil
}

// syncData populates a struct with information needed for Resource templates.
func (f *ConfigFunction) syncData(input []*yaml.RNode) error {
	if err := f.SyncMetadata(DefaultAppNameAnnotationValue); err != nil {
		return err
	}

	fnMeta, err := f.RW.FunctionConfig.GetMeta()
	if err != nil {
		return err
	}

	// Set defaults.
	f.Data = Options{
		SecretName: fnMeta.Name + "-" + fnMeta.Namespace,
	}

	return nil
}

func (d *Options) UnmarshalYAML(node *yaml.Node) error {
	var key, value string
	for i := range node.Content {
		if key == "" {
			key = node.Content[i].Value
			continue
		}
		value = node.Content[i].Value

		// Convert KV string values into associated FunctionData types.
		switch {
		case key == "secret_name":
			d.SecretName = value
		}

		key = ""
	}

	return nil
}
