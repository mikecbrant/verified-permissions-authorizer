# ADR 0002: Terraform provider publishing options (public and private)

Status: Proposed

Date: 2025-09-10

Decision drivers
- Ship a Terraform provider that mirrors the Pulumi provider’s functionality and configuration so teams on Terraform can adopt the same authorizer pattern.
- Choose a publishing model that balances discoverability, consumer friction, signing/security, and our ability to maintain releases.
- Keep the build/release pipeline simple and automatable from this repo.

## Summary
We need to distribute a Terraform provider that provisions the Verified Permissions policy store, deploys the bundled Lambda authorizer, optionally provisions Cognito (with SES email), and ingests schema/policies with the same validations as the Pulumi provider. This ADR evaluates publishing options—public and private—and recommends a path.

## In-scope
- Publishing channels for the Terraform provider and their implications
- Ownership/namespace, access control, signing, and release processes
- Artifact format and pipeline requirements

Out of scope
- Implementation details of the provider (covered by the provider spec in `docs/vp-19-terraform-provider-spec.md`)

## Options

### A. Public: Terraform Registry (registry.terraform.io)

- What it is: The canonical public distribution for providers. Provider appears as `NAMESPACE/NAME` in the public registry.
- Ownership/namespace: A GitHub organization or user namespace must be linked to a Terraform Registry publisher account; provider repository typically named `terraform-provider-<name>`. We must select the namespace (e.g., `mikecbrant` or an org such as `charlielabs`).
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
- Repository layout: `terraform-provider-<name>` (or this repo with a `providers/terraform` subdir; see open questions).
- Artifacts per release:
  - `terraform-provider-<name>_vX.Y.Z_<os>_<arch>.zip` for supported platforms (Linux/amd64, Linux/arm64, Darwin/amd64, Darwin/arm64, Windows/amd64 at minimum).
  - `SHA256SUMS` and a detached signature (e.g., `SHA256SUMS.sig`).
  - Provider manifest (generated by tooling) that declares supported protocol versions.
- CI steps:
  1) Build with Go `-trimpath` and reproducible flags.
  2) Archive per platform with correct naming.
  3) Generate checksums and sign them with the registry‑recognized method.
  4) Create a GitHub Release for tag `vX.Y.Z` including the zips, checksums, and signature.
  5) Registry picks up the release and serves it to `terraform init`.

## Organizational/account prerequisites
- Select and control the namespace: GitHub org/user that will own the provider and the registry publisher.
- Create/secure a signing key for releases (stored as CI secret; establish rotation plan).
- Reserve provider name and initialize the repository (either a new repo named `terraform-provider-<name>` or a subdirectory in this monorepo with Release workflow uploading from this repo).

## Assumptions
- The provider will be implemented in Go using the official Terraform Plugin Framework (protocol v6) unless we decide otherwise during spec review.
- We will target Linux (amd64/arm64), macOS (amd64/arm64), and Windows (amd64) initially.
- This repo remains the single source of truth for the Lambda code and the Pulumi provider; the Terraform provider can live alongside them unless we decide to split repos.

## Open questions
1. Namespace/ownership for the Terraform Registry: `mikecbrant` vs an org (e.g., `charlielabs`).
2. Provider name: short (`vpauthorizer`) vs descriptive (`verified-permissions-authorizer`).
3. Repository topology: live in this monorepo (easiest code sharing) vs a dedicated `terraform-provider-<name>` repo (more conventional for registry discoverability). If we choose a subdir, we must confirm the registry can ingest from a monorepo path via our Release artifacts (it can, since ingestion is release‑artifact based).
4. Signing method and key management (GPG key identity, storage, and rotation).
5. Minimum platform matrix beyond the baseline (Windows/arm64? FreeBSD?).
6. Documentation home: rely on registry‑rendered docs only, or also add a local `docs/terraform/*` folder in this repo and link from README.

## References
- Publishing to the Terraform Registry (provider): HashiCorp documentation. 
- Provider distribution format, signing, and manifests. 
- Provider SDK/Framework guidance and protocol versioning. 

(Citations: see the issue comment linking back to this ADR.)

