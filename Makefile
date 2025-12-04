APP_NAME := codecurfew
CMD_DIR := ./cmd/code-curfew
BIN_DIR := ./bin

.PHONY: run build clean

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(APP_NAME) $(CMD_DIR)

run:
	go run $(CMD_DIR)

clean:
	rm -rf $(BIN_DIR)
