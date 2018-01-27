FROM alpine

COPY build/linux/kubist-agent /usr/local/bin

CMD ["kubist-agent"]