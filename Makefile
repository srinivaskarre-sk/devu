INSTALL_DIR := $(HOME)/.local/bin

.PHONY: build install clean

build:
	go build -o devu .
	go build -o gwt ./gwt

install: build
	cp devu $(INSTALL_DIR)/devu
	cp gwt  $(INSTALL_DIR)/gwt
	@echo "installed: $(INSTALL_DIR)/devu  $(INSTALL_DIR)/gwt"

clean:
	rm -f devu gwt
