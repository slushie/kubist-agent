FROM golang:1.8

WORKDIR /go/src/github.com/slushie/kubist-agent

COPY . .

RUN make install

FROM alpine

COPY --from=0 /go/bin/kubist-agent /usr/local/bin

CMD ["kubist-agent"]
