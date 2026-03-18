{
  description = "terraform-provider-doit local dev environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/5293ade07b52a432be927664aef36bdb283016fb"; # includes go_1_26
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          config.allowUnfree = true;
        };

        go = pkgs.go_1_26;                # Go 1.26.0

        buildInputs = with pkgs; [
          go
          golangci-lint  # Go linter v2.5.0
          terraform      # v1.13.3
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
