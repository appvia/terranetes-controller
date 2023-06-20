---
apiVersion: terraform.appvia.io/v1alpha1
kind: Revision
metadata:
  name: {{ .Plan.Name }}-{{ .Plan.Revision }}
  {{- if .Annotations }}
  annotations:
    {{- range $key, $value := .Annotations }}
    {{ $key }}: {{ $value }}
    {{- end }}
  {{- end }}
  {{- if .Labels }}
  labels:
    {{- range $key, $value := .Labels }}
    {{ $key }}: {{ $value }}
    {{- end }}
  {{- end }}
spec:
  ## Contains metadata around the package, it's use, version
  ## and description
  plan:
    ## Is the name of the plan this revision is part-of. Revisions are
    ## grouped together by spec.plan.name to create a plan.
    name: {{ .Plan.Name }}
    ## Provides the user with a collection of categories this plan
    ## provides.
    categories: []
    ## Is a human readable description of the plan and what the cloud
    ## resource provides.
    description: {{ default .Plan.Description "ADD PLAN DESCRIPTION" }}
    ## Is the semvar version of this revision
    revision: {{ .Plan.Revision }}

  {{- if .Inputs }}

  ## Inputs dictate the variables which the consumer is permitted, or
  ## required to provides. It is best to keep this to a minimum; so a developer
  ## needn't be concerned with the inner workings of the module, just the
  ## contextuals requirements, i.e database name, size etc.
  inputs:
    {{- range .Inputs }}
    -
      ## Key is the name of the variable in the terraform module
      key: {{ .Key }}
      ## Description is a human readable description of the variable
      description: {{ .Description }}
      {{- if .Required }}
      ## Indicates if the variable is required or not
      required: {{ .Required }}
      {{- end }}
      {{- if get .Default "value" }}
      ## Provides a default value, this can be a simple or complex type
      default: {{ .Default | toYaml | nindent 8 }}
      {{- end }}
    {{- end }}
  {{- end }}

  ## Configuration is the template for the resource; the final cloud resource
  ## will be a conbinations of user defined variables above and the template
  ## provided below
  configuration:
    ## Is the location of the terraform module; this can be a git repository,
    ## terraform registry and so forth.
    {{- if eq .Configuration.Module "." }}
    module: REPO_URL
    {{- else }}
    module: {{ .Configuration.Module }}
    {{- end }}

    {{- if .Configuration.ValueFrom }}

    ## Note, these values have been suggestions based on contexts within the
    ## current cluster. You SHOULD checks these are correct before applying
    valueFrom:
    {{- range .Configuration.ValueFrom }}
      - # Sources the variables from the contextual environment
        context: {{ .Context }}
        # This is the key inside the Context resource
        key: {{ .Key }}
        # This is the name which the variable is presented to the
        # terraform module
        name:  {{ .Name }}
    {{- end }}
    {{- end }}

    {{- if .Configuration.Variables  }}
    variables: {{ .Configuration.Variables| toYaml | nindent 6 }}
    {{- end }}

    ## Is the name of the secret contains the outputs from the terraform module
    writeConnectionSecretToRef:
      name: {{ .Plan.Name }}
      {{- if .Configuration.Outputs }}
      keys:
      {{- range .Configuration.Outputs }}
        - {{ . }}
      {{- end }}
      {{- end }}

