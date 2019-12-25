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
		// agent TLS encryption.
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
	cm, err := api.parseNewTemplate("configmap", configMapTemplate)
	if err != nil {
		return nil, err
	}

	in = append(in, job, sa, role, rolebinding, stsPatch, cm)
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

// A patch to configure the StatefulSet to use agent TLS encryption.
var statefulSetPatchTemplate = `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: {{ .Spec.StatefulSetName }}
spec:
  template:
    spec:
      initContainers:
        - name: consul-server-tls-setup
          image: docker.io/library/alpine:3.11
          command:
            - /bin/sh
            - -ec
            - |-
              index="$(hostname|sed 's/.*-\(.*$\)/\1/')"
              cp /consul/tls/secret/consul-agent-ca.pem /consul/tls
              cp /consul/tls/secret/dc1-server-consul-${index}.pem \
                 /consul/tls/server-consul.pem
              cp /consul/tls/secret/dc1-server-consul-${index}-key.pem \
                 /consul/tls/server-consul-key.pem
              cp /consul/tls/secret/dc1-cli-consul-0.pem /consul/tls
              cp /consul/tls/secret/dc1-cli-consul-0-key.pem /consul/tls
          volumeMounts:
            - name: tls-secret
              mountPath: /consul/tls/secret
            - name: tls
              mountPath: /consul/tls
      containers:
        - name: consul
          env:
            - name: CONSUL_HTTP_ADDR
              value: https://127.0.0.1:8501
            - name: CONSUL_CACERT
              value: /consul/tls/consul-agent-ca.pem
            - name: CONSUL_CLIENT_CERT
              value: /consul/tls/dc1-cli-consul-0.pem
            - name: CONSUL_CLIENT_KEY
              value: /consul/tls/dc1-cli-consul-0-key.pem
          ports:
            - containerPort: 8501
              name: http
              protocol: "TCP"
          readinessProbe:
            exec:
              command:
                - /bin/sh
                - -ec
                - |
                  curl \
                    --cacert $(CONSUL_CACERT) \
                    --cert $(CONSUL_CLIENT_CERT) \
                    --key $(CONSUL_CLIENT_KEY) \
                    $(CONSUL_HTTP_ADDR)/v1/status/leader 2>/dev/null |\
                  grep -E '".+"'
          volumeMounts:
            - name: tls
              mountPath: /consul/tls
      volumes:
        - name: tls-secret
          secret:
            secretName: {{ .Metadata.Name }}
        - name: tls
          emptyDir: {}
`

var configMapTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Spec.StatefulSetName }}
data:
  00-default-agent-tls.json: |-
    {
      "verify_incoming": true,
      "verify_outgoing": true,
      "verify_server_hostname": true,
      "auto_encrypt": {
        "allow_tls": true
      },
      "ca_file": "/consul/tls/consul-agent-ca.pem",
      "cert_file": "/consul/tls/server-consul.pem",
      "key_file": "/consul/tls/server-consul-key.pem",
      "ports": {
        "http": -1,
        "https": 8501
      }
    }
`

var jobTemplate = `apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .Metadata.Name }}
spec:
  template:
    metadata:
      labels:
        app.kubernetes.io/name: consul-agent-tls-encryption
        app.kubernetes.io/instance: {{ .Metadata.Name }}
    spec:
      serviceAccountName: {{ .Metadata.Name }}
      restartPolicy: OnFailure
      initContainers:
        - name: generate-agent-tls
          image: docker.io/library/consul:1.6.2
          command:
            - /bin/sh
            - -ec
            - |-
              tls_dir=/tls/generated
              cd "${tls_dir}"
              consul tls ca create
              consul tls cert create -cli
              consul tls cert create -server
              consul tls cert create -server
              consul tls cert create -server
          volumeMounts:
            - mountPath: /tls/generated
              name: tls-generated
      containers:
        - name: create-agent-tls-secret
          image: k8s.gcr.io/hyperkube:v1.17.0
          command:
            - /bin/sh
            - -ec
            - |-
              secret="{{ .Metadata.Name }}"
              tls_dir="/tls/generated"

              if [ ! "$(kubectl get secret --no-headers "${secret}"|wc -l)" = "0" ]; then
                echo "[INFO] \"secret/${secret}\" already exists. Exiting."
                exit 0
              fi

              echo "[INFO] Creating \"secret/${secret}\"."
              kubectl create secret generic "--from-file=${tls_dir}" "${secret}"
          volumeMounts:
            - mountPath: /tls/generated
              name: tls-generated
      volumes:
        - name: tls-generated
          emptyDir: {}
`

var serviceAccountTemplate = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Metadata.Name }}
  labels:
    app.kubernetes.io/name: consul-agent-tls-encryption
    app.kubernetes.io/instance: {{ .Metadata.Name }}
`

var roleTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ .Metadata.Name }}
  labels:
    app.kubernetes.io/name: consul-agent-tls-encryption
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
    app.kubernetes.io/name: consul-agent-tls-encryption
    app.kubernetes.io/instance: {{ .Metadata.Name }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ .Metadata.Name }}
subjects:
  - kind: ServiceAccount
    name: {{ .Metadata.Name }}
`
