FROM golang:1.23-bookworm

WORKDIR /Mythic/

COPY [".","."]

RUN make build

RUN apt-get install g++-x86-64-linux-gnu libc6-dev-amd64-cross -y

CMD make run