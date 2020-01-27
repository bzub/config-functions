package consul

import (
	"fmt"

	"github.com/bzub/config-functions/cfunc"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// ConsulFilter implements kio.Filter
type ConsulFilter struct {
	RW *kio.ByteReadWriter
}

// Filter generates Resources.
func (f *ConsulFilter) Filter(in []*yaml.RNode) ([]*yaml.RNode, error) {
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

	// Generate Consul server Resources from templates.
	serverRs, err := cfunc.ParseTemplates(f.serverTemplates(), fnCfg)
	if err != nil {
		return nil, err
	}
	generatedRs = append(generatedRs, serverRs...)

	if fnCfg.Data.GossipKeyGeneratorJobEnabled {
		// Generate gossip Resouces from templates.
		gossipRs, err := cfunc.ParseTemplates(f.gossipTemplates(), fnCfg)
		if err != nil {
			return nil, err
		}
		generatedRs = append(generatedRs, gossipRs...)
	}

	if fnCfg.Data.TLSGeneratorJobEnabled {
		// Generate agent TLS Resources from templates.
		tlsRs, err := cfunc.ParseTemplates(f.tlsTemplates(), fnCfg)
		if err != nil {
			return nil, err
		}
		generatedRs = append(generatedRs, tlsRs...)
	}

	if fnCfg.Data.ACLBootstrapJobEnabled {
		// Generate ACL bootstrap Resources from templates.
		aclRs, err := cfunc.ParseTemplates(f.aclJobTemplates(), fnCfg)
		if err != nil {
			return nil, err
		}
		generatedRs = append(generatedRs, aclRs...)
	}

	if fnCfg.Data.AgentSidecarInjectorEnabled {
		// Generate sidecar patch Resources for workloads that call for
		// them from input.
		sidecarRs, err := sidecarPatches(in, fnCfg)
		if err != nil {
			return nil, err
		}
		generatedRs = append(generatedRs, sidecarRs...)
	}

	// Return the generated resources + patches + input.
	return append(generatedRs, in...), nil
}

// FunctionConfig populates a struct with information needed for Resource
// templates.
func (f *ConsulFilter) FunctionConfig() (*FunctionConfig, error) {
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
		ACLBootstrapSecretName: fnMeta.Name + "-" + fnMeta.Namespace + "-acl",
		TLSServerSecretName:    fnMeta.Name + "-" + fnMeta.Namespace + "-tls-server",
		TLSCASecretName:        fnMeta.Name + "-" + fnMeta.Namespace + "-tls-ca",
		TLSCLISecretName:       fnMeta.Name + "-" + fnMeta.Namespace + "-tls-cli",
		TLSClientSecretName:    fnMeta.Name + "-" + fnMeta.Namespace + "-tls-client",
		GossipSecretName:       fnMeta.Name + "-" + fnMeta.Namespace + "-gossip",
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
		fnCfg.Labels["app.kubernetes.io/name"] = "consul-server"
	}
	instance, ok := fnCfg.Labels["app.kubernetes.io/instance"]
	if !ok || instance == "" {
		fnCfg.Labels["app.kubernetes.io/instance"] = fnMeta.Name
	}

	return &fnCfg, nil
}
