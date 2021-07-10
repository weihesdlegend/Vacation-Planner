FROM golang:alpine as unwindtest

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOFLAGS=-mod=mod

COPY . /app/
WORKDIR /app/

RUN ls -al

EXPOSE 8010
RUN go install -v github.com/weihesdlegend/Vacation-planner
RUN go build -v .
CMD ["/go/bin/Vacation-planner"]
