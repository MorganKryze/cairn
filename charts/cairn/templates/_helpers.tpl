{{/*
The name every resource in the release takes.

`helm install cairn …` gives plain "cairn" rather than "cairn-cairn"; any
other release name is prefixed, so two cairns can share a namespace.

No nameOverride / fullnameOverride: nobody has asked, and adding a value later
breaks nothing for anyone.
*/}}
{{- define "cairn.fullname" -}}
{{- if eq .Release.Name .Chart.Name -}}
{{- .Chart.Name -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/* What the Service selects on: immutable once installed, so it stays small. */}}
{{- define "cairn.selectorLabels" -}}
app.kubernetes.io/name: cairn
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "cairn.labels" -}}
{{ include "cairn.selectorLabels" . }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}
