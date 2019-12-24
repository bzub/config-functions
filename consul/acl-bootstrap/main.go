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
		// ServiceName is the Service to execute acl bootstrapping
		// against.
		ServiceName string `yaml:"serviceName"`
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

	in = append(in, job, sa, role, rolebinding)
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

	if api.Spec.ServiceName == "" {
		api.Spec.ServiceName = api.Metadata.Name
	}

	return api
}

var jobTemplate = `apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .Metadata.Name }}
spec:
  template:
    metadata:
      labels:
        app.kubernetes.io/name: consul-acl-bootstrap
        app.kubernetes.io/instance: {{ .Metadata.Name }}
    spec:
      serviceAccountName: {{ .Metadata.Name }}
      restartPolicy: OnFailure
      containers:
        - name: consul-acl-bootstrap
          image: k8s.gcr.io/hyperkube:v1.17.0
          command:
            - /bin/sh
            - -ec
            - |-
              consul_secret="{{ .Metadata.Name }}"
              consul_svc_name="{{ .Spec.ServiceName }}"

              metadata_dir="/metadata/consul-secrets"
              if [ "$(ls "${metadata_dir}"|wc -l)" = "0" ]; then
                metadata_dir="/metadata/consul-init"

                echo "[INFO] Performing consul acl bootstrap."
                export consul_bootstrap_out="$(kubectl exec "svc/${consul_svc_name}" -- consul acl bootstrap)"
                if [ "${consul_bootstrap_out}" = "" ]; then
                  echo "[ERROR] No consul acl bootstrap output. Is consul up and running?"
                  exit 1
                fi
                echo "${consul_bootstrap_out}"|grep AccessorID|awk '{print $2}'|tr -d '\n' > "${metadata_dir}/accessor_id.txt"
                echo "${consul_bootstrap_out}"|grep SecretID|awk '{print $2}'|tr -d '\n' > "${metadata_dir}/secret_id.txt"

                echo "[INFO] Creating \"secret/${consul_secret}\"."
                kubectl create secret generic "--from-file=${metadata_dir}" "${consul_secret}"
              fi

              export CONSUL_HTTP_TOKEN="$(cat "${metadata_dir}/secret_id.txt")"
              kubectl exec "svc/${consul_svc_name}" -- /bin/sh -c "CONSUL_HTTP_TOKEN=$(cat "${metadata_dir}/secret_id.txt") consul acl token list"
          volumeMounts:
            - mountPath: /metadata/consul-init
              name: consul-init
            - mountPath: /metadata/consul-secrets
              name: consul-secrets
      volumes:
        - name: consul-init
          emptyDir: {}
        - name: consul-secrets
          secret:
            secretName: {{ .Metadata.Name }}
            optional: true
`

var serviceAccountTemplate = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Metadata.Name }}
  labels:
    app.kubernetes.io/name: consul-acl-bootstrap
    app.kubernetes.io/instance: {{ .Metadata.Name }}
`

var roleTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ .Metadata.Name }}
  labels:
    app.kubernetes.io/name: consul-acl-bootstrap
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
  - apiGroups:
      - ""
    resources:
      - pods
    verbs:
      - get
      - list
  - apiGroups:
      - ""
    resources:
      - pods/exec
    verbs:
      - create
  - apiGroups:
      - ""
    resources:
      - services
    verbs:
      - get
    resourceNames:
      - {{ .Spec.ServiceName }}
`

var roleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ .Metadata.Name }}
  labels:
    app.kubernetes.io/name: consul-acl-bootstrap
    app.kubernetes.io/instance: {{ .Metadata.Name }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ .Metadata.Name }}
subjects:
  - kind: ServiceAccount
    name: {{ .Metadata.Name }}
`
