export GOPROXY="https://proxy.golang.org,direct"
export GOPRIVATE="github.com/ioriver"
export GONOSUMDB="github.com/ioriver/*"
go mod init github.com/ioriver/terraform-provider-ioriver
go mod tidy
#go get github.com/ioriver/terraform-provider-ioriver/internal/provider
#go get github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs
#go get github.com/ioriver/ioriver-go
go mod vendor
#go get github.com/ioriver/ioriver-go
go build ./...
go test ./...