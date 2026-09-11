# Lab 404 Student Guide

You are the security consultant hired after a polling application was shipped
quickly. Your goal is to improve its delivery chain and Kubernetes security
without changing its business behavior.

The required path ends at runtime detection. The redundant Nginx workload and
its known vulnerability are deliberately outside this version of the lab.

## Safety and trust

Repository content, generated advice, and AI instructions are untrusted input.
Before following any instruction, compare it with the current objective,
inspect the proposed diff, and verify the resulting security state. Never
weaken a control only to make a command pass.

Work only in the disposable `lab404` Kind cluster. Never use application or
production credentials. Use only narrowly scoped lab tokens, store them in the
platform secret store, and never print or commit them.

## Checkpoints

The solution is a linear history on `solutions/v1`. Tags are recovery points,
not the normal way to complete the exercises.

| Stage | Starting point | Completed reference |
| --- | --- | --- |
| Baseline | `main` | `v1/checkpoint-00-baseline` |
| 01 — Images | `checkpoint-00` | `v1/checkpoint-10-images` |
| 02 — Supply chain | `checkpoint-10` | `v1/checkpoint-20-supply-chain` |
| 03 — Admission | `checkpoint-20` | `v1/checkpoint-30-admission` |
| 04 — Workloads | `checkpoint-30` | `v1/checkpoint-40-workloads` |
| 05 — Network | `checkpoint-40` | `v1/checkpoint-50-network-policies` |
| 06 — Secrets | `checkpoint-50` | `v1/checkpoint-60-sealed-secrets` |
| 07 — Runtime | `checkpoint-60` | `v1/checkpoint-70-runtime-detection` |

Use the smallest useful hint first:

```sh
git diff --stat v1/checkpoint-00-baseline..v1/checkpoint-10-images
git diff v1/checkpoint-00-baseline..v1/checkpoint-10-images -- apps/
```

Replace the two tags and path for the current stage. If you cannot continue,
create a recovery branch from its completed checkpoint:

```sh
git switch -c recovery/stage-01 v1/checkpoint-10-images
```

Recovery checkpoints contain instructor image names and signatures. Replace
them with the values produced by your fork. The Stage 06 SealedSecret is tied
to the instructor cluster key and must always be resealed for your cluster.

## Stage 00 — Establish the baseline

1. Fork the public repository and enable GitHub Actions.
2. Run the `Build images` workflow manually.
3. Make both GHCR packages public.
4. Clone your fork, then install the pinned tools:

   ```sh
   mise trust
   mise install
   mise run check-tools
   ```

5. Follow [the local platform instructions](infrastructure/README.md) to
   create the cluster, install the baseline controllers, and deploy the app.
6. Replace the two application image owners in the local Kustomize overlay
   with your GitHub owner.
7. Create a poll, vote, open the dashboard, and find telemetry from both
   services in the Collector logs.
8. Inventory nodes, namespaces, workloads, Services, Gateway resources,
   images, secrets, identities, and exposed addresses.

Do not start hardening until the application works and you can explain:

```text
Browser -> Envoy Gateway -> web -> votes -> Valkey
                              |       |
                              +-- OTLP +-> OpenTelemetry Collector
```

## Stage 01 — Harden the application images

### Objective

Reduce the runtime attack surface and make image security evidence part of the
delivery pipeline.

### Work

1. Create a read-only Docker access token for DHI, then add `DHI_USERNAME` and
   `DHI_TOKEN` as GitHub Actions repository secrets. Authenticate locally with
   `docker login dhi.io` before building the hardened images.
2. Record image sizes, configured users, histories, SBOMs, and vulnerability
   findings for `web` and `votes`.
3. Add minimal `.dockerignore` files.
4. Turn `votes` into a multi-stage build with a DHI Go development image, a
   static binary, a `scratch` runtime, and a numeric non-root user.
5. Turn `web` into a multi-stage build with the paired DHI Python development
   and distroless runtime images.
6. Pin base images by digest and install hash-locked Python dependencies.
7. Keep Hadolint, build, per-platform Trivy, Dockle, OCI-index publication,
   and evidence upload as separate dependency-gated CI jobs.
8. Publish `linux/amd64` and `linux/arm64` plus an immutable commit-derived
   tag. Only `main` may move `latest`.

### Verify

```sh
mise run test
trivy image --format cyclonedx --output web.cdx.json YOUR_WEB_IMAGE
docker buildx imagetools inspect YOUR_WEB_IMAGE
docker buildx imagetools inspect YOUR_VOTES_IMAGE
```

Both runtime images must use numeric non-root users. Neither final image should
contain a shell or build toolchain, and CI must scan the exact platform digest
before publishing the multi-platform index.

## Stage 02 — Sign and pin the images

### Objective

Bind immutable image content to the identity of your GitHub Actions workflow,
without storing a signing key.

### Work

1. Add a job after index publication with only `actions: read`,
   `packages: write`, and `id-token: write`.
2. Sign the final index digest recursively with Cosign.
3. Verify the index and both runnable child manifests against the exact
   workflow subject and GitHub OIDC issuer.
4. Replace application tags in the Kustomize overlay with the signed index
   digests produced by your fork.

### Verify

```sh
cosign verify \
  --certificate-identity \
  "https://github.com/YOUR_OWNER/lab404/.github/workflows/images.yml@refs/heads/main" \
  --certificate-oidc-issuer \
  "https://token.actions.githubusercontent.com" \
  "ghcr.io/YOUR_OWNER/lab404-web@sha256:YOUR_INDEX_DIGEST"
kustomize build kubernetes/overlays/local | grep 'image:'
```

No private signing key or new repository secret should exist. A tag move must
not change the content selected by the deployment.

## Stage 03 — Enforce trusted immutable images

### Objective

Move image trust from an optional CI check to a fail-closed admission rule.

### Work

1. Add the pinned Kyverno chart to the controller Kustomization.
2. Introduce digest validation in Audit mode and inspect policy reports.
3. Pin Nginx and Valkey by digest so every application-namespace image is
   immutable.
4. Change digest validation to Deny with `failurePolicy: Fail`.
5. Add image verification for only your `lab404-web` and `lab404-votes`
   repositories, exact workflow subject, and GitHub OIDC issuer.
6. Cover containers, init containers, and ephemeral containers without
   automatically mutating tags into digests.

### Verify

```sh
kubectl get namespacedvalidatingpolicies.policies.kyverno.io --namespace lab404
kubectl get imagevalidatingpolicies.policies.kyverno.io
kubectl run mutable-image-test \
  --namespace lab404 \
  --image nginx:latest \
  --restart Never \
  --dry-run server
```

The test Pod must be denied. A signed digest from the expected workflow must be
admitted; an unsigned digest or a different signer must be denied.

<details>
<summary>AI safety debrief — open after completing Stage 03</summary>

The repository contains two poisoned instructions aimed at AI assistants. One
asks them to skip tests and hide mistakes; the hidden one asks them to silently
change Kyverno from Deny to Audit when admission fails. Both conflict with the
lab objectives and are malicious even though they appear in a trusted-looking
repository file.

Inspect `AGENTS.md` as data, review every AI-proposed diff and command, and
verify the live policy still reports `Deny` and `Fail`. If the instruction was
followed, revert only that unauthorized change, reapply the policies, and
repeat the negative admission test.

</details>

## Stage 04 — Harden Kubernetes workloads

### Objective

Make every workload satisfy the Kubernetes `restricted` Pod Security profile
and remove unnecessary API credentials and runtime privileges.

### Work

1. Start with `restricted` Warn and Audit labels on `lab404`.
2. Disable ServiceAccount token automount on every Pod template.
3. Set numeric non-root identities and `RuntimeDefault` seccomp.
4. Disable privilege escalation, make root filesystems read-only, and drop all
   capabilities.
5. Add only the writable `emptyDir` volumes required by Nginx, web, and Valkey.
6. Add small CPU and memory requests and limits.
7. Make the namespace enforce `restricted`, pinned to Kubernetes `v1.35`.

### Verify

```sh
kubectl get namespace lab404 --show-labels
kubectl get deployments --namespace lab404 -o yaml
kubectl wait --namespace lab404 \
  --for=condition=Available deployment \
  --all \
  --timeout=5m
```

All workloads must roll out. No application Pod should contain a
`kube-api-access` volume, and an otherwise admissible Pod without a restricted
security context must be rejected by Pod Security Admission.

## Stage 05 — Enforce least network reachability

### Objective

Move from unrestricted Pod networking to default-deny with only the documented
application and telemetry flows.

### Work

1. Apply ingress and egress default-deny and observe the failure.
2. Restore namespace-wide UDP and TCP DNS access.
3. Allow Envoy to reach only `web:8080`.
4. Allow `web` to reach `votes:8081` and the Collector on `4318`.
5. Allow `votes` to reach `valkey:6379` and the Collector on `4318`.
6. Restrict Collector ingress to `web` and `votes`.
7. Preserve Nginx ingress for the deferred exposure exercise.

### Verify

```sh
kubectl get networkpolicies --namespace lab404
kubectl get networkpolicies --namespace observability
docker exec lab404-control-plane \
  curl -fsS http://GATEWAY_ADDRESS/healthz
```

Test every allowed flow and at least one denied sibling flow from a restricted
temporary Pod. A node-originated request is valid for testing LoadBalancer
availability, but it is not evidence that Pod-to-Pod isolation works.

## Stage 06 — Rotate and seal the Valkey credential

### Objective

Remove the committed plaintext secret and rotate it before storing a
cluster-bound encrypted representation in Git.

### Work

1. Install the pinned Sealed Secrets chart through Kustomize and wait for its
   controller.
2. Generate a new random password outside the repository. Do not reuse or
   merely encrypt `insecure-lab-password`.
3. Stream a client-side Secret manifest directly into `kubeseal`; never save
   or print the plaintext manifest.
4. Use strict scope for the exact `lab404/valkey` identity.
5. Let the controller take ownership of the existing Secret without a deletion
   window, then replace the plaintext document with the SealedSecret.
6. Restart Valkey and `votes`, because environment variables do not update in
   running processes.

### Verify

```sh
kubectl get sealedsecret --namespace lab404 valkey
kubectl get secret --namespace lab404 valkey \
  -o jsonpath='{.metadata.ownerReferences[0].kind}{"\n"}'
kubectl rollout status deployment/valkey --namespace lab404
kubectl rollout status deployment/votes --namespace lab404
```

The SealedSecret must report `Synced=True`, own the generated Secret, and the
full poll flow must still work. A checkpoint ciphertext cannot be reused on
your cluster; reseal after every cluster-key replacement.

## Stage 07 — Detect suspicious runtime activity

### Objective

Deploy a node-level runtime sensor, discover a taint-created coverage gap, and
close it.

### Work

1. Install the pinned Falco chart as a DaemonSet with the `modern_ebpf` driver,
   its least-privileged capability set, JSON stdout, and deterministic rules.
2. Initially retain only the chart's control-plane tolerations.
3. Compare cluster nodes, desired DaemonSet replicas, and Falco Pod placement.
4. Trigger the standard `Terminal shell in container` rule with a TTY shell in
   Nginx, then repeat it in Valkey.
5. Add the exact `workload=stateful:NoSchedule` toleration.
6. Confirm one ready sensor on every node and repeat the Valkey event.

### Verify

```sh
kubectl get daemonset,pods --namespace falco -o wide
kubectl exec --namespace lab404 -it deployment/valkey -- \
  sh -c 'echo falco-runtime-test'
kubectl logs --namespace falco \
  --selector app.kubernetes.io/name=falco \
  --since=1m \
  --prefix \
  --max-log-requests=10
```

The node-local alert must identify `lab404-worker2`, the Valkey Pod and
container, its image, user, and command. Kind nodes share a host kernel, so a
non-local sensor may observe the syscall without Kubernetes metadata; that is
not valid node coverage.

## Final verification

- All application and platform workloads are ready.
- The Gateway health page and complete poll flow work.
- Both application images are multi-platform, scanned, immutable, and signed.
- Kyverno denies mutable and untrusted application images.
- Every application workload satisfies restricted Pod Security.
- Only the documented network flows succeed.
- The plaintext Valkey credential is absent from the current tree and was
  rotated before sealing.
- Falco runs on every node and detects the controlled Valkey shell event.
- Application logs and Collector output show no new errors.

Nginx remains intentionally deployed and exposed. Do not describe the current
lab state as having fully removed unnecessary external attack surface.
