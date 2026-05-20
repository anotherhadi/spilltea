{
  pkgs,
  buildGoApplication,
}: let
  pname = "spilltea";
  version = "0.0.6";
  ldflags = ["-s" "-w" "-X main.version=${version}"];
  pkg = buildGoApplication {
    inherit pname version ldflags;
    src = ../.;
    modules = ./gomod2nix.toml;
    meta = with pkgs.lib; {
      description = "A minimal, terminal-based HTTP(S) proxy for pentesters and CTF players.";
      homepage = "https://github.com/anotherhadi/spilltea";
      platforms = platforms.unix;
    };
  };
in {
  "${pname}" = pkg;
  default = pkg;
}
