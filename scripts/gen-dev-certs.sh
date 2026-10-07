#!/usr/bin/env bash
# Generate a self-signed ECDSA cert for local Tayga TLS (IMAPS/POP3S/SMTPS/HTTPS).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIR="${1:-$ROOT/data/certs}"
HOST="${2:-localhost}"
mkdir -p "$DIR"
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes \
  -keyout "$DIR/server.key" -out "$DIR/server.crt" -days 365 \
  -subj "/O=Tayga Mail Dev/CN=$HOST" \
  -addext "subjectAltName=DNS:$HOST,DNS:localhost,IP:127.0.0.1"
chmod 600 "$DIR/server.key"
echo "wrote $DIR/server.crt and $DIR/server.key"
