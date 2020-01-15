package main

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"text/template"

	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/filters"
	"sigs.k8s.io/kustomize/kyaml/yaml"

	"github.com/bzub/config-functions/cfunc"
)

const casiAnnotation = "config.bzub.dev/consul-agent-sidecar-injector"

// filter implements kio.Filter
type filter struct {
	*cfunc.CFunc
}

// templateData holds information used in Resource templates.
type templateData struct {
	// ObjectMeta contains Resource metadata to use in templates.
	*yaml.ObjectMeta

	// Replicas is the number of configured Consul server replicas. Used
	// for other options like --bootstrap-expect.
	Replicas int
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

	filter := &filter{}
	filter.CFunc = &cfunc.CFunc{}
	filter.RW = rw

	err := kio.Pipeline{
		Inputs: []kio.Reader{rw},
		Filters: []kio.Filter{
			filter,
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
	// Verify and name function config metadata.
	if err := f.VerifyMeta("consul-server"); err != nil {
		return nil, err
	}

	// Get resources we care about from input.
	managedRs, err := f.ManagedResources(in)
	if err != nil {
		return nil, err
	}

	// Find the Consul StatefulSet.
	sts, err := getConsulStatefulSet(managedRs)
	if err != nil {
		return nil, err
	}

	// Get data for templates.
	data, err := f.templateData(sts)
	if err != nil {
		return nil, err
	}

	// Generate Consul server Resources from templates.
	templateRs, err := cfunc.ParseTemplates(f.defaultTemplates(), data)
	if err != nil {
		return nil, err
	}

	// Get templated patch StatefulSet.
	stsPatchR, err := getConsulStatefulSet(templateRs)
	if err != nil {
		return nil, err
	}

	if f.gossipEnabled() {
		// Add gossip encryption config secret volume to Consul server
		// StatefulSet.
		if err := f.injectGossipSecretVolume(stsPatchR); err != nil {
			return nil, err
		}

		// Generate gossip Resouces from templates.
		gossipRs, err := cfunc.ParseTemplates(f.gossipTemplates(), data)
		if err != nil {
			return nil, err
		}

		templateRs = append(templateRs, gossipRs...)
	}

	if f.tlsEnabled() {
		// Generate agent TLS Resources from templates.
		tlsRs, err := cfunc.ParseTemplates(f.tlsTemplates(), data)
		if err != nil {
			return nil, err
		}

		templateRs = append(templateRs, tlsRs...)
	}

	if f.aclBootstrapEnabled() {
		// Generate ACL bootstrap Resources from templates.
		aclRs, err := cfunc.ParseTemplates(f.aclJobTemplates(), data)
		if err != nil {
			return nil, err
		}

		templateRs = append(templateRs, aclRs...)
	}

	// Set function config metadata on generated Resources.
	if err := f.SetMetadata(templateRs); err != nil {
		return nil, err
	}

	if f.agentSidecarInjectorEnabled() {
		// Create sidecar injection patches.
		sidecarPatches, err := f.getSidecarPatches(in)
		if err != nil {
			return nil, err
		}
		templateRs = append(templateRs, sidecarPatches...)

		if f.tlsEnabled() {
			sidecarTLSCMs, err := f.getSidecarTLSCMs(sidecarPatches)
			if err != nil {
				return nil, err
			}
			templateRs = append(templateRs, sidecarTLSCMs...)

			if err := requireSidecarTLSVolumes(sidecarPatches); err != nil {
				return nil, err
			}
		}

		if f.gossipEnabled() {
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

// templateData populates a struct with information needed for Resource
// templates.
func (f *filter) templateData(sts *yaml.RNode) (*templateData, error) {
	fnMeta, err := f.RW.FunctionConfig.GetMeta()
	if err != nil {
		return nil, err
	}

	// Defaults
	data := &templateData{
		ObjectMeta: &fnMeta.ObjectMeta,
		Replicas:   1,
	}

	if sts == nil {
		// Use defaults if no StatefulSet is provided (needs to be
		// generated).
		return data, nil
	}

	// Find the number of replicas for the workload being managed. Defaults
	// to 1 if replicas is omitted in the input Resource config.
	value, err := sts.Pipe(yaml.Lookup("spec", "replicas"))
	if err != nil {
		return nil, err
	}
	if value != nil {
		data.Replicas, err = strconv.Atoi(value.YNode().Value)
		if err != nil {
			return nil, err
		}
	}

	return data, nil
}

// injectGossipSecretVolume adds the gossip encryption Consul config secret as
// a projected volume source to the Consul agent Resource config.
//
// TODO: Strategic merge patch seems to replace rather than merge projected
// volume sources. This should be in the patch variable and this function
// removed once that's fixed.
func (f *filter) injectGossipSecretVolume(sts *yaml.RNode) error {
	// Get data for templates.
	data, err := f.templateData(sts)
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
		return nil
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
	if sts == nil {
		return nil, nil
	}

	return sts, nil
}

func (f *filter) gossipEnabled() bool {
	enabled, _ := f.RW.FunctionConfig.Pipe(
		yaml.Lookup("spec", "gossipEncryption", "enabled"),
	)
	if enabled != nil && enabled.Document().Value == "true" {
		return true
	}

	return false
}

func (f *filter) tlsEnabled() bool {
	enabled, _ := f.RW.FunctionConfig.Pipe(
		yaml.Lookup("spec", "tlsEncryption", "enabled"),
	)
	if enabled != nil && enabled.Document().Value == "true" {
		return true
	}

	return false
}

func (f *filter) aclBootstrapEnabled() bool {
	enabled, _ := f.RW.FunctionConfig.Pipe(
		yaml.Lookup("spec", "aclBootstrap", "enabled"),
	)
	if enabled != nil && enabled.Document().Value == "true" {
		return true
	}

	return false
}

func (f *filter) agentSidecarInjectorEnabled() bool {
	enabled, _ := f.RW.FunctionConfig.Pipe(
		yaml.Lookup("spec", "agentSidecarInjector", "enabled"),
	)
	if enabled != nil && enabled.Document().Value == "true" {
		return true
	}

	return false
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
	fnMeta, err := f.RW.FunctionConfig.GetMeta()
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
	fnMeta, err := f.RW.FunctionConfig.GetMeta()
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
