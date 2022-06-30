---
name: Deployment

on:
  workflow_call:
    inputs:
      enable_release:
        description: Should we create an official release from the tag.
        default: true
        required: true
        type: boolean

jobs:
  release:
    name: Github Release
    runs-on: ubuntu-latest
    if: inputs.enable_release == true
    steps:
      - name: Clone repo
        uses: actions/checkout@v3
      - name: Release
        uses: softprops/action-gh-release@v1
        with:
          generate_release_notes: true
          token: {{ "${{" }} secrets.GITHUB_TOKEN {{ "}}" }}
