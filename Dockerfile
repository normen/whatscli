FROM golang as builder

WORKDIR /src

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build

FROM alpine
COPY --from=builder /src/whatscli /usr/local/bin/

ENTRYPOINT ["whatscli"]

