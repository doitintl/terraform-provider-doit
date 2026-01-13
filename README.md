# DoiT Cloud Intelligence Provider

This Terraform provider allows you to manage DoiT Cloud Intelligence resources using the [DoiT API](https://developer.doit.com/reference/welcome).

While the provider does not yet support all available API endpoints, version 1.0.0 marks the first stable release.

Future releases will add support for additional resources and data sources.

To get started with the provider, you first need to create an [API key](https://developer.doit.com/docs/start#api-keys) for the DoiT Console.

You then configure the provider like so:

```terraform
terraform {
  required_providers {
    doit = {
      source  = "doitintl/doit"
      version = "1.0.0"
    }
  }
}

provider "doit" {
  api_token = "your-api-key"
}
```

If you are a Do'er, you also need to define the customer context:

```terraform
provider "doit" {
  api_token = "your-api-key"
  customer_context = "your-customer-context"
}
```

Alternatively, you can also use the `DOIT_API_TOKEN` and `DOIT_CUSTOMER_CONTEXT` environment variables.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.19

### Optional: Using Nix Flake

If you have [Nix](https://nixos.org/download.html) installed with flakes enabled:

```shell
nix develop
```

Or with [direnv](https://direnv.net/) for automatic environment loading:

```shell
direnv allow
```

This will automatically set up:
- Go 1.25
- Terraform v1.13.3
- golangci-lint v2.5.0

To enable flakes if not already enabled, add to your `~/.config/nix/nix.conf`:
```
experimental-features = nix-command flakes
```

## Building The Provider

1. Clone the repository
1. Enter the repository directory
1. Build the provider using the Go `install` command:

```shell
go install
```

## Adding Dependencies

This provider uses [Go modules](https://github.com/golang/go/wiki/Modules).
Please refer to the Go documentation for the most up-to-date information on using Go modules.

To add a new dependency `github.com/author/dependency` to your Terraform provider:

```shell
go get github.com/author/dependency
go mod tidy
```

Then commit the changes to `go.mod` and `go.sum`.

## Using the provider

Please check the docs folder to find an example to use the DoiT Console provider.

## Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements) above).

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

To generate or update documentation, run `go generate`.

## Running Acceptance Tests

Acceptance tests create real resources in a DoiT account and require proper configuration.

### Required Environment Variables

| Variable | Description |
|----------|-------------|
| `DOIT_API_TOKEN` | Your DoiT API token |
| `DOIT_HOST` | The DoiT API host (e.g., `https://api.doit.com`) |
| `TEST_USER` | Email address for test budget collaborators/recipients |
| `TEST_ATTRIBUTION` | Attribution ID for test budget scope |
| `TEST_SLACK_CHAN` | Slack channel ID for notification tests |
| `TEST_PROJECT` | Project ID for allocation rule tests |
| `TEST_CUSTOMER_ID` | Customer ID for Slack channel recipient |

### Running Tests

#### Using direnv (Recommended)

If you use [direnv](https://direnv.net/), copy the example file and fill in your values:

```shell
cp .envrc.example .envrc.local
# Edit .envrc.local with your values
direnv allow
```

Then run tests:

```shell
go test -v -timeout 120m ./...
```

#### Manual Setup

Set the required environment variables and run:

```shell
export TF_ACC=1
export DOIT_API_TOKEN="your-api-token"
export DOIT_HOST="https://api.doit.com"
export TEST_USER="your-email@example.com"
export TEST_ATTRIBUTION="your-attribution-id"
export TEST_SLACK_CHAN="your-slack-channel-id"
export TEST_PROJECT="your-project-id"
export TEST_CUSTOMER_ID="your-customer-id"

go test -v -timeout 120m ./...
```

> [!IMPORTANT]
> **DoiT employees only:** You must also set the `DOIT_CUSTOMER_CONTEXT` environment variable
> to the same value as `TEST_CUSTOMER_ID` for certain tests to work correctly.

*Note:* Acceptance tests create real resources and may incur costs.
