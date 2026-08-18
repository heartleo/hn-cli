# DSH Plugin Migration Design

## Goal

Migrate the DeepSeek Harness Hacker News plugin from `../hn` into this repository while preserving its behavior and adapting repository-specific links to `heartleo/hn-cli`.

## Scope

- Keep the plugin as an independent ESM package under `plugins/hacker-news`.
- Preserve the source plugin's four tools: `hn_stories`, `hn_item`, `hn_search`, and `hn_user`.
- Keep the plugin independent of the Go CLI; it calls the public Hacker News Firebase and Algolia APIs directly.
- Update package metadata, installation commands, and source-checkout examples to reference `heartleo/hn-cli`.
- Document the plugin in the root README and ignore local Node dependency directories.
- Add repeatable validation locally and in GitHub Actions.

Refactoring the migrated plugin or redesigning its tool interfaces is outside this migration.

## Repository Layout

`plugins/hacker-news` remains a self-contained Node package:

- `index.js` exports the Cordis plugin entry point and configuration.
- `src/client.js` owns Firebase and Algolia requests.
- `src/config.js` defines deployment configuration and defaults.
- `src/text.js` converts Hacker News HTML and renders model-facing text.
- `src/tools.js` registers and implements the four model tools.
- `cordis.patch.yml` supplies the DSH bundle patch.
- `load-check.mjs` exercises plugin loading, registration, schemas, normal calls, and rejected inputs.

The repository-level integration consists of the root README, `.gitignore`, and the existing plugin workflow.

## Validation Design

The plugin package will expose a documented validation command. Validation will install the package dependencies, load the ESM entry point, register all four tools, validate representative results against each output schema, exercise the public Hacker News APIs, and confirm representative invalid inputs are rejected.

The existing `.github/workflows/plugin.yml` workflow will run this validation whenever files under `plugins/**` change. The workflow will continue validating the existing Claude plugin manifests as well.

Because the current load check reaches public APIs, it is an integration check. No unit-test refactor or HTTP mocking is added as part of this migration.

## Error Handling

The migrated implementation retains the source plugin's error behavior for invalid configuration, invalid tool arguments, missing Hacker News objects, HTTP failures, timeouts, and cancellation. The migration does not introduce fallback behavior that could hide upstream failures.

## Acceptance Criteria

- Every source plugin file is present in `plugins/hacker-news`.
- Runtime source files match `../hn` except for the repository URL in the default user agent; documentation and package metadata use the new repository as well.
- The root documentation installs from `github:heartleo/hn-cli#path:/plugins/hacker-news`.
- The plugin loads under its declared Node version and registers exactly the four documented tools.
- The load check completes successfully and validates representative outputs and errors.
- GitHub Actions invokes the same plugin validation on relevant changes.
- Existing Go and Claude-plugin validation remains passing.
