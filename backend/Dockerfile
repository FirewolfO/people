FROM golang:1.25-alpine AS build
WORKDIR /src
RUN apk add --no-cache build-base
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=1 go build -trimpath -ldflags='-s -w' -o /out/people-server ./cmd/server \
    && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/people-edge ./cmd/edge

FROM alpine:3.22
RUN addgroup -g 101 people && adduser -D -H -u 100 -G people people \
    && mkdir -p /data && chown people:people /data
COPY --from=build /out/people-server /usr/local/bin/people-server
COPY --from=build /out/people-edge /usr/local/bin/people-edge
USER people
CMD ["/usr/local/bin/people-server"]
