#!/bin/sh
set -e

/opt/keycloak/bin/kc.sh start-dev --import-realm &
kc_pid=$!

admin_user="${KC_BOOTSTRAP_ADMIN_USERNAME:-admin}"
admin_pass="${KC_BOOTSTRAP_ADMIN_PASSWORD:-admin}"

echo "Waiting for Keycloak admin API..."
ready=0
for _ in $(seq 1 90); do
  if /opt/keycloak/bin/kcadm.sh config credentials \
    --server "http://127.0.0.1:8080" \
    --realm master \
    --user "$admin_user" \
    --password "$admin_pass" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 2
done

if [ "$ready" -eq 1 ]; then
  /opt/keycloak/bin/kcadm.sh update realms/master -s sslRequired=NONE >/dev/null 2>&1 || true
  /opt/keycloak/bin/kcadm.sh update realms/nats-consol -s sslRequired=NONE >/dev/null 2>&1 || true
  echo "Keycloak dev realms configured for HTTP (sslRequired=NONE)"
else
  echo "Warning: could not configure Keycloak realms for HTTP"
fi

wait $kc_pid
