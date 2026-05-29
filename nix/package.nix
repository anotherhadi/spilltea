{
  pkgs,
  buildGoApplication,
}: let
  browser = import ./browser.nix {inherit pkgs;};
  pname = "spilltea";
  version = "0.0.7";
  ldflags = ["-s" "-w" "-X main.version=${version}"];
  pkg = buildGoApplication {
    inherit pname version ldflags;
    src = ../.;
    modules = ./gomod2nix.toml;
    nativeBuildInputs = [pkgs.installShellFiles];
    env.GOTOOLCHAIN = "local";
    postInstall = ''
      installShellCompletion --cmd spilltea \
        --bash <($out/bin/spilltea completion bash) \
        --zsh <($out/bin/spilltea completion zsh) \
        --fish <($out/bin/spilltea completion fish)
    '';
    meta = with pkgs.lib; {
      description = "A minimal, terminal-based HTTP(S) proxy for pentesters and CTF players.";
      homepage = "https://github.com/anotherhadi/spilltea";
      platforms = platforms.unix;
    };
  };
in {
  "${pname}" = pkg;
  default = pkg;
  browser = browser;
}
