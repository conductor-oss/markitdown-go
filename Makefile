.PHONY: license-check license-add

ADDLICENSE := go run github.com/google/addlicense@latest
ADDLICENSE_FLAGS := -f licenseheader.txt \
	-ignore "markitdown-py/**" \
	-ignore ".idea/**" \
	-ignore "testdata/**" \
	-ignore "build/**" \
	-ignore ".claude/**"

# Check that all source files have the license header
license-check:
	@$(ADDLICENSE) -check $(ADDLICENSE_FLAGS) .

# Add license header to all source files that are missing it
license-add:
	@$(ADDLICENSE) -v $(ADDLICENSE_FLAGS) .
