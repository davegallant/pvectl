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

      version = self.shortRev or self.dirtyShortRev or "unknown";

      pvectl =
        pkgs:
        pkgs.buildGo125Module {
          name = "pvectl";
          src = ./.;
          subPackages = [ "cmd/pvectl" ];
          vendorHash = "sha256-F5RgDmFwI4D1H1UbEg2i7WT+U/Cf2BwJer0bX6Dn9vc=";
          nativeBuildInputs = pkgs.lib.optionals pkgs.stdenv.isLinux [ pkgs.makeWrapper ];
          env.CGO_ENABLED = 0;
          doCheck = false;
          ldflags = [
            "-s"
            "-w"
            "-X github.com/davegallant/pvectl/cmd.version=${version}"
          ];
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
              asciinema
              asciinema-agg
              gifsicle
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
