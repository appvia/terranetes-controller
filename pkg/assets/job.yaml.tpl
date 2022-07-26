---
apiVersion: batch/v1
kind: Job
metadata:
  generateName: {{ .GenerateName }}
  namespace: {{ .Namespace }}
  labels:
    {{- range $key, $value := .Labels }}
    {{ $key }}: "{{ $value }}"
    {{- end }}
spec:
  backoffLimit: 2
  completions: 1
  parallelism: 1
  template:
    metadata:
      labels:
        {{- range $key, $value := .Labels }}
        {{ $key }}: "{{ $value }}"
        {{- end }}
    spec:
      # https://github.com/kubernetes/kubernetes/issues/74848
      restartPolicy: Never
      {{- if eq .Provider.Source "injected" }}
      serviceAccountName: {{ .Provider.ServiceAccount }}
      {{- else }}
      serviceAccountName: {{ .ServiceAccount }}
      {{- end }}
      securityContext:
        runAsUser: 65534
        runAsGroup: 65534
        fsGroup: 65534
      # retain the jobs for 6 hours
      ttlSecondsAfterFinished: 28800
      volumes:
        # Used to hold the terraform module source code
        - name: source
          emptyDir: {}
        # Used to hold runner scripts and shared state
        - name: run
          emptyDir: {}
        # These contains auto generated configuation required for the job
        - name: config
          secret:
            secretName: {{ .Secrets.Config }}
            optional: false
            items:
              - key: backend.tf
                path: backend.tf
              - key: provider.tf
                path: provider.tf
              {{- if .EnableVariables }}
              - key: variables.tfvars.json
                path: variables.tfvars.json
              {{- end }}
        {{- if and (.Policy) (eq .Stage "plan") }}
        - name: checkov
          secret :
            secretName: {{ .Secrets.Config }}
            optional: false
            items:
              - key: checkov.yaml
                path: checkov.yaml
        {{- end }}

      initContainers:
        - name: setup
          image: {{ .Images.Executor }}
          imagePullPolicy: {{ .ImagePullPolicy }}
          command:
            - /bin/step
          args:
            - --comment='Setting up the environment'
            - --command=/bin/mkdir -p /run/bin
            - --command=/bin/mkdir -p /run/steps
            - --command=/bin/cp /run/config/* /data
            - --command=/bin/cp /bin/step /run/bin/step
            - --command=/bin/cp /bin/kubectl /run/bin/kubectl
            - --command=/bin/source --dest=/data --source={{ .Configuration.Module }}
          {{- if .Secrets.Config }}
          envFrom:
            - secretRef:
                name: {{ .Secrets.Config }}
                optional: false
          {{- end }}
          volumeMounts:
            - name: config
              mountPath: /run/config
              reaonly: true
            - name: run
              mountPath: /run
            - name: source
              mountPath: /data

        - name: init
          image: {{ .Images.Terraform }}
          workingDir: /data
          command:
            - /bin/terraform
          args:
            - init
          securityContext:
            capabilities:
              drop: [ALL]
          volumeMounts:
            - name: source
              mountPath: /data

        {{- if and (.Policy) (eq .Stage "plan") }}
        {{- $image := .Images.Executor }}
        {{- $imagePullPolicy := .ImagePullPolicy }}
        {{- range .Policy.External }}
        - name: policy-external-{{ .Name }}
          image: {{ $image }}
          imagePullPolicy: {{ $imagePullPolicy }}
          workingDir: /run
          command:
            - /run/bin/step
          args:
            - --comment=Retrieve external source for {{ .Name }}
            - --command=/bin/mkdir -p /run/policy
            - --command=/bin/source --dest=/run/policy/{{ .Name }} --source={{ .URL }}
          {{- if and (.SecretRef) (.SecretRef.Name) }}
          envFrom:
            - secretRef:
                name: {{ .SecretRef.Name }}
          {{- end }}
          volumeMounts:
            - name: run
              mountPath: /run
        {{- end }}
        {{- end }}
      containers:
      - name: {{ .TerraformContainerName }}
        image: {{ .Images.Terraform }}
        imagePullPolicy: {{ .ImagePullPolicy }}
        workingDir: /data
        command:
          - /run/bin/step
        args:
          - --comment=Executing Terraform
          {{- if eq .Stage "plan" }}
          - --command=/bin/terraform plan {{ .TerraformArguments }} -out=/run/plan.out -lock=false
          - --command=/bin/terraform show -json /run/plan.out > /run/plan.json
          {{- end }}
          {{- if eq .Stage "apply" }}
          - --command=/bin/terraform apply {{ .TerraformArguments }} -auto-approve -lock=false
          {{- if .SaveTerraformState }}
          - --command=/bin/terraform state pull > /run/terraform.tfstate
          - --command=/bin/gzip /run/terraform.tfstate
          - --command=/run/bin/kubectl -n $(KUBE_NAMESPACE) delete secret $(TERRAFORM_STATE_NAME) --ignore-not-found >/dev/null
          - --command=/run/bin/kubectl -n $(KUBE_NAMESPACE) create secret generic $(TERRAFORM_STATE_NAME) --from-file=tfstate=/run/terraform.tfstate.gz >/dev/null
          {{- end }}
          {{- end }}
          {{- if eq .Stage "destroy" }}
          - --command=/bin/terraform destroy {{ .TerraformArguments }} -auto-approve
          {{- end }}
          - --on-error=/run/steps/terraform.failed
          - --on-success=/run/steps/terraform.complete
        env:
          - name: CONFIGURATION_NAME
            value: {{ .Configuration.Name }}
          - name: CONFIGURATION_NAMESPACE
            value: {{ .Configuration.Namespace }}
          - name: CONFIGURATION_UUID
            value: "{{ .Configuration.UUID }}"
          - name: KUBE_NAMESPACE
            valueFrom:
              fieldRef:
                fieldPath: metadata.namespace
          - name: TERRAFORM_STATE_NAME
            value: {{ .Secrets.TerraformState }}
        envFrom:
        {{- if eq .Provider.Source "secret" }}
          - secretRef:
              name: {{ .Provider.SecretRef.Name }}
        {{- end }}
        {{- range .ExecutorSecrets }}
          - secretRef:
              name: {{ . }}
              optional: true
        {{- end }}
        resources:
          limits:
            cpu: 1
            memory: 1Gi
          requests:
            cpu: 5m
            memory: 32Mi
        securityContext:
          capabilities:
            drop: [ALL]
        volumeMounts:
          - name: run
            mountPath: /run
          - name: source
            mountPath: /data

      {{- if and (.EnableInfraCosts) (eq .Stage "plan") }}
      - name: costs
        image: {{ .Images.Infracosts }}
        imagePullPolicy: {{ .ImagePullPolicy }}
        command:
          - /run/bin/step
        args:
          - --comment=Evaluating the costs
          - --command=/usr/bin/infracost breakdown --path /run/plan.json
          - --command=/usr/bin/infracost breakdown --path /run/plan.json --format json > /run/costs.json
          - --command=/run/bin/kubectl -n $(KUBE_NAMESPACE) delete secret $(COST_REPORT_NAME) --ignore-not-found >/dev/null
          - --command=/run/bin/kubectl -n $(KUBE_NAMESPACE) create secret generic $(COST_REPORT_NAME) --from-file=/run/costs.json >/dev/null
          - --is-failure=/run/steps/terraform.failed
          - --timeout=5m
          - --wait-on=/run/steps/terraform.complete
        env:
          - name: COST_REPORT_NAME
            value: {{ .Secrets.InfracostsReport }}
          - name: KUBE_NAMESPACE
            valueFrom:
              fieldRef:
                fieldPath: metadata.namespace
        {{- if .Secrets.Infracosts }}
        envFrom:
          - secretRef:
              name: {{ .Secrets.Infracosts }}
        {{- end }}
        securityContext:
          capabilities:
            drop: [ALL]
        volumeMounts:
          - name: run
            mountPath: /run
          - name: source
            mountPath: /data
      {{- end }}

      {{- if and (.Policy) (eq .Stage "plan") }}
      - name: verify-policy
        image: {{ .Images.Policy }}
        imagePullPolicy: {{ .ImagePullPolicy }}
        workingDir: /data
        command:
          - /run/bin/step
        args:
          - --comment=Evaluating Against Security Policy
          - --command=/usr/local/bin/checkov --config /run/checkov/checkov.yaml -f /run/plan.json -o json -o cli --output-file-path /run >/dev/null
          - --command=/bin/cat /run/results_cli.txt
          - --command=/run/bin/kubectl -n $(KUBE_NAMESPACE) delete secret $(POLICY_REPORT_NAME) --ignore-not-found >/dev/null
          - --command=/run/bin/kubectl -n $(KUBE_NAMESPACE) create secret generic $(POLICY_REPORT_NAME) --from-file=/run/results_json.json >/dev/null
          - --is-failure=/run/steps/terraform.failed
          - --wait-on=/run/steps/terraform.complete
        env:
          - name: KUBE_NAMESPACE
            valueFrom:
              fieldRef:
                fieldPath: metadata.namespace
          - name: POLICY_REPORT_NAME
            value: {{ .Secrets.PolicyReport }}
        {{- if .Secrets.Policy }}
        envFrom
          - secretRef:
              name: {{ .Secrets.Policy }}
              optional: false
        {{- end }}
        securityContext:
          capabilities:
            drop: [ALL]
        volumeMounts:
          - name: checkov
            mountPath: /run/checkov
          - name: run
            mountPath: /run
          - name: source
            mountPath: /data
      {{- end }}
