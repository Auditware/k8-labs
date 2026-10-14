package labs

import _ "embed"

// Nested modules ship their attack target as a manifest bundle applied into the
// per-session nested cluster (see NestedSpec / applyNestedManifests), embedded
// here so it travels in the backend binary - the backend, not ArgoCD, applies
// it (the nested cluster is created per session and gone at teardown). The
// human-facing writeup lives in services/labs/<slug>/README.md.

//go:embed modules/ns-escape.yaml
var nsEscapeManifests string

//go:embed modules/suid-privesc.yaml
var suidPrivescManifests string

//go:embed modules/ebpf-detect.yaml
var ebpfDetectManifests string

//go:embed modules/driver-research.yaml
var driverResearchManifests string

//go:embed modules/sa-impersonate.yaml
var saImpersonateManifests string

//go:embed modules/recon-first-contact.yaml
var reconFirstContactManifests string

//go:embed modules/recon-who-am-i.yaml
var reconWhoAmIManifests string

//go:embed modules/recon-neighborhood.yaml
var reconNeighborhoodManifests string

//go:embed modules/pod-exec.yaml
var podExecManifests string

//go:embed modules/create-pods.yaml
var createPodsManifests string

//go:embed modules/rolebinding-escalate.yaml
var rolebindingEscalateManifests string

//go:embed modules/sidecar-inject.yaml
var sidecarInjectManifests string

//go:embed modules/hostpath-escape.yaml
var hostpathEscapeManifests string

//go:embed modules/privileged-escape.yaml
var privilegedEscapeManifests string

//go:embed modules/node-token-harvest.yaml
var nodeTokenHarvestManifests string

//go:embed modules/ebpf-egress.yaml
var ebpfEgressManifests string

//go:embed modules/imagepullsecret-theft.yaml
var imagePullSecretTheftManifests string

//go:embed modules/static-pod-persist.yaml
var staticPodPersistManifests string

// malicious-webhook ships its webhook server as a gzipped, chunked binary inside
// this (generated) bundle - hence the size. See services/labs/malicious-webhook.
//
//go:embed modules/malicious-webhook.yaml
var maliciousWebhookManifests string
