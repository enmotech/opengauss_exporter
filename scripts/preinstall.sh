getent group prometheus >/dev/null || groupadd -r prometheus
getent passwd prometheus >/dev/null || \
  useradd -r -g prometheus -d /home/prometheus -s /sbin/nologin \
          -c "Prometheus services" prometheus
exit 0