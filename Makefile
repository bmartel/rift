dependencies:
	go get github.com/Masterminds/glide
	go get github.com/GeertJohan/go.rice/rice
	go get -u github.com/golang/protobuf/protoc-gen-go
	glide install

generate:
	protoc -I summary/ summary/summary.proto --go_out=plugins=grpc:summary

test:
	go test
