BINARY     := devu
INSTALL_DIR := $(HOME)/.local/bin

.PHONY: build install clean

build:
	go build -o $(BINARY) .

install: build
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "installed: $(INSTALL_DIR)/$(BINARY)"

clean:
	rm -f $(BINARY)
