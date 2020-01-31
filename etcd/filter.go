package etcd

import (
	"fmt"
	"strings"

	"github.com/bzub/config-functions/cfssl"
	"github.com/bzub/config-functions/cfunc"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// EtcdFilter implements kio.Filter
type EtcdFilter struct {
	RW *kio.ByteReadWriter
}

// Filter generates Resources.
func (f *EtcdFilter) Filter(in []*yaml.RNode) ([]*yaml.RNode, error) {
	// Workaround single line style of function config.
	if err := cfunc.FixStyles(in...); err != nil {
		return nil, err
	}

	// Get data for templates.
	fnCfg, err := f.FunctionConfig(in)
	if err != nil {
		return nil, err
	}

	// Generate a ConfigMap from the function config.
	fnConfigMap, err := cfunc.ParseTemplate("function-cm", functionCMTemplate, fnCfg)
	if err != nil {
		return nil, err
	}

	// Start building our generated Resource slice.
	generatedRs := []*yaml.RNode{fnConfigMap}

	// Generate Etcd server Resources from templates.
	serverRs, err := cfunc.ParseTemplates(f.serverTemplates(), fnCfg)
	if err != nil {
		return nil, err
	}
	generatedRs = append(generatedRs, serverRs...)

	if fnCfg.Data.TLSGeneratorJobEnabled {
		// Create a cfssl function config from a ConfigMap template.
		cfsslFnCfg, err := cfunc.ParseTemplate("cfssl-cm", cfsslCMTemplate, fnCfg)
		if err != nil {
			return nil, err
		}

		// Create a cfssl filter.
		cfsslRW := &kio.ByteReadWriter{FunctionConfig: cfsslFnCfg}
		cfsslFilter := &cfssl.CfsslFilter{cfsslRW}

		// Run the cfssl filter and use its Resources.
		cfsslRs, err := cfsslFilter.Filter([]*yaml.RNode{cfsslFnCfg})
		if err != nil {
			return nil, err
		}
		generatedRs = append(generatedRs, cfsslRs...)

		// Generate TLS Job Resources from templates.
		tlsRs, err := cfunc.ParseTemplates(f.tlsTemplates(), fnCfg)
		if err != nil {
			return nil, err
		}
		generatedRs = append(generatedRs, tlsRs...)
	}

	// Return the generated resources + patches + input.
	return append(generatedRs, in...), nil
}

// FunctionConfig populates a struct with information needed for Resource
// templates.
func (f *EtcdFilter) FunctionConfig(in []*yaml.RNode) (*FunctionConfig, error) {
	fnMeta, err := f.RW.FunctionConfig.GetMeta()
	if err != nil {
		return nil, err
	}
	// Make sure function config has metadata.name.
	if fnMeta.Name == "" {
		return nil, fmt.Errorf("function config must specify metadata.name.")
	}

	eic, err := f.getInitialCluster(in)
	if err != nil {
		return nil, err
	}

	hostnames, err := cfunc.GetStatefulSetHostnames(in, fnMeta.Name+"-server", fnMeta.Namespace)
	if err != nil {
		return nil, err
	}

	// Set defaults.
	fnCfg := FunctionConfig{}
	fnCfg.Data = FunctionData{
		TLSServerSecretName:     fnMeta.Name + "-" + fnMeta.Namespace + "-tls-server",
		TLSCASecretName:         fnMeta.Name + "-" + fnMeta.Namespace + "-tls-ca",
		TLSRootClientSecretName: fnMeta.Name + "-" + fnMeta.Namespace + "-tls-client-root",
		InitialCluster:          eic,
		Hostnames:               hostnames,
	}

	// Populate function data from config.
	if err := yaml.Unmarshal([]byte(f.RW.FunctionConfig.MustString()), &fnCfg); err != nil {
		return nil, err
	}

	// Set app labels.
	if fnCfg.Labels == nil {
		fnCfg.Labels = make(map[string]string)
	}
	name, ok := fnCfg.Labels["app.kubernetes.io/name"]
	if !ok || name == "" {
		fnCfg.Labels["app.kubernetes.io/name"] = "etcd-server"
	}
	instance, ok := fnCfg.Labels["app.kubernetes.io/instance"]
	if !ok || instance == "" {
		fnCfg.Labels["app.kubernetes.io/instance"] = fnMeta.Name
	}

	return &fnCfg, nil
}

func (f *EtcdFilter) getInitialCluster(in []*yaml.RNode) (string, error) {
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
