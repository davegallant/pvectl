{
  description = "pvectl - CLI for managing a Proxmox VE cluster over the REST API";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];

      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f system);

      pvectl =
        pkgs:
        pkgs.buildGo125Module {
          name = "pvectl";
          src = ./.;
          subPackages = [ "cmd/pvectl" ];
          vendorHash = "sha256-KyRVwlew7LdcTMGKzkDHbgezwy1dYpN3pPt+SITBAzg=";
          nativeBuildInputs = pkgs.lib.optionals pkgs.stdenv.isLinux [ pkgs.makeWrapper ];
          env.CGO_ENABLED = 0;
          doCheck = false;
        };
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          p = pvectl pkgs;
        in
        {
          default = p;
          pvectl = p;
        }
      );

      devShells = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.mkShell {
            name = "pvectl-dev";
            packages = with pkgs; [
              go_1_25
              gopls
              gotools
              golangci-lint
              just
            ];
            shellHook = ''
              echo "Welcome to the pvectl dev environment"
              go version
            '';
          };
        }
      );
    };
}
