eval $(docker-machine env dev)
docker run -v "$(pwd)":/go/src/github.com/pyama86/libnss_stns -w /go/src/github.com/pyama86/libnss_stns centos:libnss go test
