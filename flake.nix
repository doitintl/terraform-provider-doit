{
  description = "terraform-provider-doit local dev environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/544961dfcce86422ba200ed9a0b00dd4b1486ec5"; #v25.05
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          config.allowUnfree = true;
        };
        
        go = pkgs.go_1_25;                # Go 1.25.1
        
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