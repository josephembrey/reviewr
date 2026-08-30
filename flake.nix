{
  description = "Development shell for reviewr";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    flake-parts = {
      url = "github:hercules-ci/flake-parts";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };
  };

  outputs = inputs @ {flake-parts, ...}:
    flake-parts.lib.mkFlake {inherit inputs;} {
      systems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];

      perSystem = {pkgs, ...}: let
        reviewr = pkgs.callPackage ./package.nix {};
      in {
        packages = {
          default = reviewr;
          inherit reviewr;
        };

        apps.default = {
          type = "app";
          program = pkgs.lib.getExe reviewr;
          meta.description = reviewr.meta.description;
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            actionlint
            alejandra
            deadnix
            go
            git
            just
            pkg-config
            prek
          ];
        };
      };
    };
}
