FROM golang:1.14 as builder
WORKDIR /go/src/opengauss_exporter
COPY . .
ENV GOPROXY=https://goproxy.cn GO111MODULE=on
RUN make build

# Distribution
FROM debian:10-slim
COPY --from=builder /go/src/opengauss_exporter/bin/opengauss_exporter /bin/opengauss_exporter
COPY og_exporter.yaml  /etc/og_exporter/

EXPOSE 9187
CMD [ "opengauss_exporter" ]