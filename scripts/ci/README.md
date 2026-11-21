# Registry trust used by package CI

`redos-registry-ca.pem` is used only in the disposable runner's Docker trust directory for `registry.red-soft.ru`. It is not installed by Tayga packages and does not change workstation or OS-wide CA trust. TLS verification remains enabled.

Source: the `ca-certificates-digital.gov.ru` package from the official ALT p11 repository, obtained in the official `alt:p11` container. It contains the Russian Trusted Root CA plus the 2024 and original Russian Trusted Sub CA certificates. Both intermediates were verified against the root with `openssl verify`. The RED OS registry currently sends an older intermediate which does not validate its renewed leaf; the 2024 intermediate completes the verified chain.

Root SHA-256 fingerprint: `D2:6D:2D:02:31:B7:C3:9F:92:CC:73:85:12:BA:54:10:35:19:E4:40:5D:68:B5:BD:70:3E:97:88:CA:8E:CF:31`. The workflow pins the SHA-256 digest of the complete PEM bundle. Updating it requires checking the issuing chain again.

Official documentation: https://www.gosuslugi.ru/crt and https://redos.red-soft.ru/base/redos-7_3/7_3-security/7_3-ssl/7_3-russian-ssl-sert/ .
