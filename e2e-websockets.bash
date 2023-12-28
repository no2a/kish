#!/bin/bash

set -eu
set -o pipefail
set -o posix

killkill() {
    pkill -P $$
}

d=$(mktemp -d)
trap 'killkill || true; rm -r "$d"' EXIT

mkdir $d/server
mkdir $d/client

cat > $d/server/config.yaml <<EOF
host: 127.0.0.1
domain-suffix: ""
listen: 127.0.0.1:18087
trust-x-forwarded-for: false
enable-tcp-forwading: false
account: $d/server/account.yaml
tls-cert: ""
tls-key: ""
EOF

cat > $d/server/account.yaml <<EOF
e2e: SSsecret
EOF

cat > $d/client/config.yaml <<EOF
kish-url: ws://127.0.0.1:18087/
key: e2e/SSsecret
restriction:
  ip:
    - 127.0.0.1/32
EOF

go build -o $d/kish-server cmd/kish-server/*.go
go build -o $d/kish cmd/kish/*.go

$d/kish-server --config $d/server/config.yaml &
python e2e-websockets/echo.py 18765 &

sleep 1

$d/kish --config=$d/client/config.yaml --no-enable-tui --hostname=localhost http 18765 &

sleep 1

output=$(python e2e-websockets/hello.py ws://localhost:18765)

killkill

errors=0
set -x
[[ $output = 'Received: Hello world!' ]] || errors=$(($errors+1))
echo errors=$errors
if [[ $errors -eq 0 ]]; then
    exit 0
else
    exit 1
fi
