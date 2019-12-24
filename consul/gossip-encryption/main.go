package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/filters"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func main() {
	rw := &kio.ByteReadWriter{
		Reader:                os.Stdin,
		Writer:                os.Stdout,
		KeepReaderAnnotations: true,
	}

	err := kio.Pipeline{
		Inputs: []kio.Reader{rw},
		Filters: []kio.Filter{
			&filter{rw: rw},
			&filters.MergeFilter{},
			&filters.FileSetter{
				FilenamePattern: filepath.Join("resources", "%n_%k.yaml"),
			},
		},
		Outputs: []kio.Writer{rw},
	}.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

// API is the function configuration spec.
type API struct {
	Metadata struct {
		// Name is used to prefix Resource metadata.name. Also used to
		// exec into a Consul server pod for bootstrapping.
		Name string `yaml:"name"`
	} `yaml:"metadata"`

	Spec struct {
		// StatefulSetName is the StatefulSet to configure to use
		// gossip encryption.
		StatefulSetName string `yaml:"statefulSetName"`
	} `yaml:"spec"`
}

// filter implements kio.Filter
type filter struct {
	rw *kio.ByteReadWriter
}

func (a *API) parseNewTemplate(name, tmpl string) (*yaml.RNode, error) {
	buff := &bytes.Buffer{}
	t := template.Must(template.New(name).Parse(tmpl))
	if err := t.Execute(buff, a); err != nil {
		return nil, err
	}
	r, err := yaml.Parse(buff.String())
	if err != nil {
		return nil, err
	}
	return r, nil
}

// Filter generates Resources.
func (f *filter) Filter(in []*yaml.RNode) ([]*yaml.RNode, error) {
	api := f.parseAPI()

	var sts *yaml.RNode
	for _, r := range in {
		meta, err := r.GetMeta()
		if err != nil {
			return nil, err
		}

		if meta.Kind == "StatefulSet" && meta.Name == api.Spec.StatefulSetName {
			sts = r
			break
		}
	}

	if sts == nil {
		return nil, fmt.Errorf(
			"StatefulSet config \"%v\" not found.\n",
			api.Spec.StatefulSetName,
		)
	}

	// Find the StatefulSet command field to modify.
	command, err := sts.Pipe(yaml.Lookup("spec", "template", "spec", "containers", "[name=consul]", "command"))
	if err != nil {
		s, _ := command.String()
		return nil, fmt.Errorf("%v: %s", err, s)
	}
	if command == nil {
		return nil, fmt.Errorf(
			"Unable to find a \"consul\" command: sts/%v\n%v",
			api.Spec.StatefulSetName, sts.MustString(),
		)
	}

	arg := "-config-dir=/consul/config/gossip-encryption"

	// Check if our argument is already added to the command.
	argIsSet := false
	err = command.VisitElements(func(e *yaml.RNode) error {
		s, err := e.String()
		if err != nil {
			return err
		}
		if s == "- "+arg {
			argIsSet = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if !argIsSet {
		// Configure the Consul StatefulSet to use the gossip encryption
		// config.
		command, err = command.Pipe(yaml.Append(
			&yaml.Node{
				Value: arg,
				Kind:  yaml.ScalarNode,
			},
		))
		if err != nil {
			return nil, err
		}
	}

	// Generate Job and associated Resource configs.
	job, err := api.parseNewTemplate("job", jobTemplate)
	if err != nil {
		return nil, err
	}
	sa, err := api.parseNewTemplate("sa", serviceAccountTemplate)
	if err != nil {
		return nil, err
	}
	role, err := api.parseNewTemplate("role", roleTemplate)
	if err != nil {
		return nil, err
	}
	rolebinding, err := api.parseNewTemplate("rolebinding", roleBindingTemplate)
	if err != nil {
		return nil, err
	}
	stsPatch, err := api.parseNewTemplate("statefulset", statefulSetPatchTemplate)
	if err != nil {
		return nil, err
	}

	in = append(in, job, sa, role, rolebinding, stsPatch)
	return in, nil
}

// parseAPI parses the functionConfig into an API struct, and validates the
// input.
func (f *filter) parseAPI() API {
	// parse the input function config
	var api API
	if err := yaml.Unmarshal([]byte(f.rw.FunctionConfig.MustString()), &api); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	if api.Metadata.Name == "" {
		fmt.Fprintf(os.Stderr, "must specify metadata.name\n")
		os.Exit(1)
	}

	if api.Spec.StatefulSetName == "" {
		api.Spec.StatefulSetName = api.Metadata.Name
	}

	return api
}

// A patch to configure the StatefulSet to use the gossip encryption config.
var statefulSetPatchTemplate = `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: {{ .Spec.StatefulSetName }}
spec:
  template:
    spec:
      containers:
        - name: consul
          volumeMounts:
            - name: config-gossip-encryption
              mountPath: /consul/config/gossip-encryption
      volumes:
        - name: config-gossip-encryption
          secret:
            secretName: {{ .Metadata.Name }}
`

var jobTemplate = `apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .Metadata.Name }}
spec:
  template:
    metadata:
      labels:
        app.kubernetes.io/name: consul-gossip-encryption
        app.kubernetes.io/instance: {{ .Metadata.Name }}
    spec:
      serviceAccountName: {{ .Metadata.Name }}
      restartPolicy: OnFailure
      initContainers:
        - name: generate-gossip-encryption-config
          image: docker.io/library/consul:1.6.2
          command:
            - /bin/sh
            - -ec
            - |-
              config_file=/config/generated/00-gossip-encryption.json
              cat <<EOF > "${config_file}"
              {
                "encrypt": "$(consul keygen)",
                "encrypt_verify_incoming": true,
                "encrypt_verify_outgoing": true
              }
          volumeMounts:
            - mountPath: /config/generated
              name: config-generated
      containers:
        - name: create-gossip-encryption-config-secret
          image: k8s.gcr.io/hyperkube:v1.17.0
          command:
            - /bin/sh
            - -ec
            - |-
              secret="{{ .Metadata.Name }}"
              config_dir="/config/generated"

              if [ ! "$(kubectl get secret --no-headers "${secret}"|wc -l)" = "0" ]; then
                echo "[INFO] \"secret/${secret}\" already exists. Exiting."
                exit 0
              fi

              echo "[INFO] Creating \"secret/${secret}\"."
              kubectl create secret generic "--from-file=${config_dir}" "${secret}"
          volumeMounts:
            - mountPath: /config/generated
              name: config-generated
      volumes:
        - name: config-generated
          emptyDir: {}
`

var serviceAccountTemplate = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Metadata.Name }}
  labels:
    app.kubernetes.io/name: consul-gossip-encryption
    app.kubernetes.io/instance: {{ .Metadata.Name }}
`

var roleTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ .Metadata.Name }}
  labels:
    app.kubernetes.io/name: consul-gossip-encryption
    app.kubernetes.io/instance: {{ .Metadata.Name }}
rules:
  - apiGroups:
      - ""
    resources:
      - secrets
    verbs:
      - get
      - list
      - create
`

var roleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ .Metadata.Name }}
  labels:
    app.kubernetes.io/name: consul-gossip-encryption
    app.kubernetes.io/instance: {{ .Metadata.Name }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ .Metadata.Name }}
subjects:
  - kind: ServiceAccount
    name: {{ .Metadata.Name }}
`
