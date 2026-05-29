{pkgs}: let
  defaultCertDir = "$HOME/.local/share/spilltea";
  defaultHost = "127.0.0.1";
  defaultPort = "8080";
in
  pkgs.writeShellApplication {
    name = "spilltea-browser";
    runtimeInputs = with pkgs; [nss.tools librewolf netcat-gnu procps];
    text = ''
      CERT_DIR="${defaultCertDir}"
      HOST="${defaultHost}"
      PORT="${defaultPort}"

      while [[ $# -gt 0 ]]; do
        case "$1" in
          --cert-dir) CERT_DIR="$2"; shift 2 ;;
          --host)     HOST="$2";     shift 2 ;;
          --port)     PORT="$2";     shift 2 ;;
          *) echo "Unknown argument: $1"; exit 1 ;;
        esac
      done

      CERT_FILE="$CERT_DIR/mitmproxy-ca-cert.pem"

      if [[ ! -f "$CERT_FILE" ]]; then
        echo "Certificate not found at $CERT_FILE"
        echo "Launch spilltea first, or use --cert-dir if you changed cert_dir in your config."
        exit 1
      fi

      if ! pgrep -x spilltea > /dev/null 2>&1; then
        echo "spilltea is not running. Launch spilltea first."
        exit 1
      fi

      if ! nc -z "$HOST" "$PORT" > /dev/null 2>&1; then
        echo "Nothing is listening on $HOST:$PORT."
        echo "Is spilltea configured on that port? Use --host and --port if you changed them in your config."
        exit 1
      fi

      PROFILE=$(mktemp -d)
      trap 'rm -rf "$PROFILE"' EXIT

      certutil -N -d sql:"$PROFILE" --empty-password
      certutil -d sql:"$PROFILE" -A -n "spilltea" -t "CT,," -i "$CERT_FILE"

      cat > "$PROFILE/user.js" <<EOF
      user_pref("network.proxy.type", 1);
      user_pref("network.proxy.http", "$HOST");
      user_pref("network.proxy.http_port", $PORT);
      user_pref("network.proxy.ssl", "$HOST");
      user_pref("network.proxy.ssl_port", $PORT);
      user_pref("network.proxy.no_proxies_on", "");
      user_pref("network.stricttransportsecurity.preloadlist", false);
      user_pref("dom.security.https_only_mode", false);
      EOF

      librewolf --profile "$PROFILE" --no-remote
    '';
  }
