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
  # retain the jobs for 6 hours
  ttlSecondsAfterFinished: 28800
  template:
    metadata:
      labels:
        {{- range $key, $value := .Labels }}
        {{ $key }}: "{{ $value }}"
        {{- end }}
        aadpodidbinding: terranetes-executor
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
        {{- if and (.Policy) (not .Policy.Source) (eq .Stage "plan") }}
        - name: checkov
          secret :
            secretName: {{ .Secrets.Config }}
            optional: true
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
            - --command=/bin/source --dest=/data --source={{ .Configuration.Module }}
          env:
            - name: HOME
              value: /data
          envFrom:
          {{- if .Secrets.Config }}
            - secretRef:
                name: {{ .Secrets.Config }}
                optional: false
          {{- end }}
          {{- range .Secrets.AdditionalSecrets }}
            - secretRef:
                name: {{ . }}
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
          envFrom:
          {{- range .Secrets.AdditionalSecrets }}
            - secretRef:
                name: {{ . }}
                optional: false
          {{- end }}
          securityContext:
            capabilities:
              drop: [ALL]
          volumeMounts:
            - name: source
              mountPath: /data

        {{- if and (.Policy) (.Policy.Source) (eq .Stage "plan") }}
        - name: policy-source
          image: {{ .Images.Executor }}
          imagePullPolicy: {{ .ImagePullPolicy }}
          workingDir: /run
          command:
            - /run/bin/step
          args:
            - --comment=Retrieve policy source
            - --command=/bin/source --dest=/run/checkov --source={{ .Policy.Source.URL }}
          envFrom:
          {{- if and (.Policy.Source.SecretRef) (.Policy.Source.SecretRef.Name) }}
            - secretRef
                name: {{ .Policy.Source.SecretRef.Name }}
          {{- end }}
          volumeMounts:
            - name: run
              mountPath: /run
        {{- end }}

        {{- if and (.Policy) (not .Policy.Source) (eq .Stage "plan") }}
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
          envFrom:
          {{- if and (.SecretRef) (.SecretRef.Name) }}
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
          - --command=/bin/terraform state pull > /run/tfstate
          - --command=/bin/gzip /run/tfstate
          - --command=/bin/mv /run/tfstate.gz /run/tfstate
          - --namespace=$(KUBE_NAMESPACE)
          - --upload=$(TERRAFORM_STATE_NAME)=/run/tfstate
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
        {{- range .Secrets.AdditionalSecrets }}
          - secretRef:
              name: {{ . }}
              optional: false
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
          - --namespace=$(KUBE_NAMESPACE)
          - --upload=$(COST_REPORT_NAME)=/run/costs.json
          - --is-failure=/run/steps/terraform.failed
          - --timeout=5m
          - --wait-on=/run/steps/terraform.complete
        env:
          - name: COST_REPORT_NAME
            value: {{ .Secrets.InfracostsReport }}
          - name: INFRACOST_SKIP_UPDATE_CHECK
            value: "true"
          - name: KUBE_NAMESPACE
            valueFrom:
              fieldRef:
                fieldPath: metadata.namespace
        envFrom:
        {{- if .Secrets.Infracosts }}
          - secretRef:
              name: {{ .Secrets.Infracosts }}
        {{- end }}
        {{- range .ExecutorSecrets }}
          - secretRef:
              name: {{ . }}
              optional: true
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
      {{- $configfile := "/run/checkov/checkov.yaml" }}
      {{- $options := "--framework terraform_plan -f /run/plan.json --soft-fail -o json -o cli --output-file-path /run --repo-root-for-plan-enrichment /data" }}
      {{- if .Policy.Source }}
      {{- $configfile = printf "%s/%s" "/run/checkov" .Policy.Source.Configuration }}
      {{- end }}
      - name: verify-policy
        image: {{ .Images.Policy }}
        imagePullPolicy: {{ .ImagePullPolicy }}
        workingDir: /data
        command:
          - /run/bin/step
        args:
          - --comment=Evaluating Against Security Policy
          - --command=/usr/local/bin/checkov --config {{ $configfile }} {{ $options }} >/dev/null
          - --command=/bin/cat /run/results_cli.txt
          - --namespace=$(KUBE_NAMESPACE)
          - --upload=$(POLICY_REPORT_NAME)=/run/results_json.json
          - --is-failure=/run/steps/terraform.failed
          - --wait-on=/run/steps/terraform.complete
        env:
          - name: KUBE_NAMESPACE
            valueFrom:
              fieldRef:
                fieldPath: metadata.namespace
          - name: POLICY_REPORT_NAME
            value: {{ .Secrets.PolicyReport }}
        envFrom:
        {{- if .Secrets.Policy }}
          - secretRef:
              name: {{ .Secrets.Policy }}
              optional: true
        {{- end }}
        {{- range .ExecutorSecrets }}
          - secretRef:
              name: {{ . }}
              optional: true
        {{- end }}
        securityContext:
          capabilities:
            drop: [ALL]
        volumeMounts:
          {{- if not .Policy.Source }}
          - name: checkov
            mountPath: /run/checkov
          {{- end }}
          - name: run
            mountPath: /run
          - name: source
            mountPath: /data
      {{- end }}
