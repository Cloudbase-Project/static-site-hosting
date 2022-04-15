BINARY_NAME=main.out
 
build:
	go build -o ${BINARY_NAME} main.go
 
run:
	go build -o ${BINARY_NAME} main.go
	./${BINARY_NAME}
 
clean:
	go clean
	rm ${BINARY_NAME}

k8s-run:
	skaffold run

k8s-dev:
	skaffold dev