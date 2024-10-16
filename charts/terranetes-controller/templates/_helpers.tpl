{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "terranetes-controller.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "terranetes-controller.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "terranetes-controller.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "terranetes-controller.labels" -}}
helm.sh/chart: {{ include "terranetes-controller.chart" . }}
{{ include "terranetes-controller.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "terranetes-controller.selectorLabels" -}}
app.kubernetes.io/name: {{ include "terranetes-controller.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "terranetes-controller.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "terranetes-controller.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Generate a signed certificate using `genSignedCert` or include the logic for generating a cert.
Replace this with the actual logic if using an external tool or script.
*/}}
{{- define "webhook.generateCerts" -}}
{{- $secret := lookup "v1" "Secret" .Release.Namespace .Values.controller.webhooks.caSecret }}
{{- if $secret }}
  {{- $_ := set .Values.controller.webhooks "caBundle" (index $secret.data "ca.pem") -}}
  {{- $_ := set .Values.controller.webhooks "cert" (index $secret.data "tls.pem") -}} 
  {{- $_ := set .Values.controller.webhooks "key" (index $secret.data "tls-key.pem") -}}
{{- else }}
  {{- if not .Values.controller.webhooks.caBundle }}
    {{ $ca := genCA "terranetes-controller" 7300 }}
    {{ $dn := printf "controller.%s.svc.cluster.local" .Release.Namespace }}
    {{ $sn := printf "controller.%s.svc" .Release.Namespace }}
    {{ $server := genSignedCert "" (list "127.0.0.1") (list "localhost" "controller" $sn $dn) 3650 $ca }}
    {{- $_ := set .Values.controller.webhooks "caBundle" ($ca.Cert | b64enc) -}}
    {{- $_ := set .Values.controller.webhooks "cert" ($server.Cert | b64enc) -}}
    {{- $_ := set .Values.controller.webhooks "key" ($server.Key | b64enc) -}}
  {{- end }}
{{- end }}
{{- end }}
