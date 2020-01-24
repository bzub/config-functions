package consul

import (
	"bytes"
	"fmt"
	"text/template"

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
	fnCfg, err := f.functionConfig()
	if err != nil {
		return nil, err
	}

	// Generate a ConfigMap from the function config.
	fnConfigMap, err := cfunc.ParseTemplate("function-cm", functionCMTemplate, fnCfg)
	if err != nil {
		return nil, err
	}
	in = append(in, fnConfigMap)

	// Generate Consul server Resources from templates.
	templateRs, err := cfunc.ParseTemplates(f.serverTemplates(), fnCfg)
	if err != nil {
		return nil, err
	}

	// Get templated patch StatefulSet.
	stsPatchR, err := getConsulStatefulSet(templateRs)
	if err != nil {
		return nil, err
	}

	if fnCfg.Data.GossipEnabled {
		// Add gossip encryption config secret volume to Consul server
		// StatefulSet.
		if err := f.injectGossipSecretVolume(stsPatchR); err != nil {
			return nil, err
		}

		// Generate gossip Resouces from templates.
		gossipRs, err := cfunc.ParseTemplates(f.gossipTemplates(), fnCfg)
		if err != nil {
			return nil, err
		}

		templateRs = append(templateRs, gossipRs...)
	}

	if fnCfg.Data.AgentTLSEnabled {
		// Generate agent TLS Resources from templates.
		tlsRs, err := cfunc.ParseTemplates(f.tlsTemplates(), fnCfg)
		if err != nil {
			return nil, err
		}

		templateRs = append(templateRs, tlsRs...)
	}

	if fnCfg.Data.ACLBootstrapEnabled {
		// Generate ACL bootstrap Resources from templates.
		aclRs, err := cfunc.ParseTemplates(f.aclJobTemplates(), fnCfg)
		if err != nil {
			return nil, err
		}

		templateRs = append(templateRs, aclRs...)
	}

	// Return the input + generated resources + patches.
	return append(in, templateRs...), nil
}

// functionConfig populates a struct with information needed for Resource
// templates.
func (f *ConsulFilter) functionConfig() (*functionConfig, error) {
	fnMeta, err := f.RW.FunctionConfig.GetMeta()
	if err != nil {
		return nil, err
	}
	// Make sure function config has metadata.name.
	if fnMeta.Name == "" {
		return nil, fmt.Errorf("function config must specify metadata.name.")
	}

	// Set defaults.
	fnCfg := functionConfig{}
	fnCfg.Data = functionData{
		AgentTLSServerSecretName: fnMeta.Name + "-" + fnMeta.Namespace + "-tls-server",
		AgentTLSCASecretName:     fnMeta.Name + "-" + fnMeta.Namespace + "-tls-ca",
		AgentTLSCLISecretName:    fnMeta.Name + "-" + fnMeta.Namespace + "-tls-cli",
		GossipSecretName:         fnMeta.Name + "-" + fnMeta.Namespace + "-gossip",
		ACLBootstrapSecretName:   fnMeta.Name + "-" + fnMeta.Namespace + "-acl",
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

// injectGossipSecretVolume adds the gossip encryption Consul config secret as
// a projected volume source to the volume called "consul-configs".
func (f *ConsulFilter) injectGossipSecretVolume(sts *yaml.RNode) error {
	// Get data for templates.
	data, err := f.functionConfig()
	if err != nil {
		return err
	}

	// Execute the projected volume source template.
	buff := &bytes.Buffer{}
	t := template.Must(template.New("gossip-pvs").Parse(gossipSecretVolumeTemplate))
	if err := t.Execute(buff, data); err != nil {
		return err
	}
	vol, err := yaml.Parse(buff.String())
	if err != nil {
		return err
	}

	// Add secret volume to projected volume.
	err = sts.PipeE(
		yaml.Lookup(
			"spec", "template", "spec", "volumes",
			"[name=consul-configs]", "projected", "sources",
		),
		yaml.Append(vol.YNode().Content...),
	)
	if err != nil {
		return err
	}

	return nil
}

func getConsulStatefulSet(in []*yaml.RNode) (*yaml.RNode, error) {
	// Find the StatefulSet
	var sts *yaml.RNode
	for _, r := range in {
		rMeta, err := r.GetMeta()
		if err != nil {
			return nil, err
		}

		if rMeta.Kind == "StatefulSet" {
			sts = r
			break
		}
	}

	return sts, nil
}
