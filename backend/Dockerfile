FROM golang:1.26.5-alpine

# gRPC用protobufをインストール
RUN apk add --no-cache protobuf

# 再現可能なbuildにするため、gRPCコード生成プラグインを安定版へ固定する
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11 \
    && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2

WORKDIR /app

COPY go.mod ./

RUN go mod download

COPY . .

EXPOSE 8080

CMD ["go", "run", "./cmd/api"]
