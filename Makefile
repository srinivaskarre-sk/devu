INSTALL_DIR := $(HOME)/.local/bin
BIN_DIR     := bin

.PHONY: build install clean

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/devu .
	go build -o $(BIN_DIR)/gwt ./gwt

install: build
	cp $(BIN_DIR)/devu $(INSTALL_DIR)/devu
	cp $(BIN_DIR)/gwt  $(INSTALL_DIR)/gwt
	@echo "installed: $(INSTALL_DIR)/devu  $(INSTALL_DIR)/gwt"

clean:
	rm -rf $(BIN_DIR)
