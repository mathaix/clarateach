# Feature Documentation Workflow

This document describes the workflow for tracking features from inception to release.

## Overview

```
┌─────────────────┐     PR Merged      ┌─────────────────────┐
│ CurrentFeature  │ ─────────────────► │ Feature_<PR#>.md    │
│ .md             │                    │                     │
└─────────────────┘                    └──────────┬──────────┘
        ▲                                         │
        │                                         ▼
        │                              ┌─────────────────────┐
   New feature                         │ RELEASE.md          │
   starts                              │ (link added)        │
                                       └─────────────────────┘
```

## Workflow Steps

### 1. Start a New Feature

Create or update `docs/CurrentFeature.md` with:
- Feature overview and goals
- Architecture diagrams
- Implementation plan
- Open questions
- Testing checklist

**Only one `CurrentFeature.md` should exist at a time.** If multiple features are in progress, use separate branches with their own `CurrentFeature.md`.

### 2. During Development

Update `CurrentFeature.md` as the feature evolves:
- Check off completed items
- Document decisions made
- Add learnings and gotchas
- Update architecture if it changes

### 3. When PR is Merged

After the feature PR is merged to `main`:

```bash
# Get the PR number (e.g., 42)
PR_NUMBER=42

# Rename CurrentFeature.md
git mv docs/CurrentFeature.md docs/Feature_${PR_NUMBER}.md

# Add link to RELEASE.md
echo "- [Feature ${PR_NUMBER}](docs/Feature_${PR_NUMBER}.md) - <feature title>" >> RELEASE.md

# Commit
git add -A
git commit -m "docs: Archive Feature_${PR_NUMBER}.md and update RELEASE.md"
git push
```

### 4. Start Next Feature

Create a new `docs/CurrentFeature.md` for the next feature.

## File Locations

```
backend/
├── docs/
│   ├── CurrentFeature.md          # Active feature being worked on
│   ├── CurrentFeatureWorkflow.md  # This document
│   ├── Feature_42.md              # Archived: PR #42
│   ├── Feature_56.md              # Archived: PR #56
│   └── ...
├── RELEASE.md                     # Links to all shipped features
└── ...
```

## RELEASE.md Format

```markdown
# Release Notes

## Features

- [Feature 42](docs/Feature_42.md) - GCP + Firecracker Integration
- [Feature 56](docs/Feature_56.md) - Multi-tenant Workspace Isolation
- ...

## Bug Fixes

- PR #43 - Fix VM termination on HTTP request completion
- ...
```

## Benefits

1. **Audit Trail**: Every shipped feature has documentation
2. **Knowledge Base**: New team members can understand past decisions
3. **Release Notes**: Automatic changelog with links to details
4. **Single Source of Truth**: `CurrentFeature.md` is always the active work

## Example

### Before PR Merge

```
docs/
├── CurrentFeature.md  ← "GCP + Firecracker Integration"
└── CurrentFeatureWorkflow.md
```

### After PR #42 Merged

```
docs/
├── Feature_42.md      ← Renamed from CurrentFeature.md
└── CurrentFeatureWorkflow.md

RELEASE.md:
- [Feature 42](docs/Feature_42.md) - GCP + Firecracker Integration
```

### New Feature Starts

```
docs/
├── CurrentFeature.md  ← New: "Real-time Collaboration"
├── Feature_42.md
└── CurrentFeatureWorkflow.md
```

## Automation (Future)

Consider adding a GitHub Action that:
1. Triggers on PR merge to `main`
2. Checks if `docs/CurrentFeature.md` exists
3. Renames it to `docs/Feature_<PR#>.md`
4. Updates `RELEASE.md`
5. Creates commit

---

**See Also**: [CurrentFeature.md](CurrentFeature.md) - The active feature being developed
