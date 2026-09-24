# Jev-Powered Governance and Autonomy Layer for Terraform Changes

A calibrated risk-scoring layer that lets low-risk Terraform changes auto-apply, routes uncertain changes to a plain-English question, and escalates genuine risk to experts.

## Overview

Terraform turns infrastructure code into a reviewed plan and applies the changes needed to make real infrastructure match it. The review step — a human deciding whether to trust a plan — is where the product currently breaks down: as change volume grows, review funnels through a small number of experts who read plans by hand, and that bottleneck is about to get worse as AI coding agents start opening infrastructure PRs with no governance story attached.

This feature is a governance and autonomy layer that sits on top of existing Terraform workflows (whether run through HCP Terraform, Terraform Enterprise, Spacelift, env0, or Scalr). An LLM generates and explains each change, and Jev — a calibrated decision model — scores its risk. Low-risk changes auto-apply, high-risk or uncertain changes route to an expert, and a middle tier of medium-risk changes gets converted into a plain-English question the original requester can answer directly (e.g., "this will drop and recreate the database; is downtime acceptable?"). That middle tier is the long-term differentiator over both deterministic policy (which can't reason about intent) and blanket manual approval (which can't triage) — but it is not where the product enters the market.

The entry point is a wedge: bulk Terraform and provider version upgrades across hundreds of workspaces, a low-blast-radius change class with clear, measurable engineer-time ROI. Risk-scoring runs in shadow mode alongside the wedge, quietly accumulating the labeled outcomes needed to prove Jev's calibration on infrastructure data before the product earns any auto-apply authority.

Scope for this PRD covers the full four-stage ladder described below, from shadow-mode scoring through governing agent-authored infrastructure changes. It does not cover replacing existing deterministic policy tooling (Sentinel, OPA, Checkov) or building a proprietary calibrated verifier as a Jev alternative — both are called out explicitly in Out of Scope.

## Background

Terraform is a declarative tool: infrastructure is described as code, Terraform computes a plan showing what it will create, change, or destroy, and a human reviews that plan before apply. That review step is the trust checkpoint in the workflow today.

Teams currently handle review-at-scale in one of two ways. Deterministic policy engines (Sentinel, OPA, Checkov) encode rules and catch whatever fits a written rule, but can't reason about intent or context outside that rule set. The alternative, blanket manual approval, doesn't scale — it either produces rubber-stamped plans nobody fully read, or a queue behind a small platform team that becomes the bottleneck as change volume rises.

This is compounding for a new reason: AI coding agents are starting to open infrastructure pull requests directly, with no governance layer built for agent-originated change. Existing tooling (HashiCorp's own stack, Spacelift, env0, Scalr) owns the plan/apply workflow surface today but is not focused on this agent-PR governance problem.

The product bet is Jev: a verifier that returns a calibrated probability for a semantic judgment, rather than a raw model score. Frontier LLM calls were already becoming cheap enough to run per-change; what's new is a score reliable enough to threshold on for auto-apply decisions. That calibration has not yet been demonstrated specifically on Terraform plans and their real-world outcomes — which is why the product's first phase is shadow mode, not autonomy.

## Problem

**When** change volume across hundreds of Terraform workspaces outpaces the platform team's review capacity,
**I want** low-risk changes to be triaged and cleared automatically,
**so that** experts only spend review time on changes that actually carry risk.

**When** an AI coding agent opens an infrastructure pull request,
**I want** that change governed with the same rigor as a human-authored one,
**so that** agent-driven infrastructure changes can't bypass review controls simply because no human wrote the HCL.

**When** a Terraform plan is ambiguous — not clearly safe, not clearly dangerous (e.g., a resource replacement) —
**I want** the system to convert that ambiguity into a specific, plain-English question the requester can answer,
**so that** an expert doesn't have to get pulled in for a decision the requester is actually positioned to make.

**When** a change doesn't map cleanly to any written policy rule,
**I want** a system that can reason about the intent and context of the change,
**so that** I'm not stuck choosing between blocking a legitimate change or approving one I don't fully understand.

**When** my team wants to run bulk Terraform and provider version upgrades across hundreds of workspaces,
**I want** the low-risk instances of that change to apply without manual review,
**so that** we capture measurable engineer-time savings without waiting to trust the system on novel, higher-stakes changes.

**When** we're introducing a new automated risk-scoring system into a change-management workflow,
**I want** it to run in shadow mode and prove its calibration before it gains any gating authority,
**so that** we don't hand over review-blocking power based on an unproven scoring model.

## Assumptions

- Jev (or an equivalent calibrated verifier) can produce well-calibrated risk probabilities on Terraform plan data specifically — this is unproven today, which is why shadow mode is a required phase, not an optional one. See [[jev-calibration]].
- The compliance-conscious-but-not-mandated segment (fintech, SaaS, healthtech) will extend trust to probabilistic auto-apply once a track record is visible, even though today they rely on deterministic policy or manual approval.
- Cross-customer data pooling is not assumed as the moat. Plans contain sensitive infrastructure detail, and many customers will not permit their data to train a shared model. The default assumption is per-tenant learning, with cross-customer pooling as upside contingent on opt-in or on training only on derived risk features rather than raw plans.
- Incumbent workflow owners (HashiCorp, Spacelift, env0, Scalr) could bolt a similar risk layer onto their own products; the wedge and the agent-PR governance gap are assumed to provide enough of a head start to matter.
- Ground truth on bad outcomes is sparse and lagging. Shadow mode and manual-override capture are assumed to be sufficient to manufacture enough labeled data to validate calibration in a reasonable timeframe; this has not yet been tested at volume.

## Personas

**Head or Director of Platform Engineering**
The buyer. Runs platform engineering for a company with roughly 1,000–10,000 employees operating hundreds of Terraform workspaces across multiple cloud accounts. Owns the tradeoff between review rigor and review-team headcount, and is increasingly exposed by AI coding agents opening infrastructure PRs with no governance answer.
- Key needs:
  - A governance story for agent-authored infrastructure changes, not just human-authored ones
  - Measurable reduction in engineer-time spent on repetitive, low-risk review
  - An auditable track record to justify extending autonomy further, including to compliance stakeholders
  - Confidence that autonomy is earned incrementally, not granted all at once

**Platform Engineer / SRE**
The daily user. Currently reviews and approves Terraform plans by hand as part of the job. Cares about not missing a genuinely risky change, and about not drowning in low-value review of repetitive plans (e.g., version bumps across hundreds of workspaces).
- Key needs:
  - Confidence that auto-applied changes were genuinely low-risk, not just unreviewed
  - A clear, fast escalation path when a change is flagged as uncertain
  - Visibility into why a change was scored the way it was

**Application Developer (requester)**
Files tickets for infrastructure they need (a database, an environment, a resource change) but doesn't write or read HCL fluently. Currently waits behind the platform team's review queue regardless of how simple their request is.
- Key needs:
  - Faster turnaround on requests that don't actually need platform-team review
  - A plain-English question they can answer directly when their change is ambiguous, without needing to understand Terraform plan output
  - Clarity on what a change will actually do before it happens

## Phases & Requirements Table

| Phase | Requirement | Description |
|---|---|---|
| Phase 1 | Bulk upgrade wedge | Execute bulk Terraform and provider version upgrades across many workspaces as the entry-point use case |
| Phase 1 | Shadow-mode risk scoring | Score every plan's risk via Jev without gating power, to accumulate labeled outcomes |
| Phase 1 | Change explanation generation | Generate a plain-English explanation of what each plan will do |
| Phase 1 | Verifier availability behavior | Define and surface what happens to governance decisions when the calibrated verifier is unavailable |
| Phase 2 | Auto-apply in dev | Let low-risk changes auto-apply in non-production environments |
| Phase 2 | Auto-apply audit trail | Record and surface a reviewable history of every auto-applied change and its score |
| Phase 3 | Earned production autonomy | Extend auto-apply to production based on demonstrated shadow-mode and dev track record |
| Phase 3 | Medium-risk question tier | Convert medium-risk plans into a plain-English question the requester answers, instead of routing to an expert |
| Phase 4 | Agent-authored change governance | Apply the same scoring, auto-apply, and escalation logic to infrastructure changes opened by AI coding agents |

## Phase 1: Shadow Mode and the Bulk Upgrade Wedge

### Bulk upgrade wedge

**Narrative:** The product's entry point is bulk Terraform and provider version upgrades run across hundreds of workspaces. This class of change is common, repetitive, and low-blast-radius, which makes it a clean, measurable win in engineer-weeks saved without requiring anyone to trust auto-apply on novel or high-stakes changes yet.

**Acceptance Criteria:**
- Users must be able to initiate a bulk version upgrade targeting a defined set of workspaces.
- The system must generate a plan and a plain-English explanation for each affected workspace individually.
- Users must be able to see, before any change is applied, how many workspaces are affected and a summary of what will change in each.
- Users must be able to review results (applied, failed, skipped) per workspace after the upgrade run completes.

**Considerations:**
- How much variation in provider/module version state across workspaces can the wedge handle before a workspace needs to be routed to manual handling instead of the bulk flow?

### Shadow-mode risk scoring

**Narrative:** Every Terraform plan is scored for risk by Jev, but the score does not yet gate or block anything — it runs alongside the existing review process so its output can be compared against actual outcomes. This is how the product proves Jev's calibration holds on Terraform plan data specifically, which has not been demonstrated before.

**Acceptance Criteria:**
- Every plan processed by the system must receive a risk score, regardless of environment or change type.
- The risk score must not affect whether a change requires manual approval during this phase.
- Users must be able to view the risk score and its stated confidence alongside the plan it corresponds to.
- The system must record the actual outcome of each change (applied cleanly, rolled back, caused an incident, manually overridden) so it can be compared against the predicted score.

**Considerations:**
- What counts as ground truth for a "bad outcome" when incidents are rare, lagging, and not always attributed cleanly to a specific change? See [[jev-calibration]].
- How long does shadow mode need to run, and how many labeled outcomes are needed, before there's enough evidence to consider granting any gating authority?

### Change explanation generation

**Narrative:** Alongside the risk score, an LLM generates a plain-English explanation of what a plan will actually do. This explanation is what a non-expert requester or a time-constrained reviewer reads instead of raw Terraform plan output, and it's the raw material the later medium-risk question tier is built on.

**Acceptance Criteria:**
- Every scored plan must have an accompanying explanation written in plain language, not raw plan diff syntax.
- The explanation must call out destructive or disruptive actions specifically (e.g., resource replacement, deletion, data loss potential) when present in the plan.
- Users must be able to view the explanation next to the plan it describes.

**Considerations:**
- How is the explanation validated for accuracy against the actual plan, so that an incorrect but confident-sounding explanation doesn't itself become a governance risk?

### Verifier availability behavior

**Narrative:** The product's core capability — calibrated verification — currently depends on Jev. If Jev is unavailable or rate-limited, the product needs defined behavior rather than an undefined failure, so that governance doesn't silently fail open (unreviewed changes proceeding) or fail closed (blocking all changes) without anyone knowing.

**Acceptance Criteria:**
- The system must detect when a risk score cannot be obtained for a plan.
- Users must be notified when a change is proceeding without a risk score, rather than that fact being silent.
- The system must have a defined, documented fallback behavior (e.g., default to requiring manual review) for changes that can't be scored.

**Considerations:**
- Should there be a secondary verifier (a different model, or an in-house calibration layer) as a fallback, or is "default to manual review" sufficient for the phases where this ships? This is an open dependency risk, not yet resolved. See [[jev-calibration]].

## Phase 2: Auto-Apply in Development

### Auto-apply in dev

**Narrative:** Once shadow mode has produced enough evidence that Jev's low-risk classifications hold up against real outcomes, low-risk changes in non-production environments can auto-apply without a human in the loop. Dev is the lowest-consequence environment to earn this in, and it's the first place the product exercises actual gating authority rather than passive scoring.

**Acceptance Criteria:**
- Users must be able to designate which environments are eligible for auto-apply.
- A change must only auto-apply if its risk score falls below a configured threshold.
- Users must be able to adjust or override the auto-apply threshold for their organization.
- Any change that does not meet the auto-apply threshold must continue to route to manual review, unchanged from current behavior.

**Considerations:**
- What threshold should be the default, and should it be set globally or be allowed to vary per workspace/team, given that risk tolerance likely differs across teams within the same org?

### Auto-apply audit trail

**Narrative:** Every auto-applied change needs to be reviewable after the fact, both so platform teams can spot-check the system's judgment and so a track record can be built toward earning production autonomy later.

**Acceptance Criteria:**
- Every auto-applied change must be logged with its risk score, explanation, and outcome.
- Users must be able to filter the audit trail by workspace, time range, and outcome.
- Users must be able to flag an auto-applied change after the fact as one that should not have been auto-applied, feeding back into the labeled-outcome data set.

**Considerations:**
- Does a post-hoc "should not have auto-applied" flag need any downstream consequence (e.g., automatically pausing auto-apply for that change class) in this phase, or is it purely a labeling mechanism for now?

## Phase 3: Production Autonomy and the Question Tier

### Earned production autonomy

**Narrative:** Production auto-apply is not granted by default — it's extended incrementally based on the track record built in shadow mode and dev. This is the core trust mechanic of the product: autonomy is earned, not configured on day one.

**Acceptance Criteria:**
- Production auto-apply must be unavailable until an organization has a qualifying track record (defined by accumulated dev/shadow outcomes matching predicted risk).
- Users must be able to see what track record is required and their current progress toward it.
- Production auto-apply thresholds must be independently configurable from dev thresholds, and must default to a more conservative setting.

**Considerations:**
- What specific track-record criteria (volume of changes, time window, outcome accuracy) are sufficient to justify production autonomy, and who signs off on an organization crossing that bar — the platform, or the customer?

### Medium-risk question tier

**Narrative:** This is the long-term differentiator: changes that are neither clearly safe nor clearly dangerous get converted into a specific, plain-English question the original requester answers directly (e.g., "this will drop and recreate the database; is downtime acceptable?"), rather than being escalated to an expert or blocked outright. Deterministic policy can't do this because it can't reason about intent; blanket approval can't do this because it doesn't triage at all.

**Acceptance Criteria:**
- A change scored in the medium-risk band must generate a specific question grounded in the actual consequence of the plan, not a generic warning.
- The requester must be able to answer the question directly, and their answer must determine whether the change proceeds, is modified, or escalates further.
- If the requester doesn't respond within a defined window, the change must fall back to expert review rather than remaining stuck indefinitely.
- Users must be able to see the history of questions asked and how they were answered, for audit purposes.

**Considerations:**
- How is the medium-risk band itself defined and tuned — is it a fixed score range, or does it adapt per organization based on their earned track record and risk tolerance?
- What happens if a requester answers a question incorrectly (e.g., says downtime is acceptable but it isn't) — is there any recourse or is this treated the same as an expert's approval would be?

## Phase 4: Governing Agent-Authored Change

### Agent-authored change governance

**Narrative:** As AI coding agents begin opening infrastructure PRs directly, those changes need to go through the same scoring, auto-apply, and escalation logic as human-authored ones — this is the governance gap incumbent tools aren't focused on, and it's the long-term entry point for treating the product as a governance layer for any agent's infrastructure actions, not just a review-queue triage tool.

**Acceptance Criteria:**
- The system must identify whether a given infrastructure change originated from an AI coding agent versus a human.
- Agent-originated changes must be scored, explained, and gated using the same mechanisms as human-originated changes.
- Users must be able to view agent-originated changes as a distinct, filterable category in the audit trail.
- Organizations must be able to set different (e.g., more conservative) auto-apply thresholds specifically for agent-originated changes, independent of human-originated thresholds.

**Considerations:**
- Does an agent-originated change carry any additional context (e.g., the agent's own stated intent or confidence) that should factor into scoring, or is a Terraform plan treated identically regardless of who/what authored it?
- How is agent authorship reliably determined, especially as agent tooling evolves and PR metadata may not consistently identify the author as an agent?

## Measuring Success

- Engineer-weeks saved via the bulk upgrade wedge, measured as manual review time avoided across the workspaces covered by a bulk run.
- Shadow-mode calibration accuracy: correlation between Jev's predicted risk score and actual outcome (clean apply, rollback, incident, manual override) across accumulated plans.
- Volume of labeled outcomes accumulated during shadow mode, as the leading indicator for whether there's enough evidence to advance a phase.
- Reduction in the proportion of changes requiring full manual review, without a corresponding increase in missed high-risk changes or incidents.
- Requester response rate and resolution time on medium-risk questions, once that tier ships.

## User Research

This PRD is grounded in an internal product strategy analysis rather than direct customer interviews or call transcripts. No customer names, quotes, or call dates were provided as source evidence — the input was a synthesized thesis covering the current state of Terraform review, the emergence of AI-agent-authored infrastructure PRs, target buyer and segment definition, and named risks the team is pricing in (verifier dependency, unproven calibration on infra data, uncertain cross-customer data moat, incumbent competition, sparse ground truth).

Given this, the JTBDs, personas, and phasing above should be treated as a strategic hypothesis to be validated, not as claims backed by direct customer evidence. Before committing engineering investment past Phase 1, this needs validation with actual prospective customers (per the described buyer/segment: Head or Director of Platform Engineering at 1,000–10,000-employee companies running hundreds of Terraform workspaces, on HCP Terraform, Terraform Enterprise, Spacelift, env0, or Scalr, who have piloted AI coding agents) to confirm the problem framing and the wedge's appeal before further phases are built.

## Out of Scope

- **Replacing deterministic policy engines (Sentinel, OPA, Checkov):** the product is a complementary layer, not a replacement for existing rule-based policy tooling.
- **Non-Terraform IaC tools (Pulumi, CloudFormation, CDK, etc.):** initial scope is Terraform-only; other IaC ecosystems are not addressed by this PRD.
- **Building a proprietary calibrated verifier:** the product depends on Jev for calibrated scoring; an in-house or alternative verifier is a fallback risk to price in, not an initial deliverable.
- **Full production auto-apply at launch:** production autonomy is explicitly earned through track record (Phase 3), not available from day one.
- **Cross-customer/pooled model training:** pooling plans and outcomes across tenants is treated as unresolved upside (legal and opt-in questions unresolved), not a Phase 1–4 deliverable.
- **Serving hard-regulatory-mandate customers (banks, defense, etc.):** these customers require deterministic human sign-off by legal mandate, which this product's probabilistic model doesn't satisfy; they are explicitly not the initial market.
- **Serving small teams with a handful of workspaces:** these teams don't feel the review-bottleneck pain this product addresses and are not the initial target segment.
- **General CI/CD pipeline governance beyond Terraform plan/apply:** scope is the Terraform change lifecycle specifically, not broader deployment pipelines.
- **Authoring tooling for AI coding agents:** the product governs agent-authored infrastructure changes; it does not provide the agent framework, IDE integration, or authoring experience itself.
