# github.com/wlodev-labs/go

The shared Go library (`postgres`, `pagination`, `ptr`, `publicid`, `zerolog`) for wlodev services. It also ships the `wlodev-go` skill under `skills/`, which projects install with `npx skills add`.

## Code style
Go code in this repo follows the same style as the projects:

@skills/wlodev-go/references/style.md

Library-specific additions:
- ✅ Every exported identifier has a doc comment, because it's the package's godoc. It starts with the identifier's name and describes the contract: inputs, outputs, errors, edge cases (nil or empty input). Don't describe the implementation.
- ❌ Don't rename or remove an exported API in a minor release. Add the new name next to the old one.
- Run `go build ./... && go vet ./...` before you finish.

## Changing the lib API
- Update `skills/wlodev-go/references/lib.md` and the minimum version line in `skills/wlodev-go/SKILL.md` in the same change.
- Release with `make publish VERSION=vX.Y.Z` only when the user asks. It tags the release and pushes it.

## Changing the skill
- Follow "Changing the conventions" in `skills/README.md`.
- Example Go files use the `.go.txt` extension so they never compile into this module.
- The skill describes *service* projects (`_oas/`, handlers, migrations). Those rules don't apply to this repo's own code, apart from the style rules imported above.
