package selfcert

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"testing"
)

func TestGenerate(t *testing.T) {
	cert, err := Generate()
	if err != nil {
		t.Fatal(err)
	}

	if len(cert.Certificate) == 0 {
		t.Fatal("expected at least one certificate")
	}
	if cert.PrivateKey == nil {
		t.Fatal("expected private key")
	}

	// Parse and verify the certificate.
	parsed, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}

	if parsed.Subject.CommonName != "Flipper" {
		t.Errorf("CN = %q, want Flipper", parsed.Subject.CommonName)
	}

	// Check SANs include localhost.
	hasLocalhost := false
	for _, dns := range parsed.DNSNames {
		if dns == "localhost" {
			hasLocalhost = true
			break
		}
	}
	if !hasLocalhost {
		t.Error("expected localhost in DNS SANs")
	}

	// Check SANs include 127.0.0.1.
	hasLoopback := false
	for _, ip := range parsed.IPAddresses {
		if ip.Equal(net.IPv4(127, 0, 0, 1)) {
			hasLoopback = true
			break
		}
	}
	if !hasLoopback {
		t.Error("expected 127.0.0.1 in IP SANs")
	}

	// Verify the cert can be used in a TLS config.
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	if len(tlsCfg.Certificates) != 1 {
		t.Errorf("expected 1 certificate in TLS config, got %d", len(tlsCfg.Certificates))
	}
}

func TestLocalIPs(t *testing.T) {
	ips := localIPs()
	if len(ips) < 2 {
		t.Fatalf("expected at least 2 IPs (loopback v4+v6), got %d", len(ips))
	}

	hasV4Loopback := false
	for _, ip := range ips {
		if ip.Equal(net.IPv4(127, 0, 0, 1)) {
			hasV4Loopback = true
			break
		}
	}
	if !hasV4Loopback {
		t.Error("expected 127.0.0.1 in local IPs")
	}
}
