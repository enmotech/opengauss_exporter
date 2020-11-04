FROM golang:1.14 as builder
WORKDIR /go/src/opengauss_exporter
COPY . .
ENV GOPROXY=https://goproxy.cn GO111MODULE=on
RUN make build

# Distribution
FROM debian:10-slim
COPY --from=builder /go/src/opengauss_exporter/bin/opengauss_exporter /bin/opengauss_exporter
COPY og_exporter_default.yaml  /etc/og_exporter/
COPY script/docker-entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/docker-entrypoint.sh; ln -s /usr/local/bin/docker-entrypoint.sh / # backwards compat

ENTRYPOINT ["docker-entrypoint.sh"]
EXPOSE 9187
CMD [ "opengauss_exporter" ]