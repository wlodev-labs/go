.PHONY: openapi.bundle
openapi.bundle:
	@echo "Bundling OpenAPI specifications into a single file..."
	redocly bundle _oas/models.openapi.yml --output _oas/bundled/models-openapi-bundled.yml
	redocly bundle _oas/public.openapi.yml --output _oas/bundled/public-openapi-bundled.yml
	@echo "OpenAPI specifications bundled successfully."

.PHONY: openapi.generate
openapi.generate:
	@echo "Generating Go code from OpenAPI specifications..."
	go run scripts/openapi/generate/generate.go
	@echo "Go code generated successfully from OpenAPI specifications."

.PHONY: openapi
openapi: openapi.bundle openapi.generate
	@echo "OpenAPI tasks completed successfully."