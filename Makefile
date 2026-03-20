BUILD_TIME=`date +%FT%T%z`
LDFLAGS=-ldflags "-X main.Version=${V} -X main.BuildTime=${BUILD_TIME}"
SER=627proxy

build-docker:
	docker build --build-arg SERVICE=$(SER) -t scada/$(SER):dev .
	docker rmi $$(docker images --filter "dangling=true" -q --no-trunc)

push-docker:
	docker tag scada/$(SER):dev  asia-east1-docker.pkg.dev/muulin-universal/ml-scada/$(SER):$(V)
	docker push asia-east1-docker.pkg.dev/muulin-universal/ml-scada/$(SER):$(V)

release-win: build-win
	zip -j -X ~/Dropbox/squl/app/$(SER)/$(V).zip ./build/exe/$(SER).exe

build-win: clear-win
	GOOS=windows GOARCH=386 go build ${LDFLAGS} -o ./build/exe/$(SER).exe ./main.go


build: clear
	go build ${LDFLAGS} -o ./build/bin/$(SER) ./container/main.go
	./build/bin/$(SER) -v

run: build
	./build/bin/$(SER)
    
	

clear:
	rm -rf ./build/bin/$(SER)

clear-win:
	rm -rf ./build/exe/*

