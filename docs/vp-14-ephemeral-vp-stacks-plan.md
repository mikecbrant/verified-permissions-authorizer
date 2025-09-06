# VP-14: Pulumi-based deployment plan for ephemeral Amazon Verified Permissions stacks

Status: Proposed

Created: 2025-09-06

Owners: Platform/Infra (primary), AppSec (review), https://linear.app/mikebrant/issue/VP-14

## 0) One-paragraph summary

We need a simple, reproducible way to spin up and tear down short‑lived (ephemeral) stacks of Amazon Verified Permissions (AVP) resources—policy store, Lambda authorizer, Cognito identity source (included by default for ephemerals; optional for long‑lived envs via provider input), and the provider‑managed DynamoDB table—keyed by a supplied stage name and deployed into a target AWS account/region. SES identities/ARNs used by Cognito are always supplied by the consumer; our stacks do not provision SES. After evaluating Pulumi and SST, the recommendation is: keep this in Pulumi. Use Pulumi Cloud as the state backend and orchestrate ephemeral environments with Pulumi Deployments Review Stacks (PR‑scoped) backed by AWS OIDC. For cost control and flexibility, pair this with GitHub Actions as the trigger and fall back to GH‑hosted `pulumi up` for heavy‑usage repos to avoid exceeding Pulumi Deployments free minutes. Publish the reusable Pulumi Component Provider we already maintain to the public Pulumi Registry and npm, versioned via Changesets.

---

## 1) Requirements and constraints inventory

### Functional goal

- Create/update/destroy ephemeral stacks of AVP resources for a given `stage` within a specified AWS account/region.
- Stage names are provided (e.g., `pr-123`, `mike`, `demo-aug15`); stacks must be isolated by stage and account.
  - Cognito is included for ephemerals. When Cognito email is enabled, SES identities/ARNs are provided by the consumer and referenced by the stack (not created by it). The SES identity must be in the same AWS account and Region as the Cognito User Pool; cross‑account sending requires SES authorization and is out of scope for the default flow.

### Non-functional requirements

- Security & isolation: Least-privilege AWS roles, short‑lived credentials (OIDC), and no long‑lived cloud keys in CI.
- Reproducibility & idempotency: Re-running with the same config is safe; drift detection where available.
- Lifecycle: Create on PR open or manual request; update on push; destroy on PR close/merge or manual expiry.

- Naming conventions:
  - Pulumi stack names: keep `<account-alias>-<region>-<stage>` (examples below). For Review Stacks (PR ephemerals) created by Pulumi Deployments, use the default `pr-<number>` naming and carry `account-alias` and `region` via stack config and tags. This guidance is scoped to Pulumi stack names only.
    - `account-alias` must match `^[a-z0-9-]{1,20}$` (lowercase letters, digits, hyphens) and be organization‑approved.
    - `region` must be a valid AWS Region id (e.g., `us-east-1`).
    - `stage` must match `^[a-z0-9-]{1,32}$`.
    - Example: `sandpit-us-east-1-pr-123`.
  - AWS resource names: do NOT include account alias or region in resource names. Include `stage` only where it improves clarity and the service allows it. Put account alias and region in tags instead. Exception: resources that require globally unique names (e.g., S3 buckets) may include `stage` and, if necessary, account/region suffixes to ensure uniqueness.
  - Service‑specific uniqueness: Cognito Hosted UI domain prefixes are Region‑scoped and must be unique; including `stage` is recommended.
- Concurrency limits: Cap active ephemeral stacks per account (initially 10–25), serialized updates per stack.
- Developer UX: One PR creates one isolated environment; outputs are surfaced back to the PR.
- Cost awareness: Defaults minimize spend (no provisioned concurrency, on‑demand tables, short log retention).

### AWS account model and IAM

- Target model: Multi‑account recommended (e.g., `sandpit`, `dev`, `stage`, `prod`). Ephemeral stacks deploy only to non‑prod accounts unless explicitly allowed.
- Cross‑account deployment: Use AWS IAM roles per account that trust Pulumi Cloud OIDC for Deployments and/or GitHub OIDC for Actions. Trust policies restrict by repo, org, project, and (optionally) stack pattern.
- IAM permission scope for deployment roles: Use a curated least‑privilege policy covering AVP, Lambda, IAM (for the authorizer role), and Cognito, with appropriate conditions. Avoid using `AdministratorAccess`. If onboarding friction is high, a time‑boxed `AdministratorAccess` role may be used only as a temporary bootstrap before replacing it with the curated policy. SES identities are not created by our stacks; consumers provide SES identity ARNs when enabling email from Cognito.

### State, secrets, and sensitive config

- State backend: Pulumi Cloud (preferred) for state, history, RBAC, and Deployments. Self‑managed S3/Dynamo only if Pulumi Cloud is disallowed.
- Secrets: Decision — use Pulumi Cloud’s built‑in secrets provider as the baseline for CI/CD. Generate ephemeral secrets per stack where possible (e.g., `JWT_SECRET` via the Pulumi Random provider). Optionally adopt Pulumi ESC for centralized secrets and dynamic cloud creds; or use AWS KMS secrets‑provider if org policy requires customer‑managed keys.
- Operational secrets (examples): `JWT_SECRET` for Lambda authorizer and any test user credentials created during automation. SES `identityArn` and `fromEmail` are configuration values (not secrets) provided by the consumer. Mark sensitive outputs as Pulumi secret outputs so they are redacted in PR comments and logs.

### Assumptions and gaps to confirm

- Regions: Use AWS Regions that support both Amazon Verified Permissions and Amazon Cognito. This repo defaults to `us-east-1` for its own internal deployments. When Cognito email is enabled, ensure the provided SES identity ARN is in the same Region as the Cognito User Pool (SES identities are Region‑scoped for sending).
- Whether Pulumi Cloud SaaS is approved for state and OIDC in this org.
- Target Pulumi language/runtime: TypeScript (finalized). The actual implementation will land in a follow‑up PR.
- Desired TTL for ephemeral stacks (proposal: 48h default) and max concurrent ephemerals per account.
- When Cognito email is enabled for ephemerals, confirm SES identity ARNs per Region (provided by the consumer).

---

## 2) Options analysis (Pulumi and SST)

| Option | Authoring & fit | Ephemeral support | Maturity/maintenance | Pros | Cons |
|---|---|---|---|---|---|
| A) Pulumi only | First‑class fit: we already ship a Pulumi Component Provider in this repo; tiny Pulumi program composes it | Pulumi Deployments Review Stacks auto‑create/destroy per PR; can also script with CLI | Pulumi Cloud is mature; IaC model matches current code; single toolchain | One ecosystem; strong state/RBAC/audit; OIDC to AWS; Review Stacks; easy cross‑account | Pulumi Deployments minutes are metered (500 free on Individual; 3,000 on Team); TTL stacks are Enterprise‑only |
| B) SST | Good DX for app stacks; uses CDK/CloudFormation; stages map well to ephemerals | Built‑in `--stage` flows; Console & GitHub Actions patterns for PR envs | Solid for serverless apps; adds a second IaC/system | Nice dev ergonomics (`sst dev`), preview env patterns | Duplicates IaC stack alongside our Pulumi provider; two runtimes (CDK + Pulumi); extra SaaS (SST/SEED) and pricing|
| C) Hybrid (Pulumi for AVP provider; SST for app) | Possible but splits infra: AVP in Pulumi, app infra in SST | Both can make PR envs | Highest complexity | Can keep app devs on SST while platform manages AVP via Pulumi | Cross‑tool coordination; two state backends; two OIDC roles; cognitive load |

Conclusions:

- Given we already own a Pulumi Component Provider that creates AVP + authorizer + optional Cognito and DynamoDB, Pulumi‑only is the simplest and lowest‑maintenance path.
- Use Pulumi Cloud as the state backend. Review Stacks provide first‑class ephemerals. If Deployments minutes become a constraint, run `pulumi up` from GitHub Actions (public repos: minutes are free) and keep Pulumi Cloud solely for state and RBAC.

Pulumi state backend: Pulumi Cloud is recommended. It includes state, RBAC, history, REST API, and Deployments with included free minutes; TTL stacks and drift detection are Enterprise features. Self‑managed backends (S3+Dynamo) are an alternative but lose Review Stacks and SaaS conveniences.

Secrets management:

- Baseline: Pulumi Cloud secrets for stack config secrets.
- Optional: Pulumi ESC if we want a central secrets hub and OIDC‑fetched short‑lived creds; can also mirror from AWS Secrets Manager or Vault. If ESC is out of scope, we can use a KMS‑backed secrets provider.

References: Pulumi pricing and Deployments minutes/TTL stacks; Pulumi Review Stacks; ESC overview. [citations inline at the end]

---

## 3) Deployment orchestration decision

We evaluated three orchestration models:

- Pulumi Cloud/Console orchestrated (Deployments + Review Stacks)
- GitHub Actions orchestrated (`pulumi up` via OIDC)
- Hybrid

Recommendation: Hybrid with Pulumi‑first.

- Internal deployments from this repo will use Pulumi Deployments with OIDC to AWS (Pulumi Cloud OIDC). GitHub Actions remains a fallback/trigger and an option for repositories where Deployments minutes are constrained.

- Ephemerals (PRs): Use Pulumi Review Stacks for automatic create/update/destroy. This gives PR comments, outputs, and one‑click access in the Console. Map each PR to a Pulumi stack named `pr-<number>` (Review Stacks default) in a dedicated project (e.g., `vp-ephemeral`); carry `account-alias` and `region` via stack config and tags. To avoid name collisions when targeting multiple accounts/regions concurrently, standardize on one Review‑Stack project per target AWS account (optionally per Region).
- Stable envs (single‑main promotion): Use Pulumi Deployments on a single `main` branch with environment‑gated promotions/approvals across `dev → stage → prod` stacks. Avoid long‑lived `dev/stage/prod` branches.
- If Deployments minutes are a concern at scale, switch ephemerals to GH Actions while retaining Pulumi Cloud for state. Actions minutes are free for public repos; for private repos rely on plan quotas.

Stage → Pulumi stack → AWS account mapping:

- Project: `vp-authorizer` (or `vp-ephemeral` dedicated for PRs)
- Stack naming: `<account-alias>-<region>-<stage>` for Actions‑only ephemerals and stable envs (examples: `sandpit-us-east-1-demo`, `dev-us-east-1-mike`). Review Stacks use the default `pr-<number>` name and carry account/region via config/tags.
- Account selection mechanics:
  - Pulumi Deployments: Prefer project‑level environments that provide the AWS Role ARN (one per target account/project). Avoid per‑stack settings for Review Stacks to keep ephemerals low‑touch.
  - GitHub Actions: prefer a single input/env `AWS_ROLE_ARN` to select the role to assume via OIDC; set `AWS_REGION` explicitly and derive the account id from the role ARN when needed.

    Example (excerpt):

    ```yaml
    jobs:
      up:
        permissions:
          id-token: write
          contents: read
        env:
          ACCOUNT_ALIAS: sandpit
          AWS_ROLE_ARN: arn:aws:iam::123456789012:role/pulumi-deployer
          AWS_REGION: us-east-1
          PULUMI_ACCESS_TOKEN: ${{ secrets.PULUMI_ACCESS_TOKEN }}
          STACK_NAME: ${{ env.ACCOUNT_ALIAS }}-${{ env.AWS_REGION }}-pr-${{ github.event.pull_request.number }}
        steps:
          - uses: actions/checkout@v4
          - uses: aws-actions/configure-aws-credentials@v4
            with:
              role-to-assume: ${{ env.AWS_ROLE_ARN }}
              aws-region: ${{ env.AWS_REGION }}
          - uses: pulumi/actions@v6
            with:
              command: up
              stack-name: ${{ env.STACK_NAME }}
    ```
    (Use stable tags for readability; in production, pin these actions to specific commit SHAs per your organization’s security policy, and refresh them periodically. Store `PULUMI_ACCESS_TOKEN` as a GitHub repository secret.)

    Tip: serialize applies per logical stack with a job‑level concurrency block keyed by `STACK_NAME`:

    ```yaml
    jobs:
      up:
        concurrency:
          group: ${{ github.workflow }}-${{ env.AWS_ROLE_ARN }}-${{ env.AWS_REGION }}-${{ env.STACK_NAME }}
          cancel-in-progress: true
    ```

Lifecycle triggers:

- Create/update on PR open/push; destroy on PR close/merge (Review Stacks do this automatically). For Actions‑only, pair PR events with `pulumi up`/`pulumi destroy` jobs.

---

## 4) Cost implications (order‑of‑magnitude)

Pulumi Cloud / Deployments

- Individual plan is free and includes 500 Deployments minutes/month. Team includes 3,000 minutes; all extra minutes are $0.01/min. TTL stacks and drift detection are Enterprise features. If we exceed minutes, we can move ephemerals to GH Actions and keep Pulumi Cloud as the backend. [Pulumi Pricing, Deployments minutes, TTL stacks]

GitHub Actions

- Public repos: standard GitHub‑hosted runners are free. Private repos: 2,000–50,000 included minutes depending on plan; Linux rate is $0.008/min beyond included. We can also use self‑hosted runners. [GitHub Actions billing]

SST / SEED Console (only if we went that route)

- SST Console pricing (Jan–Feb 2025): free up to ~350 resources, then per‑resource rates; SEED CI/CD plans start at $30/month with 4,500 build minutes (Team). This would be additional to AWS service costs. [SST Console pricing update; SEED pricing]

AWS services for an ephemeral AVP stack

- Amazon Verified Permissions: Single authorization requests now cost $5 per 1M ($0.000005) since 2025‑06‑12; batch auth and policy management priced separately. Idle stacks that aren’t making auth calls incur essentially no AVP charges. [AVP pricing]
- Amazon Cognito (Essentials tier): First 10,000 MAUs per account/month are free; above that, Essentials is $0.015/MAU (SAML/OIDC federated users: 50 MAU free, then $0.015/MAU). For most ephemerals this is $0. SMS/SES sending billed separately. [Cognito pricing/FAQ]
- DynamoDB (on‑demand): After the 2024‑11 price cut, typical US‑East-1 rates are about $0.125 per 1M read request units and $0.625 per 1M write request units; first 25 GB storage free per Region (free tier). For low‑traffic ephemerals, monthly cost is near $0. [DynamoDB pricing]
- Lambda: 1M requests and 400,000 GB‑seconds free per month; with minimal invocations, cost is usually $0. Avoid Provisioned Concurrency for ephemerals. [Lambda pricing]
- CloudWatch logs: charged per GB stored; negligible for light usage, but set short retention for ephemerals (7 days).

Cost control levers (non‑prescriptive):

- TTL (automatic destroy) on PR close; optional time‑bomb labels for manual stacks.
- Down‑sized defaults for ephemerals (no provisioned concurrency; reserved concurrency 1; DynamoDB streams off unless testing it; short log retention).
- Cap number of concurrent PR stacks per account; rate‑limit updates per stack.

---

## 5) CI/CD and continuous deployment design

Flows

- Per‑PR ephemeral: Review Stacks create `pr-<n>` stacks; outputs (e.g., policy store ID, authorizer ARN, user pool IDs) are posted to the PR. Destroy on close/merge.
- Dev/Stage/Prod: Protected branches trigger Deployments (Pulumi) or Actions jobs that require approvals. Promotion is merge‑based, not in‑place mutation.

Repo‑level automation artifacts (high‑level)

- Pulumi project under `infra/` (or `infra/vp-authorizer/`) that consumes the existing component provider from `packages/provider` and exposes minimal configuration.
- Pulumi stack configs: `Pulumi.pr.yaml` template for Review Stacks; `Pulumi.dev.yaml`, etc., for stable envs.
  - Example `Pulumi.pr.yaml` (minimal):

    ```yaml
    config:
      vp-authorizer:accountAlias: sandpit
      aws:region: us-east-1
      vp-authorizer:stage: "pr-<number>"
      vp-authorizer:cognito:
        enabled: true
        sesConfig:
          fromEmail: no-reply@example.com
          identityArn: arn:aws:ses:us-east-1:123456789012:identity/example.com
    ```
    Set `jwtSecret` securely rather than committing it to source. The `stage` placeholder should be set dynamically by the orchestrator (Pulumi Deployments or GH Actions):

    - Recommended for ephemerals: generate via the Pulumi Random provider in code.
    - Or via CLI at deploy: `pulumi config set vp-authorizer:jwtSecret --secret "$(openssl rand -base64 32)"`.
- Pulumi Cloud Deployments settings per stack (stable stacks only) to bind the AWS Role ARN; Review Stacks should inherit credentials from project‑level environments. If Actions‑only, define GH workflows for `pulumi up` and `pulumi destroy` with OIDC and pass `aws_role_arn`/`aws_account_id`/`aws_region` as inputs/env.
- IAM roles per target AWS account for Pulumi OIDC and GH OIDC; trust policies constrained by org/repo/project/stack patterns.

Naming, tagging, and metadata

- Stack name (Pulumi): `<account-alias>-<region>-<stage>` (DNS‑safe; lowercase, digits, hyphens). Stage must match `^[a-z0-9-]{1,32}$`.
- AWS resource naming: omit account alias and region from resource names; prefer including only `stage` where helpful and supported. Keep account alias and region in tags. Exception: globally‑unique names (e.g., S3 buckets) may include suffixes for uniqueness.
- Standard AWS tags on all resources: `Project=vp-authorizer`, `PulumiProject=<project>`, `PulumiStack=<stack>`, `Stage=<stage>`, `Account=<alias>`, `Repo=<org/repo>` (e.g., `${{ github.repository }}` in Actions), `Owner=<github-actor or team>`, `TTL=<ISO8601 or hours>`.
- Observability/audit: Rely on Pulumi Cloud activity log for IaC operations; optional drift detection (Enterprise) or scheduled previews.

Security notes

- No long‑lived AWS keys in GitHub; use OIDC everywhere (Pulumi Cloud OIDC for Deployments; `token.actions.githubusercontent.com` for Actions). Restrict trust with the `sub` claim to org/repo/branch/stack patterns. Prefer per‑account roles.

---

## 6) Pulumi Registry release strategy (public and/or private)

- We already maintain a Pulumi Component Provider in `packages/provider` with a Node SDK in `packages/sdk/nodejs`. The intended consumers are internal stacks and external users; publish to the public Pulumi Registry and npm under the current names.
- Versioning & publishing: Continue using Changesets to produce semver releases. On release, publish:
   - Provider plugin binaries to GitHub Releases.
   - Node SDK to npm.
   - Schema to Pulumi Registry (public). If a private registry is later required, mirror via a private Pulumi Cloud org registry.
- CI hooks (high‑level): On tag or merged changeset release PR, run provider build, generate SDKs, run tests, sign artifacts, publish. Add a lightweight canary stack that exercises the latest provider.

---

## 7) Recommendation (short form)

- Use Pulumi Cloud for state and OIDC; implement PR ephemerals with Pulumi Review Stacks in a dedicated project. Keep `retainOnDelete=false` for ephemerals to ensure full cleanup.
- Keep GitHub Actions in the loop for code checks and, if needed, to run `pulumi up` when Deployments minutes are constrained; Actions are free on public repos.
- Stick with public Pulumi Registry + npm for the provider; no private registry needed at this time.

Explicit answers:

- “Pulumi Console and/or GitHub Actions?” → Pulumi Console (Deployments + Review Stacks) as primary, with GitHub Actions as the trigger and cost‑control fallback (CLI‑driven applies/destroys when minutes are tight).
- “DevOps recommendations for continuous deployment and Registry release?” → Single‑main promotions with environment‑gated approvals (Pulumi Deployments) across `dev → stage → prod`; Review Stacks for PRs; continue Changesets‑based releases to public Pulumi Registry and npm; maintain OIDC to AWS in both Pulumi and GitHub.

---

## 8) Open questions for stakeholders

1. Which AWS accounts are in scope for ephemerals? Please provide account IDs → alias map.
2. What is the default TTL for PR environments (24h, 48h, 72h)? What’s the hard cap on concurrently active PR stacks per account?
3. Is Pulumi Cloud SaaS approved for state and OIDC in these accounts? If not, what’s the approved alternative (S3+Dynamo backend, Actions‑only orchestration)?
4. Are there compliance constraints requiring AWS KMS as the Pulumi secrets provider (instead of Pulumi‑managed secrets)?
5. Any budget or plan constraints that would preclude Pulumi Team/Enterprise features (e.g., TTL stacks, drift detection)?

---

## 9) High‑level implementation plan (post‑approval)

1) Infra program
- Add `infra/vp-authorizer/` Pulumi project that consumes `verified-permissions-authorizer:index:AuthorizerWithPolicyStore` with minimal inputs and sensible ephemeral defaults: no provisioned concurrency, `reservedConcurrency=1`, DynamoDB Streams disabled by default, `retainOnDelete=false`.

2) Pulumi Cloud wiring
- Create Pulumi org/project; define stacks: `pr` template stack with `Pulumi.pr.yaml`, and stable stacks (`dev`, `stage`, `prod`).
- Configure Deployments settings for `pr` (Review Stack template) and for stable stacks (push‑to‑deploy).

3) AWS IAM & OIDC
- In each target account: add Pulumi Cloud OIDC IdP (`https://api.pulumi.com/oidc`) and a deploy role per project with trust restricted by `sub` to org/project/stack pattern; add a separate role for GH OIDC (`https://token.actions.githubusercontent.com`) for CLI‑driven applies/destroys. Document role ARNs in stack config.

4) GitHub automation
- Add workflows: (a) unit/lint/tests; (b) optional `infra-ephemeral.yml` that calls Pulumi CLI for ephemerals when Deployments minutes are exhausted; (c) `infra-destroy-stale.yml` to clean up abandoned stacks as a safety net.

5) Guardrails
- Add default AWS tags; set CloudWatch log retention (7d) for ephemeral stacks; define concurrency groups per `stage` to avoid overlapping updates; optionally add budgets/alerts.

6) Docs
- Add a short “How to test with an ephemeral stack” guide to the repo, including how to find PR outputs and how to trigger manual teardown.

---

## 10) Citations (pricing/features)

- Pulumi Pricing and Deployments minutes; TTL stacks listed under Enterprise, free 500 minutes on Individual, 3,000 on Team/Enterprise: https://www.pulumi.com/pricing/; https://www.pulumi.com/product/pulumi-deployments/.
- Pulumi Review Stacks overview & configuration: https://www.pulumi.com/docs/pulumi-cloud/deployments/review-stacks/.
- Pulumi OIDC to AWS (Deployments/ESC): https://www.pulumi.com/docs/pulumi-cloud/deployments/oidc/aws/; https://www.pulumi.com/docs/pulumi-cloud/access-management/oidc/provider/aws/.
- Pulumi ESC overview: https://www.pulumi.com/docs/esc/.
- GitHub Actions billing: public repos free minutes; private repos quotas and $/min: https://docs.github.com/en/billing/managing-billing-for-github-actions/about-billing-for-github-actions.
- GitHub Actions OIDC to AWS: https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/configuring-openid-connect-in-amazon-web-services.
- Amazon Verified Permissions pricing incl. 2025‑06‑12 reduction for single auth requests to $0.000005: https://aws.amazon.com/verified-permissions/pricing/; https://aws.amazon.com/about-aws/whats-new/2025/06/amazon-verified-permissions-reduces-price/.
- Amazon Cognito Essentials & free tier (10,000 MAU free; Plus tier and SAML/OIDC notes): https://aws.amazon.com/cognito/faqs/; https://aws.amazon.com/cognito/pricing/.
- DynamoDB on‑demand pricing and 2024 reduction; example unit prices: https://aws.amazon.com/dynamodb/pricing/on-demand/; https://aws.amazon.com/blogs/database/new-amazon-dynamodb-lowers-pricing-for-on-demand-throughput-and-global-tables/.
- AWS Lambda pricing and free tier: https://aws.amazon.com/lambda/pricing/.
