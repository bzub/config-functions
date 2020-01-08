package main

func (f *filter) defaultTemplates() map[string]string {
	return map[string]string{
		"cm":      cmTemplate,
		"sts":     stsTemplate,
		"svc":     svcTemplate,
		"dns-svc": dnsSvcTemplate,
		"ui-svc":  uiSvcTemplate,
	}
}

func (f *filter) gossipTemplates() map[string]string {
	return map[string]string{
		"gossip-job":         gossipJobTemplate,
		"gossip-sa":          gossipSATemplate,
		"gossip-role":        gossipRoleTemplate,
		"gossip-rolebinding": gossipRoleBindingTemplate,
	}
}

func (f *filter) tlsTemplates() map[string]string {
	return map[string]string{
		"tls-job":         tlsJobTemplate,
		"tls-sa":          tlsSATemplate,
		"tls-role":        tlsRoleTemplate,
		"tls-rolebinding": tlsRoleBindingTemplate,
		"tls-sts-patch":   tlsSTSPatchTemplate,
		"tls-cm-patch":    tlsCMPatchTemplate,
	}
}

func (f *filter) aclJobTemplates() map[string]string {
	return map[string]string{
		"acl-job":         aclJobTemplate,
		"acl-sa":          aclSATemplate,
		"acl-role":        aclRoleTemplate,
		"acl-rolebinding": aclRoleBindingTemplate,
	}
}

var cmTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Name }}
data:
  00-defaults.hcl: |-
    acl = {
      enabled = true
      default_policy = "allow"
      enable_token_persistence = true
    }

    connect = {
      enabled = true
    }
`

var stsTemplate = `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: {{ .Name }}
spec:
  serviceName: {{ .Name }}
  podManagementPolicy: Parallel
  updateStrategy:
    type: RollingUpdate
  template:
    spec:
      terminationGracePeriodSeconds: 10
      securityContext:
        fsGroup: 1000
      containers:
        - name: consul
          image: docker.io/library/consul:1.6.2
          command:
            - consul
            - agent
            - -advertise=$(POD_IP)
            - -bind=0.0.0.0
            - -bootstrap-expect=$(CONSUL_REPLICAS)
            - -client=0.0.0.0
            - -config-dir=/consul/config
            - -data-dir=/consul/data
            - -ui
            - -retry-join={{ .Name }}.$(NAMESPACE).svc.cluster.local
            - -server
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
            - name: NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
            - name: CONSUL_REPLICAS
              value: "{{ .Replicas }}"
          volumeMounts:
            - name: consul-data
              mountPath: /consul/data
            - name: consul-configs
              mountPath: /consul/config
          lifecycle:
            preStop:
              exec:
                command:
                - /bin/sh
                - -c
                - consul leave
          ports:
            - containerPort: 8500
              name: http
              protocol: "TCP"
            - containerPort: 8301
              name: serflan-tcp
              protocol: "TCP"
            - containerPort: 8301
              name: serflan-udp
              protocol: "UDP"
            - containerPort: 8302
              name: serfwan-tcp
              protocol: "TCP"
            - containerPort: 8302
              name: serfwan-udp
              protocol: "UDP"
            - containerPort: 8300
              name: server
              protocol: "TCP"
            - containerPort: 8600
              name: dns-tcp
              protocol: "TCP"
            - containerPort: 8600
              name: dns-udp
              protocol: "UDP"
          readinessProbe:
            exec:
              command:
                - "/bin/sh"
                - "-ec"
                - |
                  curl http://127.0.0.1:8500/v1/status/leader 2>/dev/null | \
                  grep -E '".+"'
            failureThreshold: 2
            initialDelaySeconds: 5
            periodSeconds: 3
            successThreshold: 1
            timeoutSeconds: 5
      volumes:
        - name: consul-data
          emptyDir: {}
        - name: consul-configs
          projected:
            sources:
              - configMap:
                  name: {{ .Name }}
`

var svcTemplate = `apiVersion: v1
kind: Service
metadata:
  name: {{ .Name }}
spec:
  clusterIP: None
  publishNotReadyAddresses: true
  ports:
    - name: http
      port: 8500
      targetPort: http
    - name: serflan-tcp
      protocol: "TCP"
      port: 8301
      targetPort: serflan-tcp
    - name: serflan-udp
      protocol: "UDP"
      port: 8301
      targetPort: serflan-udp
    - name: serfwan-tcp
      protocol: "TCP"
      port: 8302
      targetPort: serfwan-tcp
    - name: serfwan-udp
      protocol: "UDP"
      port: 8302
      targetPort: serfwan-udp
    - name: server
      port: 8300
      targetPort: server
    - name: dns-tcp
      protocol: "TCP"
      port: 8600
      targetPort: dns-tcp
    - name: dns-udp
      protocol: "UDP"
      port: 8600
      targetPort: dns-udp
`

var dnsSvcTemplate = `apiVersion: v1
kind: Service
metadata:
  name: {{ .Name }}-dns
spec:
  ports:
    - name: dns-tcp
      port: 53
      protocol: "TCP"
      targetPort: dns-tcp
    - name: dns-udp
      port: 53
      protocol: "UDP"
      targetPort: dns-udp
`

var uiSvcTemplate = `apiVersion: v1
kind: Service
metadata:
  name: {{ .Name }}-ui
spec:
  ports:
    - name: http
      port: 80
      targetPort: 8500
`

var gossipSecretVolumeTemplate = `
- secret:
    name: {{ .Name }}-{{ .Namespace }}-gossip
`

var gossipJobTemplate = `apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .Name }}-gossip-encryption
spec:
  template:
    spec:
      serviceAccountName: {{ .Name }}-gossip-encryption
      restartPolicy: OnFailure
      initContainers:
        - name: generate-gossip-encryption-config
          image: docker.io/library/consul:1.6.2
          command:
            - /bin/sh
            - -ec
            - |-
              config_file=/config/generated/01-gossip-encryption.json
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
              secret="{{ .Name }}-{{ .Namespace }}-gossip"
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

var gossipSATemplate = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Name }}-gossip-encryption
`

var gossipRoleTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ .Name }}-gossip-encryption
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

var gossipRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ .Name }}-gossip-encryption
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ .Name }}-gossip-encryption
subjects:
  - kind: ServiceAccount
    name: {{ .Name }}-gossip-encryption
`

// A patch to configure the StatefulSet to use agent TLS encryption.
var tlsSTSPatchTemplate = `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: {{ .Name }}
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
            secretName: {{ .Name }}-{{ .Namespace }}-agent-tls
        - name: tls
          emptyDir: {}
`

var tlsCMPatchTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Name }}
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

var tlsJobTemplate = `apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .Name }}-agent-tls
spec:
  template:
    spec:
      serviceAccountName: {{ .Name }}-agent-tls
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
              tls_dir="/tls/generated"
              prefix="{{ .Name }}-{{ .Namespace }}-agent-tls"

              secret="${prefix}"
              echo "[INFO] Creating \"secret/${secret}\"."
              kubectl create secret generic "${secret}" "--from-file=${tls_dir}"

              secret="${prefix}-ca"
              echo "[INFO] Creating \"secret/${secret}\"."
              kubectl create secret generic "${secret}" \
                "--from-file=${tls_dir}/consul-agent-ca.pem"

              secret="${prefix}-cli"
              echo "[INFO] Creating \"secret/${secret}\"."
              kubectl create secret generic "${secret}" \
                "--from-file=${tls_dir}/dc1-cli-consul-0.pem" \
                "--from-file=${tls_dir}/dc1-cli-consul-0-key.pem"
          volumeMounts:
            - mountPath: /tls/generated
              name: tls-generated
      volumes:
        - name: tls-generated
          emptyDir: {}
`

var tlsSATemplate = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Name }}-agent-tls
`

var tlsRoleTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ .Name }}-agent-tls
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

var tlsRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ .Name }}-agent-tls
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ .Name }}-agent-tls
subjects:
  - kind: ServiceAccount
    name: {{ .Name }}-agent-tls
`

var aclJobTemplate = `apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .Name }}-acl-bootstrap
spec:
  template:
    metadata:
    spec:
      serviceAccountName: {{ .Name }}-acl-bootstrap
      restartPolicy: OnFailure
      containers:
        - name: consul-acl-bootstrap
          image: k8s.gcr.io/hyperkube:v1.17.0
          command:
            - /bin/sh
            - -ec
            - |-
              consul_secret="{{ .Name }}-acl-bootstrap"
              exec_arg="sts/{{ .Name }}"

              metadata_dir="/metadata/consul-secrets"
              if [ "$(ls "${metadata_dir}"|wc -l)" = "0" ]; then
                metadata_dir="/metadata/consul-init"

                echo "[INFO] Performing consul acl bootstrap."
                export consul_bootstrap_out="$(kubectl exec "${exec_arg}" -- consul acl bootstrap)"
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
              kubectl exec "${exec_arg}" -- /bin/sh -c "CONSUL_HTTP_TOKEN=$(cat "${metadata_dir}/secret_id.txt") consul acl token list"
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
            secretName: {{ .Name }}-acl-bootstrap
            optional: true
`

var aclSATemplate = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Name }}-acl-bootstrap
`

var aclRoleTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ .Name }}-acl-bootstrap
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
      - apps
    resources:
      - statefulsets
    verbs:
      - get
`

var aclRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ .Name }}-acl-bootstrap
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ .Name }}-acl-bootstrap
subjects:
  - kind: ServiceAccount
    name: {{ .Name }}-acl-bootstrap
`

var sidecarPatchTemplate = `apiVersion: {{ .APIVersion }}
kind: {{ .Kind }}
metadata:
  name: {{ .ObjectMeta.Name }}
  namespace: {{ .ObjectMeta.Namespace }}
spec:
  template:
    spec:
      containers:
        - name: consul-agent
          image: docker.io/library/consul:1.6.2
          command:
            - consul
            - agent
            - -bind=$(POD_IP)
            - -retry-join={{ .ConsulServiceFQDN }}
            - -data-dir=/consul/data
            - -config-dir=/consul/configs
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
          volumeMounts:
            - name: consul-data
              mountPath: /consul/data
            - name: consul-configs
              mountPath: /consul/configs
            - name: consul-tls-secret
              mountPath: /consul/tls
      volumes:
        - name: consul-data
          emptyDir: {}
        - name: consul-tls
          emptyDir: {}
        - name: consul-configs
          projected:
            sources:
              - configMap:
                  name: {{ .ConsulName }}-{{ .ConsulNamespace }}-tls
                  optional: true
        - name: consul-tls-secret
          secret:
            secretName: {{ .ConsulName }}-{{ .ConsulNamespace }}-agent-tls-ca
            optional: true
`

var sidecarTLSCMTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .ConsulName }}-{{ .ConsulNamespace }}-tls
  namespace: {{ .ObjectMeta.Namespace }}
data:
  00-default-agent-tls.json: |-
    {
      "auto_encrypt": {
        "tls": true
      },
      "ca_file": "/consul/tls/consul-agent-ca.pem",
      "ports": {
        "http": -1,
        "https": 8501
      }
    }
`
