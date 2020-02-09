package etcd

import (
	"fmt"
	"strings"

	"github.com/bzub/config-functions/cfssl"
	"github.com/bzub/config-functions/cfunc"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

const DefaultAppNameAnnotationValue = "etcd-server"

const functionCMTemplate = `apiVersion: v1
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

// ConfigFunction implements kio.Filter and holds information used in Resource
// templates.
type ConfigFunction struct {
	cfunc.ConfigFunction `yaml:",inline"`

	// Data contains various options specific to this config function.
	Data Options

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
	Hostnames []string
}

// Options holds settings used in the config function.
type Options struct {
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

	// Generate Etcd server Resources from templates.
	serverRs, err := cfunc.ParseTemplates(serverTemplates(), f)
	if err != nil {
		return nil, err
	}
	generatedRs = append(generatedRs, serverRs...)

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

		// Generate TLS Job Resources from templates.
		tlsRs, err := cfunc.ParseTemplates(tlsTemplates(), f)
		if err != nil {
			return nil, err
		}
		generatedRs = append(generatedRs, tlsRs...)
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

	f.InitialCluster, err = f.getInitialCluster(in)
	if err != nil {
		return err
	}

	f.Hostnames, err = cfunc.GetStatefulSetHostnames(in, fnMeta.Name+"-server", fnMeta.Namespace)
	if err != nil {
		return err
	}

	// Set defaults.
	f.Data = Options{
		TLSServerSecretName:     fnMeta.Name + "-" + fnMeta.Namespace + "-tls-server",
		TLSCASecretName:         fnMeta.Name + "-" + fnMeta.Namespace + "-tls-ca",
		TLSRootClientSecretName: fnMeta.Name + "-" + fnMeta.Namespace + "-tls-client-root",
	}

	// Populate function data from config.
	if err := yaml.Unmarshal([]byte(f.RW.FunctionConfig.MustString()), f); err != nil {
		return err
	}

	return nil
}

func (f *ConfigFunction) getInitialCluster(in []*yaml.RNode) (string, error) {
	fnMeta, err := f.RW.FunctionConfig.GetMeta()
	if err != nil {
		return "", err
	}

	names, err := cfunc.GetStatefulSetHostnames(in, fnMeta.Name+"-server", fnMeta.Namespace)
	if err != nil {
		return "", err
	}

	for i := range names {
		names[i] = fmt.Sprintf("%s=https://%s.%s-server:2380", names[i], names[i], fnMeta.Name)
	}

	return strings.Join(names, ","), nil
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
