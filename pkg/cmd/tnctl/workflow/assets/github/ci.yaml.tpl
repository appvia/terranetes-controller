---
name: Review

on:
  pull_request:

jobs:
  review:
    name: Review
    uses: ./.github/workflows/main.yaml
    secrets: inherit
    with:
      check_commits: {{ .EnsureCommitLint }}
      check_policy: {{ .EnsurePolicyLint }}
      policy_source: '{{ .PolicySource }}'
      policy_version: '{{ .PolicyVersion }}'
