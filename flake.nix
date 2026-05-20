{
  description = "Spilltea: A minimal, terminal-based HTTP(S) proxy for pentesters and CTF players.";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    git-hooks = {
      url = "github:cachix/git-hooks.nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = {
    self,
    nixpkgs,
    gomod2nix,
    git-hooks,
  }: let
    supportedSystems = ["x86_64-linux" "aarch64-linux"];

    forAllSystems = f:
      nixpkgs.lib.genAttrs supportedSystems
      (system: f system (import nixpkgs {inherit system;}));
  in {
    packages = forAllSystems (system: pkgs:
      import ./nix/package.nix {
        inherit pkgs;
        buildGoApplication = gomod2nix.legacyPackages.${system}.buildGoApplication;
      });

    devShells = forAllSystems (system: pkgs: {
      default = import ./nix/shell.nix {
        inherit pkgs;
        gitHooksLib = git-hooks.lib.${system};
        gomod2nixPkgs = gomod2nix.legacyPackages.${system};
      };
    });
  };
}
