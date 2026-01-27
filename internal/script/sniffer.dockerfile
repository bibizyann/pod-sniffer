FROM alpine:3.18

# tcpdump
RUN apk add --no-cache tcpdump jq util-linux curl

# crictl
RUN curl -L https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.28.0/crictl-v1.28.0-linux-amd64.tar.gz | tar -xz -C /usr/local/bin

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x ./entrypoint.sh

ENTRYPOINT ["./entrypoint.sh"]