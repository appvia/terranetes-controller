---
name: Release

on:
  push:
    tags:
      - "v*"

jobs:
  verify:
    name: Review
    uses: ./.github/workflows/main.yaml
    secrets: inherit
    with:
      check_commits: {{ .EnsureCommitLint }}
      check_policy: {{ .EnsurePolicyLint }}
      policy_source: '{{ .PolicySource }}'
      policy_version: '{{ .PolicyVersion }}'

  release:
    needs: verify
    name: Release
    uses: ./.github/workflows/deployment.yaml
    secrets: inherit
    with:
      enable_release: true
