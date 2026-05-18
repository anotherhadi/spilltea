## CA Certificate Installation

1. Copy your **CA Certificate** located in `{{.Cfg.App.CertDir}}`
2. Install your certificate:
   - On Chrome:
     - Open your Chrome settings, search for "Certificates" and click on "Security".
     - In the security settings page, scroll down and click on "Manage certificates".
     - Select the "Authorities" tab and click on "Import tab and click on "Import".
     - Select the `mitmproxy-ca-cert.pem` file in `{{.Cfg.App.CertDir}}`.
   - On Firefox:
     - Open your Firefox settings, search for "Certificates" and click on "View Certificates".
     - Select the "Authorities" tab and click on "Import".
     - Select the `mitmproxy-ca-cert.pem` file in `{{.Cfg.App.CertDir}}`.
     - When prompted, click the "Trust this CA to identify websites" checkbox, then click on "OK".

> [!WARNING]
> Never install this certificate in a permanent system trust store: it grants decryption of all HTTPS traffic. Remove it from your browser after use.
