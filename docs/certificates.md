# Client Certificates

Mobilithek machine-account access can require mutual TLS. Mobilithek typically provides a PKCS#12 bundle (`.p12` or `.pfx`) and a password.

The Go standard library loads client certificates as separate PEM files:

- certificate: `client.crt`
- private key: `client.key`

## Convert PKCS#12 To PEM

```bash
mkdir -p certs
openssl pkcs12 -in certs/client.p12 -clcerts -nokeys -out certs/client.crt
openssl pkcs12 -in certs/client.p12 -nocerts -nodes -out certs/client.key
chmod 600 certs/client.key
```

If OpenSSL asks for an import password, use the password that belongs to the `.p12` file.

## Use The Certificate

```go
client, err := mobilithek.New(
	mobilithek.WithClientCertificate("certs/client.crt", "certs/client.key"),
)
```

## Optional Root CA

Most production HTTPS endpoints should validate through the operating system trust store. Only pass `WithRootCA` if Mobilithek or a test system explicitly gives you an additional CA certificate:

```go
client, err := mobilithek.New(
	mobilithek.WithClientCertificate("certs/client.crt", "certs/client.key"),
	mobilithek.WithRootCA("certs/ca.pem"),
)
```

## Do Not Commit

Never commit:

- `.p12` / `.pfx`
- private keys
- certificate passwords
- `.env` files with credential paths
- production response data
