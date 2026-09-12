APP_NAME = folio
SRC = cmd/main.go
BIN_DIR = bin
EXEC = $(BIN_DIR)/$(APP_NAME)

.PHONY: all
all: build

.PHONY: build
build:
	@mkdir -p $(BIN_DIR)
	go build -o $(EXEC) $(SRC)

.PHONY: run
run: build
	./$(EXEC)

.PHONY: test
test:
	go test ./...

.PHONY: test-integration
test-integration:
	go test -tags=integration ./...

.PHONY: clean
clean:
	rm -rf $(BIN_DIR) generated

.PHONY: deps
deps:
	go mod tidy
