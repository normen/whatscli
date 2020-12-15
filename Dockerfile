FROM golang

WORKDIR /src

COPY . .

RUN make build

ENTRYPOINT ["./whatscli"]

