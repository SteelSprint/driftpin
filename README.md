# Driftpin

Driftpin helps LLMs keep specs and code in sync.

## Quickstart

1. `make build`
2. Run `./drift init` in your project
3. Create spec files (`*.pin.xml`) and code markers (`#F <shortcode>`)
4. Run `./drift link <marker>:<spec>` to connect them
5. Run `./drift todo` to check for drift
6. Run `./drift reset <marker>:<spec>` to resolve drift

## Anatomy

- **Specs** — `*.pin.xml` files containing `<spec id="...">` elements
- **Markers** — `#F <shortcode>` comment lines in code files
- **drift.pin** — XML file at project root storing baseline hashes, links, and resolution state
- **CLI** — `drift init`, `drift todo`, `drift reset`, `drift link`

See [DOCUMENTATION.md](DOCUMENTATION.md) for the full documentation.
