# Evidence pack — `pocketbase/pocketbase`

> Auto-generated raw material for the hand-made gold-standard preview (G13).
> It reproduces what the ingestion client sees; it makes **no claims**. Write the preview against this, citing the files below.

_Generated 2026-10-24 via `cmd/evidencepack`._

## Metadata

- **Full name:** pocketbase/pocketbase
- **Description:** Open Source realtime backend in 1 file
- **Primary language:** Go
- **Stars:** 61578
- **License:** MIT
- **Default branch:** master
- **Last pushed:** 2026-10-23T12:26:56Z
- **Archived:** false · **Fork:** false
- **Topics:** authentication, backend, golang, realtime
- **URL:** https://github.com/pocketbase/pocketbase

## Structure (top 2 levels)

```
.github/
  SECURITY.md
  workflows/
.gitignore
.goreleaser.yaml
CHANGELOG.md
CHANGELOG_08_15.md
CHANGELOG_16_22.md
CHANGELOG_23_39.md
CONTRIBUTING.md
LICENSE.md
Makefile
README.md
apis/
  api_error_aliases.go
  backup.go
  backup_create.go
  backup_test.go
  backup_upload.go
  base.go
  base_test.go
  batch.go
  batch_test.go
  collection.go
  collection_import.go
  collection_import_test.go
  collection_test.go
  cron.go
  cron_test.go
  extensions.go
  extensions_test.go
  file.go
  file_test.go
  health.go
  health_test.go
  installer.go
  logs.go
  logs_test.go
  middlewares.go
  middlewares_body_limit.go
  middlewares_body_limit_test.go
  middlewares_cors.go
  middlewares_cors_test.go
  middlewares_gzip.go
  middlewares_rate_limit.go
  middlewares_rate_limit_test.go
  middlewares_test.go
  realtime.go
  realtime_test.go
  record_auth.go
  record_auth_email_change_confirm.go
  record_auth_email_change_confirm_test.go
  record_auth_email_change_request.go
  record_auth_email_change_request_test.go
  record_auth_impersonate.go
  record_auth_impersonate_test.go
  record_auth_methods.go
  record_auth_methods_test.go
  record_auth_otp_request.go
  record_auth_otp_request_test.go
  record_auth_password_reset_confirm.go
  record_auth_password_reset_confirm_test.go
  record_auth_password_reset_request.go
  record_auth_password_reset_request_test.go
  record_auth_refresh.go
  record_auth_refresh_test.go
  record_auth_verification_confirm.go
  record_auth_verification_confirm_test.go
  record_auth_verification_request.go
  record_auth_verification_request_test.go
  record_auth_with_oauth2.go
  record_auth_with_oauth2_redirect.go
  record_auth_with_oauth2_redirect_test.go
  record_auth_with_oauth2_test.go
  record_auth_with_otp.go
  record_auth_with_otp_test.go
  record_auth_with_password.go
  record_auth_with_password_test.go
  record_crud.go
  record_crud_auth_origin_test.go
  record_crud_external_auth_test.go
  record_crud_mfa_test.go
  record_crud_otp_test.go
  record_crud_superuser_test.go
  record_crud_test.go
  record_helpers.go
  record_helpers_test.go
  serve.go
  settings.go
  settings_test.go
  sql.go
  sql_test.go
cmd/
  serve.go
  superuser.go
  superuser_test.go
core/
  app.go
  auth_origin_model.go
  auth_origin_model_test.go
  auth_origin_query.go
  auth_origin_query_test.go
  backup.go
  backup_create.go
  backup_restore.go
  backup_test.go
  base.go
  base_test.go
  collection_import.go
  collection_import_test.go
  collection_model.go
  collection_model_auth_options.go
  collection_model_auth_options_test.go
  collection_model_auth_templates.go
  collection_model_base_options.go
  collection_model_test.go
  collection_model_view_options.go
  collection_model_view_options_test.go
  collection_query.go
  collection_query_test.go
  collection_record_table_sync.go
  collection_record_table_sync_test.go
  collection_validate.go
  collection_validate_test.go
  db.go
  db_builder.go
  db_connect.go
  db_connect_nodefaultdriver.go
  db_model.go
  db_model_test.go
  db_retry.go
  db_retry_test.go
  db_table.go
  db_table_test.go
  db_test.go
  db_tx.go
  db_tx_test.go
  event_request.go
  event_request_batch.go
  event_request_batch_test.go
  event_request_test.go
  events.go
  external_auth_model.go
  external_auth_model_test.go
  external_auth_query.go
  external_auth_query_test.go
  field.go
  field_autodate.go
  field_autodate_test.go
  field_bool.go
  field_bool_test.go
  field_date.go
  field_date_test.go
  field_editor.go
  field_editor_test.go
  field_email.go
  field_email_test.go
  field_file.go
  field_file_test.go
  field_geo_point.go
  field_geo_point_test.go
  field_json.go
  field_json_test.go
  field_number.go
  field_number_test.go
  field_password.go
  field_password_test.go
  field_relation.go
  field_relation_test.go
  field_select.go
  field_select_test.go
  field_test.go
  field_text.go
  field_text_test.go
  field_url.go
  field_url_test.go
  fields_list.go
  fields_list_test.go
  log_model.go
  log_model_test.go
  log_printer.go
  log_printer_test.go
  log_query.go
  log_query_test.go
  mfa_model.go
  mfa_model_test.go
  mfa_query.go
  mfa_query_test.go
  migrations_list.go
  migrations_list_test.go
  migrations_runner.go
  migrations_runner_test.go
  notify_watcher.go
  notify_watcher_test.go
  otp_model.go
  otp_model_test.go
  otp_query.go
  otp_query_test.go
  record_field_resolver.go
  record_field_resolver_replace_expr.go
  record_field_resolver_runner.go
  record_field_resolver_test.go
  record_model.go
  record_model_auth.go
  record_model_auth_test.go
  record_model_superusers.go
  record_model_superusers_test.go
  record_model_test.go
  record_proxy.go
  record_proxy_test.go
  record_query.go
  record_query_expand.go
  record_query_expand_test.go
  record_query_test.go
  record_tokens.go
  record_tokens_test.go
  settings_model.go
  settings_model_test.go
  settings_query.go
  settings_query_test.go
  syscall.go
  syscall_wasm.go
  system_alert.go
  system_alert_test.go
  validators/
  view.go
  view_test.go
examples/
  base/
forms/
  apple_client_secret_create.go
  apple_client_secret_create_test.go
  record_upsert.go
  record_upsert_test.go
  test_email_send.go
  test_email_send_test.go
  test_s3_filesystem.go
  test_s3_filesystem_test.go
go.mod
go.sum
golangci.yml
mails/
  base.go
  record.go
  record_test.go
  templates/
migrations/
  1640988000_aux_init.go
  1640988000_init.go
  1717233556_v0.23_migrate.go
  1717233557_v0.23_migrate2.go
  1717233558_v0.23_migrate3.go
  1717233559_v0.23_migrate4.go
  1763020353_update_default_auth_alert_templates.go
  1778828400_normalize_indexes.go
modernc_versions_check.go
plugins/
  ghupdate/
  jsvm/
  migratecmd/
  update/
pocketbase.go
pocketbase_test.go
tests/
  api.go
  app.go
  data/
  dynamic_stubs.go
  mailer.go
  request.go
  validation_errors.go
tools/
  archive/
  auth/
  cron/
  dbutils/
  filesystem/
  hook/
  inflector/
  list/
  logger/
  mailer/
  osutils/
  picker/
  router/
  routine/
  search/
  security/
  store/
  subscriptions/
  template/
  tokenizer/
  types/
ui/
  .env
  .env.development
  .gitignore
  README.md
  dist/
  dprint-typescript-0.95.15.wasm
  dprint.json
  embed.go
  embed_no_ui.go
  index.html
  package-lock.json
  package.json
  public/
  src/
  vite.config.js
```

_Total entries ingested: 1107._

## Key files

Selected 14 candidate key files; 4 fetched, 10 skipped.

### `go.mod`

```
module github.com/pocketbase/pocketbase

go 1.27

require (
	github.com/disintegration/imaging v1.6.2
	github.com/domodwyer/mailyak/v3 v3.6.2
	github.com/dop251/goja v0.0.0-20261021140635-64adc8bb4d51
	github.com/dop251/goja_nodejs v0.0.0-20260212111938-1f56ff5bcf14
	github.com/fatih/color v1.19.0
	github.com/fsnotify/fsnotify v1.10.1
	github.com/gabriel-vasile/mimetype v1.4.15
	github.com/ganigeorgiev/fexpr v0.6.0
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/pocketbase/dbx v1.12.0
	github.com/pocketbase/ozzo-validation/v4 v4.3.0
	github.com/pocketbase/tygoja v0.1.0
	github.com/spf13/cast v1.10.0
	github.com/spf13/cobra v1.10.2
	golang.org/x/crypto v0.58.0
	golang.org/x/image v0.47.0
	golang.org/x/net v0.61.0
	golang.org/x/oauth2 v0.37.0
	golang.org/x/sync v0.24.0
	modernc.org/sqlite v1.60.1
)

require (
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/dlclark/regexp2/v2 v2.9.1 // indirect
	github.com/dop251/base64dec v0.0.0-20231022112746-c6c9f9a96217 // indirect
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/go-sourcemap/sourcemap v2.1.4+incompatible // indirect
	github.com/google/pprof v0.0.0-20261016150608-1facf8f91b50 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/mattn/go-colorable v0.1.16 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.1.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/stretchr/testify v1.8.0 // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	golang.org/x/tools v0.52.0 // indirect
	modernc.org/libc v1.77.1 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
)
```

### `ui/package.json`

```json
{
    "name": "PocketBase Superuser UI",
    "private": true,
    "type": "module",
    "scripts": {
        "dev": "vite",
        "build": "dprint fmt && vite build"
    },
    "dependencies": {
        "leaflet": "^1.9.4",
        "pocketbase": "^0.28.1"
    },
    "devDependencies": {
        "dprint": "^0.52.0",
        "vite": "^8.0.0"
    }
}
```

### `CONTRIBUTING.md`

```markdown
# Contributing to PocketBase

> [!IMPORTANT]
> Due to recent LLM spam, PRs are temporary disabled and only existing collaborators can open a PR.
> If you stumble on a problem that you want to fix, please consider instead opening an issue or discussion with link to your fork _(if not obvious - LLM contributions are not welcome)_.
> This status may change in the future in case GitHub finally decide to do something about the constant spam, or when I find time to move the project somewhere else.

This document describes how to prepare a PR for a change in the main repository.

- [Prerequisites](#prerequisites)
- [Making changes in the Go code](#making-changes-in-the-go-code)
- [Making changes in the Superuser UI](#making-changes-in-the-superuser-ui)

## Prerequisites

- Go 1.27+ (for making changes in the Go code)
- Node 24+ (for making changes in the Superuser UI)

If you haven't already, you can fork the main repository and clone your fork so that you can work locally:

```
git clone https://github.com/your_username/pocketbase.git
```

> [!IMPORTANT]
> It is recommended to create a new branch from master for each of your bugfixes and features.
> This is required if you are planning to submit multiple PRs in order to keep the changes separate for review until they eventually get merged.

## Making changes in the Go code

PocketBase is distributed as a Go package, which means that in order to run the project you'll have to create a Go `main` program that imports the package.

The repository already includes such program, located in `examples/base`, that is also used for the prebuilt executables.

So, let's assume that you already done some changes in the PocketBase Go code and you want now to run them:

1. Navigate to `examples/base`
2. Run `go run main.go serve`

This will start a web server on `http://localhost:8090` with the embedded prebuilt Superuser UI from `ui/dist`. And that's it!

**Before making a PR to the main repository, it is a good idea to:**

- Add unit/integration tests for your changes (we are using the standard `testing` go package).
  To run the tests, you could execute (while in the root project directory):

  ```sh
  go test ./...

  # or using the Makefile
  make test
  ```

- Run the linter - **golangci** ([see how to install](https://golangci-lint.run/usage/install/#local-installation)):

  ```sh
  golangci-lint run -c ./golangci.yml ./...

  # or using the Makefile
  make lint
  ```

## Making changes in the Superuser UI

PocketBase Superuser UI is a single-page application (SPA) built with Svelte and Vite.

To start the Superuser UI:

1. Navigate to the `ui` project directory
2. Run `npm install` to install the node dependencies
3. Start vite's dev server
   ```sh
   npm run dev
   ```

You could open the browser and access the running Superuser UI at `http://localhost:5173`.

Since the Superuser UI is just a client-side application, you need to have the PocketBase backend server also running in the background (either manually running the `examples/base/main.go` or download a prebuilt executable).

> [!NOTE]
> By default, the Superuser UI is expecting the backend server to be started at `http://localhost:8090`, but you could change that by creating a new `ui/.env.development.local` file with `PB_BACKEND_URL = YOUR_ADDRESS` variable inside it.

Every change you make in the Superuser UI should be automatically reflected in the browser at `http://localhost:5173` without reloading the page.

Once you are done with your changes, you have to build the Superuser UI with `npm run build`, so that it can be embedded in the go package. And that's it - you can make your PR to the main PocketBase repository.
```

### `README.md`

```markdown
<p align="center">
    <a href="https://pocketbase.io" target="_blank" rel="noopener">
        <img src="https://i.imgur.com/aCBbjKx.png" alt="PocketBase - open source backend in 1 file" />
    </a>
</p>

<p align="center">
    <a href="https://github.com/pocketbase/pocketbase/actions/workflows/release.yaml" target="_blank" rel="noopener"><img src="https://github.com/pocketbase/pocketbase/actions/workflows/release.yaml/badge.svg" alt="build" /></a>
    <a href="https://github.com/pocketbase/pocketbase/releases" target="_blank" rel="noopener"><img src="https://img.shields.io/github/release/pocketbase/pocketbase.svg" alt="Latest releases" /></a>
    <a href="https://pkg.go.dev/github.com/pocketbase/pocketbase" target="_blank" rel="noopener"><img src="https://godoc.org/github.com/pocketbase/pocketbase?status.svg" alt="Go package documentation" /></a>
</p>

[PocketBase](https://pocketbase.io) is an open source Go backend that includes:

- embedded database (_SQLite_) with **realtime subscriptions**
- built-in **files and users management**
- convenient **Admin dashboard UI**
- and simple **REST-ish API**

**For documentation and examples, please visit https://pocketbase.io/docs.**

> [!WARNING]
> Please keep in mind that PocketBase is still under active development
> and therefore full backward compatibility is not guaranteed before reaching v1.0.0.

## API SDK clients

The easiest way to interact with the PocketBase Web APIs is to use one of the official SDK clients:

- **JavaScript - [pocketbase/js-sdk](https://github.com/pocketbase/js-sdk)** (_Browser, Node.js, React Native_)
- **Dart - [pocketbase/dart-sdk](https://github.com/pocketbase/dart-sdk)** (_Web, Mobile, Desktop, CLI_)

You could also check the recommendations in https://pocketbase.io/docs/how-to-use/.


## Overview

### Use as standalone app

You could download the prebuilt executable for your platform from the [Releases page](https://github.com/pocketbase/pocketbase/releases).
Once downloaded, extract the archive and run `./pocketbase serve` in the extracted directory.

The prebuilt executables are based on the [`examples/base/main.go` file](https://github.com/pocketbase/pocketbase/blob/master/examples/base/main.go) and comes with the JS VM plugin enabled by default which allows to extend PocketBase with JavaScript (_for more details please refer to [Extend with JavaScript](https://pocketbase.io/docs/js-overview/)_).

### Use as a Go framework/toolkit

PocketBase is distributed as a regular Go library package which allows you to build
your own custom app specific business logic and still have a single portable executable at the end.

Here is a minimal example:

0. [Install Go 1.27+](https://go.dev/doc/install) (_if you haven't already_)

1. Create a new project directory with the following `main.go` file inside it:
    ```go
    package main

    import (
        "log"

        "github.com/pocketbase/pocketbase"
        "github.com/pocketbase/pocketbase/core"
    )

    func main() {
        app := pocketbase.New()

        app.OnServe().BindFunc(func(se *core.ServeEvent) error {
            // registers new "GET /hello" route
            se.Router.GET("/hello", func(re *core.RequestEvent) error {
                return re.String(200, "Hello world!")
            })

            return se.Next()
        })

        if err := app.Start(); err != nil {
            log.Fatal(err)
        }
    }
    ```

2. To init the dependencies, run `go mod init myapp && go mod tidy`.

3. To start the application, run `go run main.go serve`.

4. To build a statically linked executable, you can run `CGO_ENABLED=0 go build` and then start the created executable with `./myapp serve`.

_For more details please refer to [Extend with Go](https://pocketbase.io/docs/go-overview/)._

### Building and running the repo main.go example

To build the minimal standalone executable, like the prebuilt ones in the releases page, you can simply run `go build` inside the `examples/base` directory:

0. [Install Go 1.27+](https://go.dev/doc/install) (_if you haven't already_)
1. Clone/download the repo
2. Navigate to `examples/base`
3. Run `CGO_ENABLED=0 go build` to build a binary for your current environment
   _(or to target other platforms use `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build`; see https://go.dev/doc/install/source#environment)_
4. Start the created executable by running `./base serve`.

Note that the supported build targets by the pure Go SQLite driver at the moment are:

| GOOS    | GOARCH  |
|---------|---------|
| darwin  | amd64   |
| darwin  | arm64   |
| freebsd | 386     |
| freebsd | amd64   |
| freebsd | arm     |
| freebsd | arm64   |
| linux   | 386     |
| linux   | amd64   |
| linux   | arm     |
| linux   | arm64   |
| linux   | loong64 |
| linux   | ppc64le |
| linux   | riscv64 |
| linux   | s390x   |
| netbsd  | amd64   |
| openbsd | amd64   |
| openbsd | arm64   |
| windows | 386     |
| windows | amd64   |
| windows | arm64   |

### Testing

PocketBase comes with mixed bag of unit and integration tests.
To run them, use the standard `go test` command:

```sh
go test ./...
```

Check also the [Testing guide](http://pocketbase.io/docs/testing) to learn how to write your own custom application tests.

## Security

If you discover a security vulnerability within PocketBase, please send an e-mail to **support at pocketbase.io**.

You could find more details in the project [Security policy](https://github.com/pocketbase/pocketbase/security/policy).

## Contributing

PocketBase is free and open source project licensed under the [MIT License](LICENSE.md).
You are free to do whatever you want with it, even offering it as a paid service.

You could help continuing its development by:

- [Contribute to the source code](CONTRIBUTING.md)
- [Suggest new features and report issues](https://github.com/pocketbase/pocketbase/issues)

Please refrain creating PRs for _new features_ without previously discussing the implementation details.
PocketBase has a [roadmap](https://github.com/orgs/pocketbase/projects/2) and I try to work on issues in specific order and such PRs often come in out of nowhere and skew all initial planning with tedious back-and-forth communication.

Don't get upset if I close your PR, even if it is well executed and tested. This doesn't mean that it will never be merged.
Later we can always refer to it and/or take pieces of your implementation when the time comes to work on the issue (don't worry you'll be credited in the release notes).

> [!IMPORTANT]
> Due to recent LLM spam, PRs are temporary disabled and only existing collaborators can open a PR.
> If you stumble on a problem that you want to fix, please consider instead opening an issue or discussion with link to your fork _(if not obvious - LLM contributions are not welcome)_.
> This status may change in the future in case GitHub finally decide to do something about the constant spam, or when I find time to move the project somewhere else.
```

### Skipped

- `ui/README.md` — fetch error: githubclient: rate limited
- `Makefile` — fetch error: githubclient: rate limited
- `.github/workflows/release.yaml` — fetch error: githubclient: rate limited
- `examples/base/main.go` — fetch error: githubclient: rate limited
- `modernc_versions_check.go` — fetch error: githubclient: rate limited
- `pocketbase.go` — fetch error: githubclient: rate limited
- `pocketbase_test.go` — fetch error: githubclient: rate limited
- `apis/api_error_aliases.go` — fetch error: githubclient: rate limited
- `apis/backup.go` — fetch error: githubclient: rate limited
- `apis/backup_create.go` — fetch error: githubclient: rate limited

## Source index (cite against these)

- `go.mod`
- `ui/package.json`
- `CONTRIBUTING.md`
- `README.md`

