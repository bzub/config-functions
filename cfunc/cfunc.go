package cfunc

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"text/template"

	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func FixStyles(in ...*yaml.RNode) error {
	for _, r := range in {
		r.YNode().Style = 0
		switch r.YNode().Kind {
		case yaml.MappingNode:
			err := r.VisitFields(func(node *yaml.MapNode) error {
				return FixStyles(node.Value)
			})
			if err != nil {
				return err
			}
		case yaml.SequenceNode:
			err := r.VisitElements(func(node *yaml.RNode) error {
				return FixStyles(node)
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

func GetStatefulSetHostnames(in []*yaml.RNode, name, ns string) ([]string, error) {
	sts, err := GetStatefulSet(in, name, ns)
	if err != nil {
		return nil, err
	}
	if sts == nil {
		return []string{name + "-0"}, nil
	}

	replicas, err := GetReplicas(sts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sts: %v\n", sts.MustString())
		return nil, err
	}

	names := []string{}
	for i := 0; i < replicas; i++ {
		names = append(names, fmt.Sprintf(name+"-%v", i))
	}

	return names, nil
}

func GetStatefulSet(in []*yaml.RNode, name, ns string) (*yaml.RNode, error) {
	for _, r := range in {
		rMeta, err := r.GetMeta()
		if err != nil {
			return nil, err
		}

		if rMeta.Name != name || rMeta.Namespace != ns || rMeta.Kind != "StatefulSet" {
			continue
		}

		// Found it.
		return r, nil
	}

	return nil, nil
}

func GetReplicas(r *yaml.RNode) (int, error) {
	rMeta, err := r.GetMeta()
	if err != nil {
		return 0, err
	}

	// Make sure it's a Kind that supports `spec.replicas`.
	switch rMeta.Kind {
	case "Deployment", "ReplicaSet", "ReplicationController", "StatefulSet":
	default:
		return 0, fmt.Errorf("unable to determine replica count for Kind: %v", rMeta.Kind)
	}

	replicasR, err := r.Pipe(yaml.Lookup("spec", "replicas"))
	if err != nil {
		return 0, err
	}
	if replicasR == nil {
		return 1, nil
	}

	replicas, err := strconv.Atoi(replicasR.Document().Value)
	if err != nil {
		return 0, err
	}

	return replicas, nil
}
