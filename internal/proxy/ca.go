package proxy

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io/fs"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/cert"
)

// CAName is the name the CA shows in the browser's certificate store.
const CAName = "spilltea CA"

// File names go-mitmproxy expects in the CA folder.
const (
	caKeyFile  = "mitmproxy-ca.pem" // private key, then certificate
	caCertFile = "mitmproxy-ca-cert.pem"
	caCerFile  = "mitmproxy-ca-cert.cer"
)

// LoadCA loads the CA from dir, or creates it. An existing CA (spilltea's or
// mitmproxy's) is kept as is, since browsers already trust it.
//
// spilltea creates the CA itself rather than letting go-mitmproxy do it: the
// latter names it "mitmproxy" and writes the private key with default
// permissions (0644), readable by any local user.
func LoadCA(dir string) (cert.CA, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	key := filepath.Join(dir, caKeyFile)
	if _, err := os.Stat(key); errors.Is(err, fs.ErrNotExist) {
		if err := createCA(dir); err != nil {
			return nil, err
		}
	}
	// a CA created by an older spilltea, or by go-mitmproxy
	if fi, err := os.Stat(key); err == nil && fi.Mode().Perm()&0o077 != 0 {
		if err := os.Chmod(key, 0o600); err != nil {
			return nil, err
		}
	}
	return cert.NewSelfSignCA(dir)
}

func createCA(dir string) error {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: CAName, Organization: []string{"spilltea"}},
		NotBefore:             now.Add(-48 * time.Hour), // clocks that run late
		NotAfter:              now.AddDate(3, 0, 0),
		BasicConstraintsValid: true,
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		// unlike go-mitmproxy's, a stolen key cannot sign code or mail
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return err
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := append(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}), certPEM...)
	// O_EXCL: two spilltea starting together must not overwrite each other's CA
	f, err := os.OpenFile(filepath.Join(dir, caKeyFile), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = f.Write(keyPEM)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	for _, name := range []string{caCertFile, caCerFile} {
		if err := os.WriteFile(filepath.Join(dir, name), certPEM, 0o644); err != nil {
			return err
		}
	}
	return nil
}
