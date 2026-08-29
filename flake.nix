{
  description = "Development shell for reviewr";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    flake-parts = {
      url = "github:hercules-ci/flake-parts";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };
    crane.url = "github:ipetkov/crane";
    rust-overlay = {
      url = "github:oxalica/rust-overlay";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = inputs @ {flake-parts, ...}:
    flake-parts.lib.mkFlake {inherit inputs;} {
      systems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];

      perSystem = {system, ...}: let
        pkgs = import inputs.nixpkgs {
          inherit system;
          overlays = [inputs.rust-overlay.overlays.default];
        };
        rustToolchainFor = p: p.rust-bin.fromRustupToolchainFile ./rust-toolchain.toml;
        rustToolchain = rustToolchainFor pkgs;
        craneLib = (inputs.crane.mkLib pkgs).overrideToolchain rustToolchainFor;
        reviewr = pkgs.callPackage ./package.nix {inherit craneLib;};
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
            rustToolchain
            go
            git
            just
            pkg-config
            python3
          ];
        };
      };
    };
}
