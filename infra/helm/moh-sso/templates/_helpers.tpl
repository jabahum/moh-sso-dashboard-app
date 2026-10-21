{{/* =========================================================
   Name helpers
========================================================= */}}

{{- define "moh-sso.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "moh-sso.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "moh-sso.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "moh-sso.labels" -}}
app.kubernetes.io/name: {{ include "moh-sso.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{- end -}}


{{/* =========================================================
   Keycloak Resolution Logic (Internal vs External)
========================================================= */}}

{{/*
Fail fast if both internal and external are enabled
*/}}
{{- define "moh-sso.keycloak.validate" -}}
{{- if and .Values.keycloak.enabled .Values.keycloak.external.enabled -}}
{{- fail "You cannot enable both internal and external Keycloak at the same time." -}}
{{- end -}}
{{- end -}}


{{/*
Resolve Keycloak Base URL
*/}}
{{- define "moh-sso.keycloak.baseUrl" -}}
{{- include "moh-sso.keycloak.validate" . -}}
{{- if .Values.keycloak.external.enabled -}}
{{- .Values.keycloak.external.baseUrl -}}
{{- else -}}
{{- printf "http://%s-keycloak:%d" .Release.Name (.Values.keycloak.service.port | int) -}}
{{- end -}}
{{- end -}}


{{/*
Resolve Keycloak Realm
*/}}
{{- define "moh-sso.keycloak.realm" -}}
{{- include "moh-sso.keycloak.validate" . -}}
{{- if .Values.keycloak.external.enabled -}}
{{- .Values.keycloak.external.realm -}}
{{- else -}}
moh-realm
{{- end -}}
{{- end -}}


{{/* =========================================================
   Environment Helpers
========================================================= */}}

{{- define "moh-sso.frontend.baseUrl" -}}
{{- if .Values.frontend.baseUrl -}}
{{- .Values.frontend.baseUrl -}}
{{- else if .Values.ingress.enabled -}}
https://{{ .Values.ingress.host }}
{{- else -}}
http://localhost:3000
{{- end -}}
{{- end -}}


{{- define "moh-sso.backend.baseUrl" -}}
{{- if .Values.backend.baseUrl -}}
{{- .Values.backend.baseUrl -}}
{{- else if .Values.ingress.enabled -}}
https://{{ .Values.ingress.host }}
{{- else -}}
http://localhost:9000
{{- end -}}
{{- end -}}


{{- define "moh-sso.keycloak.redirectUri" -}}
{{ include "moh-sso.backend.baseUrl" . }}/api/v1/auth/callback
{{- end -}}

{{/* =========================================================
   Environment Mode Helpers
========================================================= */}}

{{- define "moh-sso.environment" -}}
{{- if .Values.global }}
{{- .Values.global.environment | default "dev" -}}
{{- else -}}
dev
{{- end -}}
{{- end -}}



{{- define "moh-sso.ginMode" -}}
{{- $env := include "moh-sso.environment" . -}}
{{- if or (eq $env "development") (eq $env "dev") (eq $env "local") -}}
debug
{{- else -}}
release
{{- end -}}
{{- end -}}


{{/* =========================================================
   Backend Public URL (for frontend consumption)
========================================================= */}}

{{- define "moh-sso.backend.publicUrl" -}}
{{- if .Values.frontend.apiBaseUrl -}}
{{- .Values.frontend.apiBaseUrl -}}
{{- else if .Values.ingress.enabled -}}
https://{{ .Values.ingress.host }}
{{- else -}}
http://{{ .Release.Name }}-backend:{{ .Values.backend.service.port }}
{{- end -}}
{{- end -}}


{{/* =========================================================
   Keycloak external  URL f
========================================================= */}}

{{- define "moh-sso.keycloakPublicURL" -}}
{{- include "moh-sso.keycloak.validate" . -}}

{{- if .Values.keycloak.external.enabled -}}
{{- .Values.keycloak.external.baseUrl -}}

{{- else if .Values.keycloakIngress.enabled -}}
https://{{ .Values.keycloakIngress.host }}

{{- else if .Values.keycloak.publicBaseUrl -}}
{{- .Values.keycloak.publicBaseUrl -}}

{{- else -}}
{{- printf "http://%s-keycloak:%d" .Release.Name (.Values.keycloak.service.port | int) -}}

{{- end -}}
{{- end -}}



#  login redirect url
{{- define "moh-sso.keycloak.loginRedirectUri" -}}
{{ include "moh-sso.backend.baseUrl" . }}/api/v1/auth/login
{{- end -}}
