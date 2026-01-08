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

In order to run the full suite of Acceptance tests, run `make testacc`.

*Note:* Acceptance tests create real resources, and often cost money to run.

```shell
make testacc
```
