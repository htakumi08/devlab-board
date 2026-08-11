FROM golang:1.26.5-alpine

# gRPC用protobufをインストール
RUN apk add --no-cache protobuf

# 再現可能なbuildにするため、gRPCコード生成・動作確認ツールを安定版へ固定する
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11 \
    && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2 \
    && go install github.com/fullstorydev/grpcurl/cmd/grpcurl@v1.9.3

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

# Goアプリを直接PID 1で実行し、DockerのSIGTERMをGraceful Shutdown処理へ届ける。
RUN go build -o /usr/local/bin/devlab-board-api ./cmd/api

EXPOSE 8080 50051

CMD ["/usr/local/bin/devlab-board-api"]
