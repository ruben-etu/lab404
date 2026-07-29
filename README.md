# Lab 404

Lab 404 is a guided cloud-native security lab. Students inherit a working
polling application that was shipped quickly, then improve its delivery chain
and Kubernetes security posture.

## Application

```text
Browser -> Envoy Gateway -> Python web -> Go votes -> Valkey
                                  |           |
                                  +---- OTLP -+-> OpenTelemetry Collector
```

Users can create public polls, share their URLs, vote anonymously, and view a
dashboard of current results.

## Repository layout

- `apps/` contains the complete application source and initial Dockerfiles.
- `infrastructure/` contains Kind configuration and pinned platform setup.
- `kubernetes/` contains the application Kustomize base and local overlay.
- `.github/workflows/` builds public multi-platform images in GHCR.

## Start here

1. Fork this public repository.
2. Enable GitHub Actions and run the **Build images** workflow.
3. Make the generated `lab404-web` and `lab404-votes` GHCR packages public.
4. Clone your fork.
5. Inspect `mise.toml`, then run `mise trust` and `mise install`.
6. Follow [the local platform instructions](infrastructure/README.md).

The initial deployment is intentionally insecure. Do not use it as a production
reference.
