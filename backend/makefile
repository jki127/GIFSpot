GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get -u
BINARY_NAME=backend

build:
	$(GOBUILD) -o $(BINARY_NAME) -v
clean:
		$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)
