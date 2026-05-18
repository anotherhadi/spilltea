{
  description = "Spilltea: A minimal, terminal-based HTTP(S) proxy for pentesters and CTF players.";

  inputs = {nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";};

  outputs = {
    self,
    nixpkgs,
  }: let
    supportedSystems = ["x86_64-linux" "aarch64-linux"];

    forAllSystems = f:
      nixpkgs.lib.genAttrs supportedSystems
      (system: f system (import nixpkgs {inherit system;}));

    pname = "spilltea";
    version = "0.0.3";

    ldflags = ["-s" "-w" "-X main.version=${version}"];
  in {
    packages = forAllSystems (system: pkgs: let
      pkg = pkgs.buildGoModule {
        inherit pname version ldflags;

        src = ./.;
        outputs = ["out"];

        vendorHash = "sha256-v37RFS/T6KGZTO1tHmtUqBrRcCqNS3+ACBcsd7tl50c=";

        meta = with pkgs.lib; {
          description = "A minimal, terminal-based HTTP(S) proxy for pentesters and CTF players.";
          homepage = "https://github.com/anotherhadi/spilltea";
          platforms = platforms.unix;
        };
      };
    in {
      "${pname}" = pkg;
      default = pkg;
    });

    devShells = forAllSystems (system: pkgs: {
      default = pkgs.mkShell {
        packages = with pkgs; [
          go
          python3
          lefthook
          doctoc
        ];

        shellHook = ''
          lefthook install
        '';
      };
    });
  };
}
