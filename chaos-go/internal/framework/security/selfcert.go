// Package security 提供开发 / 内网环境下的自签名证书生成能力，
// 用于在不依赖外部 CA 的前提下启用 HTTPS 与 HTTP/3（QUIC）。
// 生产环境应使用受信任证书，而非本包生成的临时自签名证书。
package security

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

// SelfSignedCert 持有自签名证书及其 PEM 明文，便于落盘供客户端导入信任。
type SelfSignedCert struct {
	Certificate tls.Certificate
	CertPEM     []byte
	KeyPEM      []byte
}

// GenerateSelfSigned 生成一张有效期 10 年的自签名证书，SAN 覆盖 localhost 与本机所有 IPv4 地址，
// 供开发 / 内网环境启用 HTTPS + HTTP/3 使用（生产环境应替换为受信任证书）。
func GenerateSelfSigned() (*SelfSignedCert, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("生成私钥失败: %w", err)
	}

	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "chaos.local", Organization: []string{"chaos"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
		DNSNames:              []string{"localhost"},
		IPAddresses:           collectIPs(),
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, fmt.Errorf("生成证书失败: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("编码私钥失败: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	return &SelfSignedCert{Certificate: cert, CertPEM: certPEM, KeyPEM: keyPEM}, nil
}

// Save 将证书与私钥写入指定目录，返回文件路径。
func (s *SelfSignedCert) Save(dir string) (certPath, keyPath string, err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	certPath = filepath.Join(dir, "chaos.crt")
	keyPath = filepath.Join(dir, "chaos.key")
	if err := os.WriteFile(certPath, s.CertPEM, 0o644); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(keyPath, s.KeyPEM, 0o600); err != nil {
		return "", "", err
	}
	return certPath, keyPath, nil
}

// collectIPs 收集本机所有 IPv4 地址（含回环），用于证书 SAN，确保局域网通过 IP 访问时也能匹配。
func collectIPs() []net.IP {
	ips := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ips
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok {
			if ip4 := ipnet.IP.To4(); ip4 != nil {
				ips = append(ips, ip4)
			}
		}
	}
	return ips
}
