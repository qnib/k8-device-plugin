FROM ubuntu:16.04 as build

RUN apt-get update \
 && apt-get install -y --no-install-recommends \
        g++ \
        ca-certificates \
        wget \
        libusb-1.0.0-dev \
        pkg-config \
 && rm -rf /var/lib/apt/lists/*

ENV GOLANG_VERSION 1.10.3
RUN wget -nv -O - https://storage.googleapis.com/golang/go${GOLANG_VERSION}.linux-amd64.tar.gz \
    | tar -C /usr/local -xz
ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH

WORKDIR /go/src/github.com/qnib/k8s-device-plugin
COPY . .

RUN export CGO_LDFLAGS_ALLOW='-Wl,--unresolved-symbols=ignore-in-object-files' && \
    go install -ldflags="-s -w"


FROM debian:stretch-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends libusb-1.0.0-dev
ENV NVIDIA_VISIBLE_DEVICES=all
ENV NVIDIA_DRIVER_CAPABILITIES=utility

COPY --from=build /go/bin/k8s-device-plugin /usr/bin/qnib-device-plugin

CMD ["qnib-device-plugin"]
