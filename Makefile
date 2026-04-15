.PHONY: go ui test build clean

# Build the UI and copy its dist into webui/assets/ for embedding
ui:
	cd ui && npm install --silent && npm run build
	rm -rf webui/assets
	mkdir -p webui/assets
	cp -R ui/dist/. webui/assets/

test:
	go test ./...

build: ui go

go:
	go build ./...

clean:
	rm -rf ui/dist ui/node_modules webui/assets
	mkdir -p webui/assets
	@echo "Note: run 'make ui' to repopulate webui/assets before building."
