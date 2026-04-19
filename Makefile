BINARY := lazygithubactions

.PHONY: build run clean

build:
	go build -o $(BINARY) .

run: build
	./$(BINARY)

debug: build
	LAZYGH_DEBUG=1 ./$(BINARY)

clean:
	rm -f $(BINARY)
