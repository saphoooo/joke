FROM golang:1.17

WORKDIR /

COPY . .

ADD https://github.com/upx/upx/releases/download/v3.96/upx-3.96-amd64_linux.tar.xz /usr/local

RUN set -x && \
    apt update && \
    apt install -y xz-utils && \
    xz -d -c /usr/local/upx-3.96-amd64_linux.tar.xz | \
    tar -xOf - upx-3.96-amd64_linux/upx > /bin/upx && \
    chmod a+x /bin/upx && \
    go get -d -v . && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o jokefront . && \
    strip --strip-unneeded jokefront && \
    upx jokefront

FROM scratch

LABEL maintainer="stephane.beuret@gmail.com"

COPY --from=0 jokefront /

ENTRYPOINT ["/jokefront"]