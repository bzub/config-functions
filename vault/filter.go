package vault

import (
	"fmt"

	"github.com/bzub/config-functions/cfssl"
	"github.com/bzub/config-functions/cfunc"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// VaultFilter implements kio.Filter
type VaultFilter struct {
	RW *kio.ByteReadWriter
}

// Filter generates Resources.
func (f *VaultFilter) Filter(in []*yaml.RNode) ([]*yaml.RNode, error) {
	// Workaround single line style of function config.
	if err := cfunc.FixStyles(in...); err != nil {
		return nil, err
	}

	// Get data for templates.
	fnCfg, err := f.FunctionConfig()
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

	// Generate Vault server Resources from templates.
	serverRs, err := cfunc.ParseTemplates(f.serverTemplates(), fnCfg)
	if err != nil {
		return nil, err
	}
	generatedRs = append(generatedRs, serverRs...)

	if fnCfg.Data.InitEnabled {
		// Generate init Job Resources from templates.
		initRs, err := cfunc.ParseTemplates(f.initJobTemplates(), fnCfg)
		if err != nil {
			return nil, err
		}
		generatedRs = append(generatedRs, initRs...)
	}

	if fnCfg.Data.UnsealEnabled {
		// Generate unseal Job Resources from templates.
		unsealRs, err := cfunc.ParseTemplates(f.unsealJobTemplates(), fnCfg)
		if err != nil {
			return nil, err
		}
		generatedRs = append(generatedRs, unsealRs...)
	}

	if fnCfg.Data.GenerateTLSEnabled {
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
	}

	// Return the generated resources + patches + input.
	return append(generatedRs, in...), nil
}

// FunctionConfig populates a struct with information needed for Resource
// templates.
func (f *VaultFilter) FunctionConfig() (*FunctionConfig, error) {
	fnMeta, err := f.RW.FunctionConfig.GetMeta()
	if err != nil {
		return nil, err
	}
	// Make sure function config has metadata.name.
	if fnMeta.Name == "" {
		return nil, fmt.Errorf("function config must specify metadata.name.")
	}

	// Set defaults.
	fnCfg := FunctionConfig{}
	fnCfg.Data = FunctionData{
		UnsealSecretName: fnMeta.Name + "-" + fnMeta.Namespace + "-unseal",
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
		fnCfg.Labels["app.kubernetes.io/name"] = "vault-server"
	}
	instance, ok := fnCfg.Labels["app.kubernetes.io/instance"]
	if !ok || instance == "" {
		fnCfg.Labels["app.kubernetes.io/instance"] = fnMeta.Name
	}

	return &fnCfg, nil
}
