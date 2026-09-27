package phone

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Browsers expose a phone's motion sensors only to pages served over HTTPS.
// A program on a garage laptop has no domain and no certificate authority, so
// it makes its own certificate: the phone's browser warns once, the person
// taps through, and the sensors work. The certificate is kept so the warning
// is not repeated on every start, and remade when the computer's addresses
// change, since the addresses are written into it.

// certificate returns a TLS certificate valid for the given addresses, loading
// it from dir when a suitable one is already there.
func certificate(dir string, ips []net.IP) (tls.Certificate, error) {
	certPath := filepath.Join(dir, "phone-cert.pem")
	keyPath := filepath.Join(dir, "phone-key.pem")

	if c, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil && covers(c, ips) {
		return c, nil
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	if err != nil {
		return tls.Certificate{}, err
	}
	host, _ := os.Hostname()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "Сход-развал — открытый стенд", Organization: []string{"wheelalign"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(2, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  append([]net.IP{net.IPv4(127, 0, 0, 1)}, ips...),
		DNSNames:     []string{"localhost"},
	}
	if host != "" {
		tmpl.DNSNames = append(tmpl.DNSNames, host)
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	kb, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb})

	if err := os.MkdirAll(dir, 0o700); err == nil {
		// Failing to save only means a new warning next time; not fatal.
		_ = os.WriteFile(certPath, certPEM, 0o644)
		_ = os.WriteFile(keyPath, keyPEM, 0o600)
	}
	c, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("сертификат: %w", err)
	}
	return c, nil
}

// covers reports whether a stored certificate is still usable: not near expiry
// and naming every current address.
func covers(c tls.Certificate, ips []net.IP) bool {
	if len(c.Certificate) == 0 {
		return false
	}
	x, err := x509.ParseCertificate(c.Certificate[0])
	if err != nil || time.Until(x.NotAfter) < 30*24*time.Hour {
		return false
	}
	for _, ip := range ips {
		found := false
		for _, have := range x.IPAddresses {
			if have.Equal(ip) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// lanIPs lists the computer's IPv4 addresses on the local network, the likely
// Wi-Fi one first.
func lanIPs() []net.IP {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var first, rest []net.IP
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipn.IP.To4()
			if ip == nil || ip.IsLinkLocalUnicast() {
				continue
			}
			// 192.168.x.x is what home routers and phone hotspots hand out;
			// virtual adapters (WSL, VirtualBox, Docker) tend to sit in
			// 172.16/12 and are listed last.
			if ip[0] == 192 && ip[1] == 168 {
				first = append(first, ip)
			} else {
				rest = append(rest, ip)
			}
		}
	}
	return append(first, rest...)
}
