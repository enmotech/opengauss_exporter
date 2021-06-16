systemctl daemon-reload >/dev/null 2>&1
systemctl start og_exporter >/dev/null 2>&1
systemctl enable og_exporter >/dev/null 2>&1