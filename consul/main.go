package main

import (
	"bytes"
	"fmt"
	"os"
	"text/template"

	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/filters"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

const casiAnnotation = "config.bzub.dev/consul-agent-sidecar-injector"

// filter implements kio.Filter
type filter struct {
	rw *kio.ByteReadWriter
}

// sidecarTemplateData holds information used in agent sidecar patches.
type sidecarTemplateData struct {
	// ResourceMeta contains Resource metadata from a workload to be
	// patched.
	yaml.ResourceMeta

	// ConsulServiceFQDN is the Consul server endpoint the agent should
	// use.
	ConsulServiceFQDN string

	// ConsulName is the name of the Consul server Resources for the agent.
	// Used for ConfigMap/Secret name prefixes.
	ConsulName string

	// ConsulNamespace is the namespace of the Consul server Resources for
	// the agent.  Used for ConfigMap/Secret name prefixes.
	ConsulNamespace string
}

func main() {
	rw := &kio.ByteReadWriter{
		Reader:                os.Stdin,
		Writer:                os.Stdout,
		KeepReaderAnnotations: true,
	}

	err := kio.Pipeline{
		Inputs: []kio.Reader{rw},
		Filters: []kio.Filter{
			&filter{rw},
			&filters.MergeFilter{},
			&filters.FormatFilter{},
			&filters.FileSetter{},
		},
		Outputs: []kio.Writer{rw},
	}.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

// Filter generates Resources.
func (f *filter) Filter(in []*yaml.RNode) ([]*yaml.RNode, error) {
	// Workaround single line style of function config.
	if err := fixStyles(in...); err != nil {
		return nil, err
	}

	// Get data for templates.
	fnCfg, err := f.functionConfig()
	if err != nil {
		return nil, err
	}

	// Generate a ConfigMap from the function config.
	fnConfigMap, err := ParseTemplate("function-cm", functionCMTemplate, fnCfg)
	if err != nil {
		return nil, err
	}
	in = append(in, fnConfigMap)

	// Generate Consul server Resources from templates.
	templateRs, err := ParseTemplates(f.serverTemplates(), fnCfg)
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
		gossipRs, err := ParseTemplates(f.gossipTemplates(), fnCfg)
		if err != nil {
			return nil, err
		}

		templateRs = append(templateRs, gossipRs...)
	}

	if fnCfg.Data.AgentTLSEnabled {
		// Generate agent TLS Resources from templates.
		tlsRs, err := ParseTemplates(f.tlsTemplates(), fnCfg)
		if err != nil {
			return nil, err
		}

		templateRs = append(templateRs, tlsRs...)
	}

	if fnCfg.Data.ACLBootstrapEnabled {
		// Generate ACL bootstrap Resources from templates.
		aclRs, err := ParseTemplates(f.aclJobTemplates(), fnCfg)
		if err != nil {
			return nil, err
		}

		templateRs = append(templateRs, aclRs...)
	}

	if fnCfg.Data.AgentSidecarInjectorEnabled {
		// Create sidecar injection patches.
		sidecarPatches, err := f.getSidecarPatches(in)
		if err != nil {
			return nil, err
		}
		templateRs = append(templateRs, sidecarPatches...)

		if fnCfg.Data.AgentTLSEnabled {
			sidecarTLSCMs, err := f.getSidecarTLSCMs(sidecarPatches)
			if err != nil {
				return nil, err
			}
			templateRs = append(templateRs, sidecarTLSCMs...)

			if err := requireSidecarTLSVolumes(sidecarPatches); err != nil {
				return nil, err
			}
		}

		if fnCfg.Data.GossipEnabled {
			// Add gossip encryption config secret volume to Consul
			// agent sidecars.
			for _, scp := range sidecarPatches {
				if err := f.injectGossipSecretVolume(scp); err != nil {
					return nil, err
				}
			}
		}
	}

	// Merge our templated Resources into the input Resources.
	return append(in, templateRs...), nil
}

// functionConfig populates a struct with information needed for Resource
// templates.
func (f *filter) functionConfig() (*functionConfig, error) {
	fnMeta, err := f.rw.FunctionConfig.GetMeta()
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
		Replicas:                 1,
	}

	// Populate function data from config.
	if err := yaml.Unmarshal([]byte(f.rw.FunctionConfig.MustString()), &fnCfg); err != nil {
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
func (f *filter) injectGossipSecretVolume(sts *yaml.RNode) error {
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

func (f *filter) getACLJobEnv(in []*yaml.RNode) (*yaml.RNode, error) {
	fnMeta, err := f.rw.FunctionConfig.GetMeta()
	if err != nil {
		return nil, err
	}

	var cm *yaml.RNode
	for _, r := range in {
		rMeta, err := r.GetMeta()
		if err != nil {
			return nil, err
		}

		switch {
		case rMeta.Kind != "ConfigMap":
			fallthrough
		case rMeta.Name != fnMeta.Name+"-acl-bootstrap-env":
			fallthrough
		case rMeta.Namespace != fnMeta.Namespace:
			continue
		}

		cm = r
		break
	}

	return cm, nil
}

func (f *filter) getGossipJobEnv(in []*yaml.RNode) (*yaml.RNode, error) {
	fnMeta, err := f.rw.FunctionConfig.GetMeta()
	if err != nil {
		return nil, err
	}

	var cm *yaml.RNode
	for _, r := range in {
		rMeta, err := r.GetMeta()
		if err != nil {
			return nil, err
		}

		switch {
		case rMeta.Kind != "ConfigMap":
			fallthrough
		case rMeta.Name != fnMeta.Name+"-gossip-encryption-env":
			fallthrough
		case rMeta.Namespace != fnMeta.Namespace:
			continue
		}

		cm = r
		break
	}

	return cm, nil
}

// getSidecarPatches returns patches with a sidecar added.
func (f *filter) getSidecarPatches(in []*yaml.RNode) ([]*yaml.RNode, error) {
	// Get resources that are calling for sidecar injection.
	sidecarRs, err := sidecarResources(in)
	if err != nil {
		return nil, err
	}

	// Create a patch for each resource.
	sidecarPatches := []*yaml.RNode{}
	for r, c := range sidecarRs {
		p, err := f.getSidecarPatch(in, r, c)
		if err != nil {
			return nil, err
		}
		sidecarPatches = append(sidecarPatches, p)
	}

	return sidecarPatches, nil
}

// sidecarResources returns a map of resources/configs from in that have the
// config.bzub.dev/consul-agent-sidecar-injector annotation.
func sidecarResources(in []*yaml.RNode) (map[*yaml.RNode]*yaml.RNode, error) {
	sidecarRs := make(map[*yaml.RNode]*yaml.RNode)
	for _, r := range in {
		aValue, err := r.Pipe(yaml.GetAnnotation(casiAnnotation))
		if err != nil {
			return nil, err
		}
		if aValue == nil {
			continue
		}

		config, err := yaml.Parse(aValue.Document().Value)
		if err != nil {
			return nil, err
		}
		sidecarRs[r] = config
	}

	return sidecarRs, nil
}

// getSidecarPatch returns a patch with a sidecar added.
func (f *filter) getSidecarPatch(in []*yaml.RNode, r, c *yaml.RNode) (*yaml.RNode, error) {
	fnMeta, err := f.rw.FunctionConfig.GetMeta()
	if err != nil {
		return nil, err
	}

	// Determine if sidecar injector config name matches this Consul
	// instance.
	cName, err := c.Pipe(yaml.Lookup("metadata", "name"))
	if err != nil {
		return nil, err
	}
	if cName == nil {
		return nil, fmt.Errorf("metadata.name missing in config.")
	}
	if cName.Document().Value != fnMeta.Name {
		return nil, nil
	}

	// Determine if sidecar injector config namespace matches this
	// Consul instance.
	cNS, err := c.Pipe(yaml.Lookup("metadata", "namespace"))
	if err != nil {
		return nil, err
	}
	if cNS == nil {
		return nil, fmt.Errorf("metadata.namespace missing in config.")
	}
	if cNS.Document().Value != fnMeta.Namespace {
		return nil, nil
	}

	// Populate Consul server specific template data.
	data := &sidecarTemplateData{}
	data.ConsulName = fnMeta.Name
	data.ConsulNamespace = fnMeta.Namespace
	data.ConsulServiceFQDN = fmt.Sprintf(
		"%v.%v.svc.cluster.local", fnMeta.Name, fnMeta.Namespace,
	)

	// Populate resource specific patch template data.
	data.ResourceMeta, err = r.GetMeta()
	if err != nil {
		return nil, err
	}

	// Create the patch from template.
	buff := &bytes.Buffer{}
	t := template.Must(template.New("patch").Parse(sidecarPatchTemplate))
	if err := t.Execute(buff, data); err != nil {
		return nil, err
	}
	p, err := yaml.Parse(buff.String())
	if err != nil {
		return nil, err
	}

	return p, nil
}

func (f *filter) getSidecarTLSCMs(in []*yaml.RNode) ([]*yaml.RNode, error) {
	fnMeta, err := f.rw.FunctionConfig.GetMeta()
	if err != nil {
		return nil, err
	}
	data := &sidecarTemplateData{}
	data.ConsulName = fnMeta.Name
	data.ConsulNamespace = fnMeta.Namespace

	// Create a ConfigMap for each resource. Redundant CMs will be merged
	// together later.
	cms := []*yaml.RNode{}
	for _, r := range in {
		data.ResourceMeta, err = r.GetMeta()
		if err != nil {
			return nil, err
		}

		// Create the ConfigMap from template.
		buff := &bytes.Buffer{}
		t := template.Must(template.New("cm").Parse(sidecarTLSCMTemplate))
		if err := t.Execute(buff, data); err != nil {
			return nil, err
		}
		cm, err := yaml.Parse(buff.String())
		if err != nil {
			return nil, err
		}

		cms = append(cms, cm)
	}

	return cms, nil
}

func requireSidecarTLSVolumes(in []*yaml.RNode) error {
	for _, r := range in {
		optional, err := r.Pipe(
			yaml.Lookup(
				"spec", "template", "spec", "volumes",
				"[name=consul-tls-secret]", "secret", "optional",
			),
		)
		if err != nil {
			return err
		}
		if optional == nil {
			return fmt.Errorf("Unable to find consul-tls-secret volume.")
		}

		if err := optional.PipeE(
			yaml.Set(yaml.NewScalarRNode("false")),
		); err != nil {
			return err
		}
	}

	return nil
}

func fixStyles(in ...*yaml.RNode) error {
	for _, r := range in {
		r.YNode().Style = 0
		switch r.YNode().Kind {
		case yaml.MappingNode:
			err := r.VisitFields(func(node *yaml.MapNode) error {
				return fixStyles(node.Value)
			})
			if err != nil {
				return err
			}
		case yaml.SequenceNode:
			err := r.VisitElements(func(node *yaml.RNode) error {
				return fixStyles(node)
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func ParseTemplates(tmpls map[string]string, data interface{}) ([]*yaml.RNode, error) {
	templateRs := []*yaml.RNode{}
	for name, tmpl := range tmpls {
		r, err := ParseTemplate(name, tmpl, data)
		if err != nil {
			return nil, err
		}
		templateRs = append(templateRs, r)
	}

	return templateRs, nil
}

func ParseTemplate(name, tmpl string, data interface{}) (*yaml.RNode, error) {
	buff := &bytes.Buffer{}
	t := template.Must(template.New(name).Parse(tmpl))
	if err := t.Execute(buff, data); err != nil {
		return nil, err
	}
	r, err := yaml.Parse(buff.String())
	if err != nil {
		return nil, err
	}
	return r, nil
}
