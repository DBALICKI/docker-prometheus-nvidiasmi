FROM nvidia/cuda:11.4.3-base-ubuntu20.04

LABEL maintainer='Michaël "e7d" Ferrand <michael@e7d.io>'

RUN apt-get update && \
    apt-get -y install golang --no-install-recommends && \
    rm -r /var/lib/apt/lists/*

WORKDIR /go

RUN go mod download
RUN go build -v -o bin/prometheus-nvidia-smi src/app.go

EXPOSE 9202

CMD [ "./bin/prometheus-nvidia-smi" ]
