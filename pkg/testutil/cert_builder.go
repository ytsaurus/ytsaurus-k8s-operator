package testutil

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
)

const (
	TestIssuerName      = "ytsaurus-dev-ca"
	TestTrustBundleName = "ytsaurus-dev-ca.crt"
)

type CertBuilder struct {
	Namespace   string
	DNSZones    []string
	IPAddresses []string
}

func (b *CertBuilder) BuildCertificate(name string, hostNames []string) *certv1.Certificate {
	dnsNames := make([]string, 0, len(hostNames)*len(b.DNSZones))
	if len(b.DNSZones) == 0 {
		dnsNames = hostNames
	}
	for _, zone := range b.DNSZones {
		for _, hostName := range hostNames {
			dnsNames = append(dnsNames, hostName+zone)
		}
	}
	return &certv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: b.Namespace,
		},
		Spec: certv1.CertificateSpec{
			IssuerRef: certmetav1.ObjectReference{
				Kind: "ClusterIssuer",
				Name: TestIssuerName,
			},
			Subject: &certv1.X509Subject{
				Organizations:       []string{"YTsaurus dev"},
				OrganizationalUnits: []string{name},
			},
			CommonName:  dnsNames[0],
			SecretName:  name,
			DNSNames:    dnsNames,
			IPAddresses: b.IPAddresses,
			IsCA:        false,
			Usages: []certv1.KeyUsage{
				certv1.UsageServerAuth,
				certv1.UsageClientAuth,
			},
			PrivateKey: &certv1.CertificatePrivateKey{
				Algorithm: certv1.RSAKeyAlgorithm,
				Size:      2048,
			},
		},
	}
}
