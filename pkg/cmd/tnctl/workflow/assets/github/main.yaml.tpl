---
name: Pipeline

on:
  workflow_call:
    inputs:
      check_commits:
        description: Ensures all commit messages must pass linting checks
        default: false
        required: true
        type: boolean
      check_policy:
        description: Should policy checks be enforced.
        default: true
        required: true
        type: boolean
      policy_source:
        description: |
          Reference to an external repository contains the security policy.
        default: ''
        required: false
        type: string
      policy_version:
        description: The tag to checkout in the external policy repository.
        default: ''
        required: false
        type: string

jobs:
  code-linting:
    name: Linting Code
    runs-on: ubuntu-latest
    permissions:
      contents: read
      issues: write
      pull-requests: read
    outputs:
      linting: {{ "${{" }} steps.linting.outcome {{ "}}" }}
    steps:
      - name: Clone repo
        uses: actions/checkout@v3
      - name: Install Terraform
        uses: hashicorp/setup-terraform@v2
      - name: Setup Linter
        uses: terraform-linters/setup-tflint@v2
      - name: Linter Version
        run: tflint --version
      - name: Linter Initialize
        run: tflint --init
      - name: Linting Code
        id: linting
        run: tflint -f compact

  code-validate:
    name: Validating Terraform
    runs-on: ubuntu-latest
    outputs:
      validate: {{ "${{" }} steps.validate.outcome {{ "}}" }}
    steps:
      - name: Clone repo
        uses: actions/checkout@v3
      - name: Install Terraform
        uses: hashicorp/setup-terraform@v2
      - name: Terraform Initialize
        run: terraform init
      - name: Terraform Validate
        id: validate
        uses: dflook/terraform-validate@v1

  code-format:
    name: Checking Format
    runs-on: ubuntu-latest
    outputs:
      format: {{ "${{" }} steps.format.outcome {{ "}}" }}
    steps:
      - name: Clone repo
        uses: actions/checkout@v3
      - name: Install Terraform
        uses: hashicorp/setup-terraform@v2
      - name: Format Check
        id: format
        uses: dflook/terraform-fmt-check@v1

  code-docs:
    name: Terraform Docs
    runs-on: ubuntu-latest
    continue-on-error: true
    outputs:
      docs: {{ "${{" }} steps.docs.outcome {{ "}}" }}
    steps:
      - name: Clone repo
        uses: actions/checkout@v3
      - name: Generate Documentation
        uses: terraform-docs/gh-actions@v1
        with:
          output-file: README.md
          output-method: inject

  code-security:
    name: Security Check
    runs-on: ubuntu-latest
    outputs:
      security: {{ "${{" }} steps.security.outcome {{ "}}" }}
    steps:
      - name: Clone repo
        uses: actions/checkout@v3
      - name: Cloning Security Policy
        if: inputs.policy_source != ''
        uses: actions/checkout@v3
        with:
          repository: {{ "${{" }} inputs.policy_source {{ "}}" }}
          token: {{ "${{" }} secrets.policy_token {{ "}}" }}
          path: policy/
      - name: Checkov Config
        if: inputs.policy_source == ''
        run: |
          mkdir -p policy
          echo "framework: terraform" > policy/config.yaml
      - name: Using Latest Policy
        if: inputs.policy_source != '' && inputs.policy_version != 'latest'
        run: |
          cd policy
          latest=$(git describe --tags --abbrev=0)
          git checkout $latest
      - name: Running Checkov
        if: inputs.check_policy == true
        id: security
        uses: bridgecrewio/checkov-action@master
        with:
          config_file: ./policy/config.yaml
          framework: terraform

  cost-token:
    name: Check Costs Enabled
    runs-on: ubuntu-latest
    outputs:
      enable_infracost: {{ "${{" }} steps.infracost.outputs.enable_infracost {{ "}}" }}
    steps:
      - name: Check whether container scanning should be enabled
        id: infracost
        env:
          INFRACOST_API_KEY: {{ "${{" }} secrets.ORG_INFRACOST_API_KEY {{ "}}" }}
        run: |
          echo "Enable costs integration: {{ "${{" }} env.INFRACOST_API_KEY != '' {{ "}}" }}"
          echo "::set-output name=enable_infracost::{{ "${{" }} env.INFRACOST_API_KEY != '' {{ "}}" }}"

  code-costs:
    name: Cost Review
    needs: cost-token
    if: needs.cost-token.outputs.enable_infracost == 'true'
    runs-on: ubuntu-latest
    env:
      TF_ROOT: .
    outputs:
      security: {{ "${{" }} steps.costs.outcome {{ "}}" }}
    steps:
      - name: Clone repo
        uses: actions/checkout@v3
      - name: Setup Infracost
        uses: infracost/actions/setup@v2
        with:
          api-key: {{ "${{" }} secrets.ORG_INFRACOST_API_KEY {{ "}}" }}
      - name: Checkout base branch
        uses: actions/checkout@v3
        with:
          ref: '{{ "${{" }} github.event.pull_request.base.ref {{ "}}" }}'
      - name: Generate Infracost cost estimate baseline
        run: |
          infracost breakdown --path=${TF_ROOT} \
            --format=json \
            --out-file=/tmp/infracost-base.json
      - name: Checkout PR branch
        uses: actions/checkout@v3
      - name: Generate Infracost diff
        id: costs
        run: |
          infracost diff --path=${TF_ROOT} \
            --format=json \
            --compare-to=/tmp/infracost-base.json \
            --out-file=/tmp/infracost.json
      - name: Post Infracost comment
        run: |
          infracost comment github \
            --behavior=update  \
            --github-token={{ "${{" }} github.token {{ "}}" }} \
            --path=/tmp/infracost.json \
            --pull-request={{ "${{" }} github.event.pull_request.number {{ "}}" }} \
            --repo=$GITHUB_REPOSITORY

  code-commits:
    name: Lint Code Commits
    runs-on: ubuntu-latest
    if: inputs.check_commits == true
    steps:
      - name: Clone repo
        uses: actions/checkout@v3
        with:
          fetch-depth: 0
      - uses: wagoid/commitlint-github-action@v5

  review:
    name: Update Review
    if: github.event_name == 'pull_request' && always()
    runs-on: ubuntu-latest
    needs:
      - code-costs
      - code-docs
      - code-format
      - code-linting
      - code-security
      - code-validate
    steps:
      - name: Update PR
        uses: actions/github-script@0.9.0
        with:
          github-token: {{ "${{" }} secrets.GITHUB_TOKEN {{ "}}" }}
          script: |
            const { data: comments } = await github.issues.listComments({
              owner: context.repo.owner,
              repo: context.repo.repo,
              issue_number: context.issue.number,
            })
            const botComment = comments.find(comment => {
              return comment.user.type === 'Bot' && comment.body.includes('Pull Request Review Status')
            })

            const output = `### Pull Request Review Status
            * ⚙️ Terraform Format:         \`{{ "${{" }} needs.code-format.outputs.format }}\`
            * 📖 Terraform Documentation:  \`{{ "${{" }} needs.code-docs.outputs.docs }}\`
            * 🔍 Terraform Linting:        \`{{ "${{" }} needs.code-linting.outputs.linting }}\`
            * 🔒 Terraform Security:       \`{{ "${{" }} needs.code-security.outputs.security }}\`
            * 🤖 Terraform Validation:     \`{{ "${{" }} needs.code-validate.outputs.validate }}\``

            if (botComment) {
              github.issues.updateComment({
                owner: context.repo.owner,
                repo: context.repo.repo,
                comment_id: botComment.id,
                body: output
              })
            } else {
              github.issues.createComment({
                issue_number: context.issue.number,
                owner: context.repo.owner,
                repo: context.repo.repo,
                body: output
              })
            }
