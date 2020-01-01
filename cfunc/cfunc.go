package cfunc

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// CFunc is an opinionated config function helper.
type CFunc struct {
	RW   *kio.ByteReadWriter
	name string
}

// VerifyMeta validates the function config's metadata and sets
// app.kubernetes.io/name as provided.
func (f *CFunc) VerifyMeta(appName string) error {
	fnMeta, err := f.RW.FunctionConfig.GetMeta()
	if err != nil {
		return err
	}

	// Make sure function config has metadata.name.
	if fnMeta.Name == "" {
		return fmt.Errorf("function config must specify metadata.name.")
	}

	// Set label app.kubernetes.io/name.
	labels, err := f.RW.FunctionConfig.Pipe(yaml.Lookup("metadata", "labels"))
	if err != nil {
		return err
	}
	err = labels.PipeE(yaml.SetField("app.kubernetes.io/name", yaml.NewScalarRNode(appName)))
	if err != nil {
		return err
	}

	return nil
}

// isManagedResource checks if a given resource matches indicators which tell
// us it should be managed by this config function.
func (f *CFunc) isManagedResource(r *yaml.RNode) (bool, error) {
	// Get Resource and Config Function metadata.
	rMeta, err := r.GetMeta()
	if err != nil {
		return false, err
	}
	fnMeta, err := f.RW.FunctionConfig.GetMeta()
	if err != nil {
		return false, err
	}

	// Check metadata matches.
	rInstance := rMeta.Labels["app.kubernetes.io/instance"]
	fnInstance := fnMeta.Labels["app.kubernetes.io/instance"]
	if rMeta.Namespace != fnMeta.Namespace || rInstance != fnInstance {
		return false, nil
	}

	return true, nil
}

// ManagedResources checks a collection of Resources indivudually for
// indicators which tell us they should be managed by this config function, and
// returns the matching Resources.
func (f *CFunc) ManagedResources(in []*yaml.RNode) ([]*yaml.RNode, error) {
	managedRs := []*yaml.RNode{}
	for _, r := range in {
		ok, err := f.isManagedResource(r)
		if err != nil {
			return nil, err
		}
		if ok {
			managedRs = append(managedRs, r)
		}
	}

	return managedRs, nil
}

// SetMetadata sets labels, selectors and namespace from the CFunc onto given
// Resources.
func (f *CFunc) SetMetadata(in []*yaml.RNode) error {
	for _, r := range in {
		// Set labels from config function to resources.
		if err := f.setLabels(r); err != nil {
			return err
		}

		// Set namespace from config function to resources.
		if err := f.setNamespace(r); err != nil {
			return err
		}
	}

	return nil
}

// setLabels ensures the config function's labels/values are set on a Resource.
func (f *CFunc) setLabels(r *yaml.RNode) error {
	fnMeta, err := f.RW.FunctionConfig.GetMeta()
	if err != nil {
		return err
	}
	rMeta, err := r.GetMeta()
	if err != nil {
		return err
	}

	rLabels, err := r.Pipe(yaml.LookupCreate(
		yaml.MappingNode,
		"metadata", "labels",
	))
	if err != nil {
		return err
	}

	rAnnotations, err := r.Pipe(yaml.LookupCreate(
		yaml.MappingNode,
		"metadata", "annotations",
	))
	if err != nil {
		return err
	}

	rPodTemplate, err := r.Pipe(yaml.Lookup("spec", "template"))
	if err != nil {
		return err
	}

	// Find cfunc/preserve-label/ prefixed annotations to configure
	// labeling.
	preserveLabels := []string{}
	rAnnotations.VisitFields(func(node *yaml.MapNode) error {
		key := node.Key.YNode().Value
		value := node.Value.YNode().Value
		if !strings.HasPrefix(key, "cfunc/preserve-label/") || value != "true" {
			return nil
		}

		// Mark the label for preservation.
		preserveLabels = append(
			preserveLabels,
			strings.TrimPrefix(key, "cfunc/preserve-label/"),
		)

		// Remove the annotation.
		if err := r.PipeE(yaml.ClearAnnotation(key)); err != nil {
			return err
		}

		return nil
	})

	// Sync function config labels with resource labels.
	for k, v := range fnMeta.Labels {
		// Check if this is a preserved label.
		skip := false
		for _, preserved := range preserveLabels {
			if k == preserved {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		// Set the label to what's given in the function config.
		if err := rLabels.PipeE(yaml.SetField(k, yaml.NewScalarRNode(v))); err != nil {
			return err
		}

		// Set the label to the pod template if it's a workload
		// Resource.
		if rPodTemplate != nil {
			rPodTemplateLabels, err := rPodTemplate.Pipe(yaml.LookupCreate(
				yaml.MappingNode,
				"metadata", "labels",
			))
			if err != nil {
				return err
			}
			if err := rPodTemplateLabels.PipeE(yaml.SetField(k, yaml.NewScalarRNode(v))); err != nil {
				return err
			}
		}

		// Sync the label to selector.
		switch rMeta.Kind {
		case "StatefulSet":
			selector, err := r.Pipe(
				yaml.LookupCreate(
					yaml.MappingNode,
					"spec", "selector", "matchLabels",
				),
			)
			if err != nil {
				return err
			}
			if err := selector.PipeE(yaml.SetField(k, yaml.NewScalarRNode(v))); err != nil {
				return err
			}
		case "Service":
			selector, err := r.Pipe(
				yaml.LookupCreate(
					yaml.MappingNode,
					"spec", "selector",
				),
			)
			if err != nil {
				return err
			}
			if err := selector.PipeE(yaml.SetField(k, yaml.NewScalarRNode(v))); err != nil {
				return err
			}
		}
	}

	return nil
}

// setNamespace ensures the config function's namespace is set on a Resource.
func (f *CFunc) setNamespace(r *yaml.RNode) error {
	fnMeta, err := f.RW.FunctionConfig.GetMeta()
	if err != nil {
		return err
	}

	if fnMeta.Namespace == "" {
		// No namespace provided in function config.
		return nil
	}

	rNamespace, err := r.Pipe(yaml.LookupCreate(
		yaml.ScalarNode,
		"metadata", "namespace",
	))
	if err != nil {
		return err
	}

	rNamespace.YNode().SetString(fnMeta.Namespace)

	return nil
}

func ParseTemplates(tmpls map[string]string, data interface{}) ([]*yaml.RNode, error) {
	templateRs := []*yaml.RNode{}
	for name, tmpl := range tmpls {
		buff := &bytes.Buffer{}
		t := template.Must(template.New(name).Parse(tmpl))
		if err := t.Execute(buff, data); err != nil {
			return nil, err
		}
		r, err := yaml.Parse(buff.String())
		if err != nil {
			return nil, err
		}

		templateRs = append(templateRs, r)
	}

	return templateRs, nil
}
