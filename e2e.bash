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
mkdir $d/www

cat > $d/server/config.yaml <<EOF
host: 127.0.0.1
domain-suffix: ""
listen: 127.0.0.1:8087
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
kish-url: ws://127.0.0.1:8087/
key: e2e/SSsecret
restriction:
  ip:
    - 127.0.0.1/32
EOF

cp /etc/services $d/www/services.txt

go build -o $d/kish-server cmd/kish-server/*.go
go build -o $d/kish cmd/kish/*.go

$d/kish-server --config $d/server/config.yaml &
python3 -m http.server 8080 --directory $d/www &

sleep 1

$d/kish --config $d/client/config.yaml --no-enable-tui --hostname localhost http 8080 &

sleep 1

curl http://localhost:8087/services.txt \
    --dump-header "$d/resp-header.txt" \
    --output "$d/resp-content.txt"

killkill

cat $d/resp-header.txt
#cat $d/resp-content.txt

errors=0
set -x
grep $'^X-Robots-Tag: none\r$' $d/resp-header.txt || errors=$(($errors+1))
#diff -u /etc/services $d/resp-content.txt || errors=$(($errors+1))
set +x
echo errors=$errors
if [[ $errors -eq 0 ]]; then
    exit 0
else
    exit 1
fi
