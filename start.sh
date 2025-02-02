export GOPROXY="https://proxy.golang.org,direct"
export GOPRIVATE="github.com/ioriver"
export GONOSUMDB="github.com/ioriver/*"
export GOBIN=$PWD/bin
export PATH=$GOBIN:$PATH
go mod init github.com/ioriver/terraform-provider-ioriver
go mod tidy
go mod vendor
go build ./...
#go test ./...
go generate ./...