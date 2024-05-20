# Terraform/Terragrunt Lock Checker

This is a CLI tool to list and manage Terraform/Terragrunt locks in AWS DynamoDB and Azure Blob Storage.

## Installation

You can install the tool using `go get`:

```sh
go get github.com/naviat/terraform-lock-checker
```

## Usage

1. Set the appropriate environment variables for your cloud provider:

**For AWS:**

```sh
export AWS_REGION=your-aws-region
```

**For Azure:**

```sh
export AZURE_STORAGE_ACCOUNT=your-storage-account
export AZURE_STORAGE_KEY=your-storage-key
```

2. Then, run the CLI tool:

`terraform-lock-checker`

The tool will auto-detect the environment and prompt you to unlock any detected lock items.

3. Install the Tool:

Others can now install your tool using go get.

`go get github.com/naviat/terraform-lock-checker`
