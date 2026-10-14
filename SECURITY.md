# Security Policy

## Labs are intentionally vulnerable

K8 Labs is a security training platform. **Every lab ships with deliberate vulnerabilities** - weak
RBAC, SUID command injection, `hostPath` mounts, privileged pods, a vulnerable kernel driver, and so
on. These are the lessons, not bugs. Please do **not** report intentionally-vulnerable lab content as
a security issue.

Lab exploits only ever affect a disposable, isolated nested cluster inside a microVM. Do not deploy
the lab manifests into a cluster you care about, and do not reuse their (deliberately weak)
configurations anywhere real.

## Reporting a vulnerability in the platform

If you find a vulnerability in the **platform itself** - the backend, the frontend, the session
provisioning, the isolation model (a way to escape a lab's microVM/nested-cluster boundary and reach
the host or another tenant), the auth flow, or the deployment charts - please report it privately:

- Use GitHub's **private vulnerability reporting**: open the repository's **Security** tab and choose
  **Report a vulnerability**.

Please do not open a public issue for platform vulnerabilities until they have been addressed.

### What to include

- A clear description of the issue and its impact.
- Steps to reproduce (a proof of concept if you have one).
- The affected component and, if known, a suggested fix.

### Isolation escapes

Escaping the microVM or nested-cluster boundary to reach the host, the control plane, or another
session is the highest-severity class of issue for this project. If you find one, report it privately
with as much detail as you can.

Thanks for helping keep K8 Labs safe.
