# ADR 0002: Terraform provider publishing options (public and private)

Status: Accepted

Date: 2025-09-10

Decision drivers
- Ship a Terraform provider that mirrors the Pulumi provider’s functionality and configuration so teams on Terraform can adopt the same authorizer pattern.
- Choose a publishing model that balances discoverability, consumer friction, signing/security, and our ability to maintain releases.
- Keep the build/release pipeline simple and automatable from this repo.

## Summary
We need to distribute a Terraform provider that provisions the Verified Permissions policy store, deploys the bundled Lambda authorizer, optionally provisions Cognito, and ingests schema/policies with the same validations as the Pulumi provider. This ADR evaluates publishing options—public and private—and recommends a path.

## In-scope
- Publishing channels for the Terraform provider and their implications
- Ownership/namespace, access control, signing, and release processes
- Artifact format and pipeline requirements

Out of scope
- Implementation details of the provider (covered by the provider spec in `docs/vp-19-terraform-provider-spec.md`)

## Options

### A. Public: Terraform Registry (registry.terraform.io)

- What it is: The canonical public distribution for providers. Provider appears as `NAMESPACE/NAME` in the public registry.
- Ownership/namespace: The public Terraform Registry namespace will be `mikecbrant`. The provider source will be `mikecbrant/vpauthorizer`. A GitHub user/organization must be linked to the Terraform Registry publisher account.
- Access control: Public read access; releases are driven by signed GitHub releases in the repo.
- Discoverability: Highest (searchable on the public registry, shown in `terraform init` messages).
- Consumer friction: Lowest. Standard `required_providers` with `source = "namespace/name"` and `version` constraint. No extra CLI config.
- Security/signing: Registry requires a checksums file and a signature for each release. Typical pipeline uses GPG to sign the `SHA256SUMS` file (or uses the registry-supported signing method). Keys must be managed and rotated if needed. Terraform CLI verifies provider signatures on install. See HashiCorp docs for publishing and signature verification details. [Citations below]
- Versioning: SemVer tags (e.g., `v0.1.0`). Registry tracks versions and deprecations.
- Release cadence: Our CI publishes on every tagged release.
- Support/maintenance: Issues and docs live in this repo; registry shows README and docs pages.
- Docs requirements: A provider overview and resource/data source docs in the repo to render on the registry page.
- Build/release pipeline: Build per OS/arch, create zip per platform named `terraform-provider-<name>_vX.Y.Z_<os>_<arch>.zip`, generate `SHA256SUMS` and signature, and create a GitHub Release; the registry ingests from GitHub. We can use GoReleaser or equivalent.
- Organizational prerequisites: Terraform Registry publisher account linked to the chosen GitHub org/user; a long‑lived signing key (GPG) available to CI; repository name typically `terraform-provider-<name>`.

When to choose: We want community discoverability and the easiest consumption for external users.

### B. Private: HCP Terraform / Terraform Enterprise Private Registry

- What it is: Private provider distribution scoped to an HCP Terraform (Terraform Cloud) or Terraform Enterprise organization.
- Ownership/namespace: Provider lives in the org’s private registry namespace; visibility limited to org (and optionally specific projects/workspaces).
- Access control: Access managed by org membership and workspace permissions.
- Discoverability: Internal only; visible to the org in `terraform init` without extra CLI config.
- Consumer friction: Low within the org; no mirrors or filesystem overrides required.
- Security/signing: Same signature and checksum requirements as public publishing; releases are discovered from GitHub Releases or uploaded via API. The private registry verifies signatures.
- Versioning: SemVer tags, release approval flows possible inside HCP Terraform.
- Release cadence: CI can push new versions on tag; org can require manual promotion.
- Support/maintenance: Internal support expectations apply; docs are rendered similarly to public.
- Docs requirements: Same structure as public (overview + resource/data source docs) to render in the private registry UI.
- Build/release pipeline: Identical artifacts as public (zips + checksums + signature); registry is configured to pull from the repo.
- Organizational prerequisites: An HCP Terraform/Terraform Enterprise org with Private Registry enabled; a linked VCS connection; signing key available to CI; namespace/slug reserved in the org.

When to choose: We need to restrict consumption to our org and keep versions private while keeping a first‑class user experience.

### C. Private: Self‑hosted Provider Registry (protocol implementation)

- What it is: Operate a registry that implements the Terraform Provider Registry Protocol (e.g., served from S3/CloudFront or GitHub Pages).
- Ownership/namespace: Fully controlled host/domain; namespaces are up to us.
- Access control: We must implement authentication/authorization at the edge (if required) or keep the host private.
- Discoverability: None outside the audience we configure the CLI for.
- Consumer friction: Medium to high. Every consumer must add a `provider_installation` block to `~/.terraformrc` or repo `.terraformrc` to point at the custom registry (network mirror), or set environment variables. Bootstrap docs and support are required.
- Security/signing: We must publish checksums and signature files alongside artifacts and maintain the registry index/manifest endpoints. TLS and origin integrity are our responsibility.
- Versioning: Managed by our index; SemVer tags still recommended.
- Release cadence: CI must publish artifacts and update the registry index JSON.
- Docs requirements: No built‑in docs UI; must host docs separately.
- Build/release pipeline: Similar artifact build to public publishing, plus scripts to update the registry JSON endpoints for platform lists, versions, and checksums.
- Organizational prerequisites: A domain/host, storage (S3/Pages), and edge/security controls; internal docs for configuring `terraform` to use the mirror.

When to choose: Air‑gapped or highly controlled environments where public/HCP registries are prohibited.

### D. Private: Filesystem or Network Mirrors (no registry UI)

- What it is: Skip a registry—publish zipped binaries to an internal file share, S3 bucket, or artifact repo and point Terraform at it using the `provider_installation` configuration (`filesystem_mirror` or `network_mirror`).
- Ownership/namespace: Local folder structure defines coordinates. Usually scoped to a single org.
- Access control: Whatever the storage provides.
- Discoverability: None; requires bootstrap instructions.
- Consumer friction: Highest. Users must install a CLI config file and accept the lack of a UI; version browsing is rudimentary.
- Security/signing: Mirrors can include checksums/signatures and should be served over TLS for network mirrors. Terraform trusts the configured mirror.
- Versioning: Folder layout encodes versions; no registry metadata.
- Release cadence: Copy artifacts on release; optionally generate mirror indexes.
- Docs requirements: Separate documentation.
- Build/release pipeline: Build the same zips; push to the mirror location (optionally with checksum/signature files).
- Organizational prerequisites: Shared storage and a documented setup guide.

When to choose: Temporary internal distribution or bootstrap for air‑gapped teams.

## Comparison

- Discoverability: A (highest) > B > C/D
- Consumer friction: A/B (lowest) < C < D
- Security and signing: All require robust signing; A/B provide built‑in verification UX.
- Docs UX: A/B render provider docs; C/D require a separate docs site.
- Ongoing cost: A/B lowest (managed), C/D higher (operate and support).

## Recommendation
Choose Option A now: publish publicly to the Terraform Registry under a project‑controlled namespace.

Rationale
- Minimal friction for adopters and parity with how many providers are consumed today.
- We already maintain source in a public repo; publishing adds little incremental risk.
- The same signed artifacts can be used later for a private mirror if needed.

If organizational requirements dictate private distribution later, Option B (Private Registry in HCP Terraform) is the preferred private path due to low friction and built‑in docs/UX. Options C/D remain viable for air‑gapped deployments.

## Build & Release implications (for Options A/B)
- Repository layout: This Terraform provider will live in this monorepo under `providers/terraform`, alongside a Pulumi provider under `providers/pulumi`. Core provider logic will be factored into shared Go packages under `providers/internal` and reused by both providers; the `terraform` and `pulumi` subdirectories only map framework-specific inputs/outputs.
- Artifacts per release:
  - `terraform-provider-<name>_vX.Y.Z_<os>_<arch>.zip` for supported platforms (Linux/amd64, Linux/arm64, Darwin/amd64, Darwin/arm64, Windows/amd64 at minimum).
  - `SHA256SUMS` and a detached signature (e.g., `SHA256SUMS.sig`).
  - Provider manifest (generated by tooling) that declares supported protocol versions.
- CI steps:
  1) Release determination is driven by comment-based Changesets. The same Changeset drives both the Pulumi and Terraform providers to ensure coordinated releases.
  2) Publish Pulumi and Terraform artifacts in parallel jobs. For Pulumi, publish the provider plugin (e.g., via GitHub Release/S3) and language SDK packages (npm/PyPI/NuGet/Go) and optionally update the Pulumi Registry. For Terraform, publish to the public Terraform Registry. All jobs use reproducible build flags and signed artifacts.
  3) After both publishes succeed, run a single common workflow step that updates the repository with the final version information (e.g., changelogs, tags) and commits that to the default branch.
  4) For Terraform: archive per platform with correct naming, generate `SHA256SUMS` and signature, and attach to a GitHub Release for tag `vX.Y.Z`; the registry ingests from GitHub.
  5) For Pulumi: publish component provider/SDK packages matching the same `vX.Y.Z`.

Failure handling
- Treat both publishes as a unit. If either publish fails, do not run the common version‑update step; investigate, fix, and retry the failed job so both providers publish the same version.

### Versioning
- Providers must maintain strict 1:1 version parity. The Pulumi and Terraform providers are versioned and released together with identical SemVer (`X.Y.Z`) numbers, coordinated by the same Changeset and automated pipelines.

## Organizational/account prerequisites
- Select and control the namespace: GitHub org/user that will own the provider and the registry publisher.
- Create/secure a signing key for releases (stored as CI secret; establish rotation plan).
- Reserve provider name and initialize the repository subdirectory (`providers/terraform`) in this monorepo with Release workflow uploading from this repo.
- Internal documentation under `.charlie/` encodes the synchronized versioning and release workflow requirements (Changesets usage, parallel Pulumi/Terraform publish, and the common post‑publish version update step).

## Assumptions
- The provider will be implemented in Go using common logic to the terraform provider.
- We will target Linux (amd64/arm64), macOS (amd64/arm64), and Windows (amd64) initially.
- This repo remains the single source of truth for the Lambda code and the Pulumi provider; the Terraform provider will live alongside them in this monorepo.
- We intend to reuse the exact same Go logic across providers wherever possible to guarantee behavior parity.

## Open questions
1. Signing method and key management (GPG key identity, storage, and rotation).
2. Minimum platform matrix beyond the baseline (Windows/arm64? FreeBSD?).
3. Documentation home: rely on registry‑rendered docs only, or also add a local `docs/terraform/*` folder in this repo and link from README.

## References
- Publishing to the Terraform Registry (providers): https://developer.hashicorp.com/terraform/registry/providers/publishing
- Provider distribution, signing, and manifests: https://developer.hashicorp.com/terraform/registry/providers/publishing/signing
- Terraform Plugin Framework (protocol v6): https://github.com/hashicorp/terraform-plugin-framework

