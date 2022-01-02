FROM golang:alpine as builder

WORKDIR /build

LABEL maintainer="Sebastián Chamena <sebachamena@gmail.com>"

COPY go.* ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./build ./server

RUN chmod +x ./build

FROM scratch

WORKDIR /bencher

COPY --from=builder /build/build /bin/bencher

ENTRYPOINT ["/bin/bencher"]