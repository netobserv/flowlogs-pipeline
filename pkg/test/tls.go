package test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Fake certificates / private key for test

const userCert string = `-----BEGIN CERTIFICATE-----
MIICuzCCAaMCCQCwxTs8nSaQAzANBgkqhkiG9w0BAQsFADAtMRMwEQYDVQQKDApp
by5zdHJpbXppMRYwFAYDVQQDDA1jbGllbnRzLWNhIHYwMB4XDTE5MDgyNjExMjgx
MVoXDTIwMDgyNTExMjgxMVowEjEQMA4GA1UEAwwHZm9vdXNlcjCCASIwDQYJKoZI
hvcNAQEBBQADggEPADCCAQoCggEBAMjkeJpZkNanRb3MjXwi7EQV0Sj2HbssT9Pr
p0L1Ha4aqEO3mgjBR5a2/iM+z5b1bF33S1GJiKkuz9Dq/BiFOmqBJSHqZQBGtFus
vbBFeV+pkS4OwAX68QRhzfD/rN/zhHcH9Ms1WGiV11KRMi15TPdgn1wP31qdq67u
VgQr2HlExQfzW5tEc5UpN6srRswnVxelPvXFpDsk5GORKvMhHshU8DJXx/vdHk5J
GT8KW+wi1nfmVTV79ZQR6ndmPR4PaoeOf/wawKews9N4jr17uQNvqQn488s+KNrT
O7eGd37x3tpPfAjqZmMKpt4sD/Z5MJs8ryakEQzAn3TKZnwo5CECAwEAATANBgkq
hkiG9w0BAQsFAAOCAQEAWNNALALbhsEaOpBz8J19FFKNh5JtUiBecQWNBnA3HXt5
X61dubaEwQMQqPZcIsdInkQ5n90OBb0dAY7A9wDlihmHc0GtcSo5NTMGELLTWH+g
GQO62OyUOUmkAgxhJZ/MDPzJSko9mUtiwXuovOt7EqyTvmvNjGBDwulcYMZ/VpQm
wxUFbfw4+mgVBH9lsWvAqNdflyXyEV1KxsHaApDeoG66sN2NSr09mGtADTrYskpM
qG9UQeYdb96qGmnkPWKlDKCl0s5ch9t3BZKhgNztmUtNUQiT7bH1whwIbyNA7/yo
XcB7AfxW+DNCSL51qnsOJoTG7N9gbfNhHpwlqHVuKA==
-----END CERTIFICATE-----`

const userKey string = `-----BEGIN PRIVATE KEY-----
MIIEvAIBADANBgkqhkiG9w0BAQEFAASCBKYwggSiAgEAAoIBAQDI5HiaWZDWp0W9
zI18IuxEFdEo9h27LE/T66dC9R2uGqhDt5oIwUeWtv4jPs+W9Wxd90tRiYipLs/Q
6vwYhTpqgSUh6mUARrRbrL2wRXlfqZEuDsAF+vEEYc3w/6zf84R3B/TLNVholddS
kTIteUz3YJ9cD99anauu7lYEK9h5RMUH81ubRHOVKTerK0bMJ1cXpT71xaQ7JORj
kSrzIR7IVPAyV8f73R5OSRk/ClvsItZ35lU1e/WUEep3Zj0eD2qHjn/8GsCnsLPT
eI69e7kDb6kJ+PPLPija0zu3hnd+8d7aT3wI6mZjCqbeLA/2eTCbPK8mpBEMwJ90
ymZ8KOQhAgMBAAECggEAegZNO3QsBjaUpjUZu816teCKq9bTOF4yHweFEabR4G9Q
xdFAPxEn6uQ8eiws7AUnTexoU5625A0LLluNxVcnpInNhExcDU7lPsoubmPE1dap
2NAc04UZ4Q+HiFvFJkNEswiiKMy+Zsidggmv8O89UQXfxovdn60mG5upo97+HqoS
YNxDtCuV1vbyr3n3kT3Hckqs6Rg2i1VTC12v17XfRiPgl13swoYbPh3Rb6x+nkvA
99PEO5941tC7WlpXU1q+t+keBnAbZ8MpErKHeAwwbqO10zxq5ZXQ/q8Po0hTPmD5
e/UDGeH6NEXoFfoIkFZ4i5Qn3ruxSu+uCO43tY7OAQKBgQD4WPgTOQhhhNm7bDPl
IAR9AOKUiqUrhjk+HgXS5Cmoe0pDOnlIrgtk0PFI+y6liHiSY0xgP7jYmXYYMhyu
CoLkzXSpto4iGB17Vf0Ad9CEH5IYv0GYf9e04wmGwSGAvpP6LRw1BpvkIaOERARC
c/cCB3SxW3ZRlEzhY+nbrqVkjwKBgQDPFSn3m/ZzLdyX/GpfIYDxprIAhDMzVVvQ
dCaJ7WLw+eIXmM4B+fE5dmQPj5n5aCAv4+byZD8SVybRK6FRC9GNA+2oR+2ImAWH
yJ+bKzOW4NrcFHoLOu6+viq/IkdkL/rBga+UShgNOZUMm/5y5X+nxelo4zujcNTr
YwKy6DlkTwKBgG1LalG7Zc7VEqWDJwuNHayNuSm6IpqXBZYqzFFVjGfTaolPsJSl
0+nYcne143+CId36yWAKayUX1HstgqWthpF/Qfp2lvK2PjNLUn7kO+YJptgxQ4MD
sECxMj4VvNLWDHWraKCFehHaJAZPkLhWJLzF3zs2j0mzxGnk+MRvheZNAoGAPgBS
Lmat5VJn58GVf6IiXzfPt8PdKJN4B/OezlEa/Jd0kCgaFhFlnhTKZLZUHY6FhJEx
xoUpNS6O2rW7eO6W8Sep8maGwgzyKvNwhh7rNVNhc99VoyMj9EwvtEZpJaAP7fwM
O9PrW5pP/BSAnJoGHI9vEQ5n/sl7lnZwimxpMpUCgYB5Q3UJLTdt23O/rFVz9HuX
ct2F3TRGA1WdHAH8XIn6lvgi6vGmwVSfRdcVu0uKIIQhQZOK9Bm8K8nn+gBzx8Qe
9n3zs6kNmZYy/V6DB/oYJ+NSEpB9hPZa+otmrrjIlY9q61jvNpnSlkfyBgeODZKW
dbCu3RRO6z6fiMUk49pbrQ==
-----END PRIVATE KEY-----`

const caCert string = `-----BEGIN CERTIFICATE-----
MIIDLTCCAhWgAwIBAgIJAOGqv8bs2vsHMA0GCSqGSIb3DQEBCwUAMC0xEzARBgNV
BAoMCmlvLnN0cmltemkxFjAUBgNVBAMMDWNsaWVudHMtY2EgdjAwHhcNMTkwODE5
MDU1MjI3WhcNMjAwODE4MDU1MjI3WjAtMRMwEQYDVQQKDAppby5zdHJpbXppMRYw
FAYDVQQDDA1jbGllbnRzLWNhIHYwMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIB
CgKCAQEA62RSy57QNyfIR16c4AzLxGl+HQjX6rGvLzuB6rJt/J4mnPwn0m5i5ubt
QyD1bsdd/hyO8V3yqg8moZ+MEgM6ckiJhlDvO1+/pbUdNwr29vKcBgB0yoTyDrx8
g2i5vpbbm5Q0cljbtrmD/UzBkTOLpCC+R/HtuwI6ea3iFN8J6u5TlmlFNVLhpzJv
uwwWvW29ryRDtvz60F+YApCv6bJwkL/ZtDkujVqBKCIFT+QFBt/35/EE/tZ7g5WL
kgPcr8d9oA/2LMk1gJIqUyQBL9dlFeKxSl4xLkK8n1xhjHfgn9vtasB2FtuaZ362
tfkWtARL7M17HVSEa5g9lOIHp2XYswIDAQABo1AwTjAdBgNVHQ4EFgQUqD+5Vwcr
2J2QMJQTxXb9XnB/i6AwHwYDVR0jBBgwFoAUqD+5Vwcr2J2QMJQTxXb9XnB/i6Aw
DAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAQEAEuI62DUpQfuI4uz87XlX
N5BbNEe5pqjS6VGE+JkO6SWjyu7S3V7I9Q1C0CfUsqn44xftPWco5T7TcBj5ATpS
GtLPa9B5C7WlP5VrNhyS55BZz2nU0lysXNY59b89cl2fVj+09vIpE9tdYmC/PolL
U1nFGo5xgZJbPVAhCMYyrVebDAKS/aViDSO2CbDlxNN5SUwIoMPd51in49z33vg7
W+BWYFyufGaDaB2R/BkWxmwPA+6j+uYyb8kMwk3qivVecDrBUuaJQZNfLIElOXWk
bK65qQDFSPn6bXowKNPbNsXwdIaJBdTLkEB98HzpMU+v7xwEQ/4JPyuNgeKJq86v
dA==
-----END CERTIFICATE-----`

// CreateCACert returns paths to CA cert and the cleanup function to defer
func CreateCACert(t *testing.T) (string, func()) {
	name, cleanup, err := DumpToTemp(caCert)
	require.NoError(t, err)
	return name, cleanup
}

// CreateAllCerts returns paths to:
// - ca
// - user cert
// - user key
// and the cleanup function to defer
func CreateAllCerts(t *testing.T) (string, string, string, func()) {
	ca, cleanupCA, err := DumpToTemp(caCert)
	require.NoError(t, err)
	uc, cleanupUC, err := DumpToTemp(userCert)
	require.NoError(t, err)
	uk, cleanupUK, err := DumpToTemp(userKey)
	require.NoError(t, err)
	return ca, uc, uk, func() {
		cleanupCA()
		cleanupUC()
		cleanupUK()
	}
}
