dependencies:
	cd ./static && npm install
	cd ../ && glide install
	go get github.com/GeertJohan/go.rice/rice

build:
	if [ -a ./static/dist ]; then rm -rf ./static/dist; fi;
	if [ -a stats ]; then rm stats; fi;
	cd ./static && npm run build
	rice embed-go
	go build
