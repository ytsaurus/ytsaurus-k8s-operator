package ytconfig

import (
	"path"

	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

type Keyring struct {
	BusCABundle          *PemBlob
	BusClientCertificate *PemBlob
	BusClientPrivateKey  *PemBlob
	BusServerCertificate *PemBlob
	BusServerPrivateKey  *PemBlob
}

// Certificates from secrets mounted into servers and init-jobs
func getMountKeyring(commonSpec *ytv1.CommonSpec, transportSpec *ytv1.RPCTransportSpec) *Keyring {
	if transportSpec == nil {
		transportSpec = commonSpec.NativeTransport
	}
	if transportSpec == nil {
		transportSpec = &ytv1.RPCTransportSpec{}
	}

	var keyring Keyring

	if commonSpec.CABundle != nil {
		keyring.BusCABundle = &PemBlob{
			FileName: path.Join(consts.CABundleMountPoint, consts.CABundleFileName),
		}
	} else {
		keyring.BusCABundle = &PemBlob{
			FileName: consts.DefaultCABundlePath,
		}
	}

	if transportSpec.TLSSecret != nil {
		keyring.BusServerCertificate = &PemBlob{
			FileName: path.Join(consts.BusServerSecretMountPoint, corev1.TLSCertKey),
		}
		keyring.BusServerPrivateKey = &PemBlob{
			FileName: path.Join(consts.BusServerSecretMountPoint, corev1.TLSPrivateKeyKey),
		}
	}

	if transportSpec.TLSClientSecret != nil {
		keyring.BusClientCertificate = &PemBlob{
			FileName: path.Join(consts.BusClientSecretMountPoint, corev1.TLSCertKey),
		}
		keyring.BusClientPrivateKey = &PemBlob{
			FileName: path.Join(consts.BusClientSecretMountPoint, corev1.TLSPrivateKeyKey),
		}
	}

	return &keyring
}

// Certificates from secure vault environment variables with file fallback
func getVaultKeyring(commonSpec *ytv1.CommonSpec, transportSpec *ytv1.RPCTransportSpec) *Keyring {
	if transportSpec == nil {
		transportSpec = commonSpec.NativeTransport
	}
	if transportSpec == nil {
		transportSpec = &ytv1.RPCTransportSpec{}
	}

	var keyring Keyring

	if commonSpec.CABundle != nil {
		keyring.BusCABundle = &PemBlob{
			EnvironmentVariable: consts.SecureVaultEnvPrefix + consts.BusCABundleVaultName,
			FileName:            path.Join(consts.CABundleMountPoint, consts.CABundleFileName),
		}
	} else {
		keyring.BusCABundle = &PemBlob{
			FileName: consts.DefaultCABundlePath,
		}
	}

	if transportSpec.TLSSecret != nil {
		keyring.BusServerCertificate = &PemBlob{
			EnvironmentVariable: consts.SecureVaultEnvPrefix + consts.BusServerCertificateVaultName,
			FileName:            path.Join(consts.BusServerSecretMountPoint, corev1.TLSCertKey),
		}
		keyring.BusServerPrivateKey = &PemBlob{
			EnvironmentVariable: consts.SecureVaultEnvPrefix + consts.BusServerPrivateKeyVaultName,
			FileName:            path.Join(consts.BusServerSecretMountPoint, corev1.TLSPrivateKeyKey),
		}
	}

	if transportSpec.TLSClientSecret != nil {
		keyring.BusClientCertificate = &PemBlob{
			EnvironmentVariable: consts.SecureVaultEnvPrefix + consts.BusClientCertificateVaultName,
			FileName:            path.Join(consts.BusClientSecretMountPoint, corev1.TLSCertKey),
		}
		keyring.BusClientPrivateKey = &PemBlob{
			EnvironmentVariable: consts.SecureVaultEnvPrefix + consts.BusClientPrivateKeyVaultName,
			FileName:            path.Join(consts.BusClientSecretMountPoint, corev1.TLSPrivateKeyKey),
		}
	}

	return &keyring
}
