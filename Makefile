COVERFILE=coverprofile

test-cover:
	go test -v ./... -coverprofile=${COVERFILE}
	go tool cover -html=${COVERFILE} && go tool cover -func ${COVERFILE} && unlink ${COVERFILE}
