{
  description = "terraform-provider-doit local dev environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/b6c8664de9b6cc07fe5666a29f91884ba81197c4"; # includes Go 1.27.1
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          config.allowUnfree = true;
        };

        go = pkgs.go_1_27;                # Go 1.27.1

        buildInputs = with pkgs; [
          go
          golangci-lint  # Go linter v2.14.0
          terraform      # v1.16.4
        ];

      in
      {
        devShells.default = pkgs.mkShell {
          inherit buildInputs;

          shellHook = ''
            # Show versions
            echo "  Go: $(go version | cut -d' ' -f3)"
            echo "  Terraform: $(terraform -v)"
            echo "  golangci-lint: $(golangci-lint version --format short 2>/dev/null || echo 'v2.5.0')"
          '';
        };
      }
    );
}
