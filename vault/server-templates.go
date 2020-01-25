package vault

func (f *VaultFilter) serverTemplates() map[string]string {
	return map[string]string{
		"server-cm":  serverCmTemplate,
		"server-sts": serverStsTemplate,
		"server-svc": serverSvcTemplate,
	}
}

var serverCmTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Name }}-server
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
data:
  00-server-listener.hcl: |-
    listener "tcp" {
      address = "[::]:8200"
      cluster_address = "[::]:8201"
      tls_cert_file = "/vault/tls/server.pem"
      tls_key_file  = "/vault/tls/server-key.pem"
    }
  00-server-storage-backend.hcl: |-
    storage "file" {
      path = "/vault/data"
    }
`

var serverStsTemplate = `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: {{ .Name }}-server
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
spec:
  serviceName: {{ .Name }}-server
  podManagementPolicy: Parallel
  updateStrategy:
    type: RollingUpdate
  selector:
    matchLabels:
      app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
      app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
        app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
    spec:
      initContainers:
        - name: vault-server-tls-setup
          image: docker.io/library/alpine:3.11
          command:
            - /bin/sh
            - -ec
            - |-
              index="$(hostname|sed 's/.*-\(.*$\)/\1/')"
              cp "/vault/tls/secret/ca.pem" /vault/tls
              cp "/vault/tls/secret/${index}-server.pem" /vault/tls/server.pem
              cp "/vault/tls/secret/${index}-server-key.pem" /vault/tls/server-key.pem
          volumeMounts:
            - name: vault-server-tls-secret
              mountPath: /vault/tls/secret
            - name: vault-server-tls
              mountPath: /vault/tls
      containers:
        - name: vault-server
          image: docker.io/library/vault:1.3.2
          command:
            - /usr/local/bin/docker-entrypoint.sh
            - vault
            - server
            - -config=/vault/configs
          envFrom:
            - configMapRef:
                name: {{ .Name }}
          env:
            - name: VAULT_ADDR
              value: https://127.0.0.1:8200
              name: VAULT_CACERT
              value: /vault/tls/ca.pem
          lifecycle:
            preStop:
              exec:
                command:
                  - vault
                  - step-down
          ports:
            - name: https
              containerPort: 8200
            - name: internal
              containerPort: 8201
            - name: replication
              containerPort: 8202
          readinessProbe:
            exec:
              command:
                - /bin/sh
                - -ec
                - vault status
          securityContext:
            capabilities:
              add:
                - IPC_LOCK
          volumeMounts:
            - name: vault-configs
              mountPath: /vault/configs
              readOnly: true
            - name: vault-server-tls
              mountPath: /vault/tls
              readOnly: true
      volumes:
        - name: vault-configs
          projected:
            sources:
              - configMap:
                  name: {{ .Name }}-server
        - name: vault-server-tls
        - name: vault-server-tls-secret
          projected:
            sources:
              - secret:
                  name: {{ .Name }}-server-tls
`

var serverSvcTemplate = `apiVersion: v1
kind: Service
metadata:
  name: {{ .Name }}-server
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
spec:
  selector:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
  publishNotReadyAddresses: true
  ports:
    - name: https
      port: 8200
      targetPort: 8200
    - name: internal
      port: 8201
      targetPort: 8201
`
