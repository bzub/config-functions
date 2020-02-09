package nodeexporter

import (
	"github.com/bzub/config-functions/cfunc"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

const DefaultAppNameAnnotationValue = "nodeexporter"

const functionCMTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Name }}
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
data:
`

// ConfigFunction implements kio.Filter and holds information used in
// Resource templates.
type ConfigFunction struct {
	cfunc.ConfigFunction `yaml:",inline"`
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

	// Generate NodeExporter server Resources from templates.
	serverRs, err := cfunc.ParseTemplates(serverTemplates(), f)
	if err != nil {
		return nil, err
	}
	generatedRs = append(generatedRs, serverRs...)

	// Return the generated resources + patches + input.
	return append(generatedRs, in...), nil
}

// syncData populates a struct with information needed for Resource templates.
func (f *ConfigFunction) syncData(in []*yaml.RNode) error {
	if err := f.SyncMetadata(DefaultAppNameAnnotationValue); err != nil {
		return err
	}

	return nil
}
