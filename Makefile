dependencies:
	go get github.com/Masterminds/glide
	go get github.com/GeertJohan/go.rice/rice
	glide install
	go install ./vendor/github.com/golang/protobuf/protoc-gen-go/
	rm -rf vendor/golang.org/x/net/context/

generate:
	protoc --go_out=plugins=grpc:. summary/*.proto

test:
	go test
