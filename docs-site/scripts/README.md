# Documentation Scripts for Zensical and GitHub Pages

Shell tooling that stages `.web-docs` for [Zensical](https://zensical.org) and publishes versioned
documentation with [`mike`](https://github.com/squidfunk/mike) to GitHub Pages.

## Entry Points

| Script            | Purpose                                                                                      |
|-------------------|----------------------------------------------------------------------------------------------|
| `prepare-docs.sh` | Stage `.web-docs` and `extra/` into `.build/docs`, rewrite links, emit `zensical.build.toml` |
| `mike-deploy.sh`  | Deploy one release version to `gh-pages`                                                     |
| `mike-backfill.sh` | Deploy multiple historical versions (interactive)                                            |
| `mike-preview.sh` | Local multi-version preview (no push)                                                        |

Each entry-point script supports `--help` for usage, options, environment variables, and examples:

```bash
./docs-site/scripts/prepare-docs.sh --help
./docs-site/scripts/mike-deploy.sh --help
./docs-site/scripts/mike-backfill.sh --help
./docs-site/scripts/mike-preview.sh --help
```

`lib/` holds sourced helpers. `test/` holds unit tests — run `test/test-all.sh` or `make docs-test`.

## What `prepare-docs.sh` does

1. Transform each component README through `lib/stage-markdown.sh`.
2. Stage the home page and inject `extra/intro-upper.md`, `extra/intro-lower.md`, and a data sources section when that release includes data sources.
3. Copy `extra/` community pages and section index files.
4. Copy site assets (`stylesheets/extra.css`, `javascripts/*.js`) into the staging tree.
5. Generate navigation and write `zensical.build.toml`.

Versioned deploys use `INCLUDE_EXTRA=true` so every release shares the same site shell.

Component reference content still comes from that tag’s `.web-docs`.

## Common Commands

Use the `./Makefile` targets.

```bash
make docs-prepare          # stage only
make docs-test             # all script tests
make docs-build            # generate + stage + zensical build
make docs-serve            # generate + stage + local serve

make docs-serve-version VERSION=2.1.0            # preview a single release
make docs-serve-mike                             # local mike preview (tags + development)
make docs-backfill VERSIONS="2.0.0 2.1.0 2.2.0"   # push versions to gh-pages
```

Stable release tags trigger `deploy-docs` in `.github/workflows/release.yml` after GoReleaser succeeds.

## Environment variables

| Variable              | Default            | Notes                                             |
|-----------------------|--------------------|---------------------------------------------------|
| `INCLUDE_EXTRA`       | `true`             | Community pages, home injections, section indexes |
| `WEB_DOCS_DIR`        | `<repo>/.web-docs` | Source component READMEs                          |
| `MIKE_PREVIEW_BRANCH` | `docs-preview`     | Local preview git branch                          |
| `MIKE_PREVIEW_PORT`   | `8001`             | Local preview URL port                            |
| `MIKE_PREVIEW_SKIP_DEVELOPMENT` | `false`  | Omit working-tree version from preview deploys    |
| `MIKE_BRANCH`         | `gh-pages`         | Remote pages branch                               |
| `MIKE_COMMIT_MESSAGE` | unset              | Optional mike git commit message (`{version}`, `{branch}`) |
| `MIKE_COMMIT_VERSION` | set by deploy scripts | Value substituted for `{version}` in commit messages |

## Tests

```bash
./docs-site/scripts/test/test-all.sh
```

Most `lib` scripts have a matching `test/test-<name>.sh`.
