## Configuring your browser's proxy settings

We recommend installing **FoxyProxy** to manage your browser's proxies.
You can install it from the [Google Chrome extension store](https://chromewebstore.google.com/) or from the [Firefox extension store](https://addons.mozilla.org/en-US/firefox/extensions)

1. Open FoxyProxy's options, then click on `Add New Proxy` button.
2. Click the "Manual Proxy Configuration" radio button. Set the "HTTP Proxy" field to `{{.Cfg.App.Host}}` and the "Port" field to `{{.Cfg.App.Port}}`. Click "Save".
3. Forward traffic to Spilltea by selecting the new proxy in FoxyProxy's extension button.
4. You're all set! You can now use Spilltea.

If `proxy_auth` is set in the config (`user:pass`), enter the same credentials in FoxyProxy under "Username" and "Password" in the proxy settings.
