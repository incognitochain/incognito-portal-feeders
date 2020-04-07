FROM golang:1.13.1

LABEL maintainer="Hoang Nguyen Gia <hoang@incognito.org>"

WORKDIR /go/src/app
COPY go.mod .
RUN go mod download

COPY . .

RUN go build -o portalfeeders

CMD ["./portalfeeders"]