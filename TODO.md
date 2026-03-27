# TODO

## Feature: `mw catalog init`

### Implementation
- [ ] Add `InitResult` struct to `mw-core/src/catalog.rs` (`created`, `already_existed` path lists)
- [ ] Add `create_dir_tracking()` private helper in `catalog.rs`
- [ ] Add `Catalog::init(config) -> Result<InitResult, CatalogError>` method
  - [ ] Create config directory
  - [ ] Write default `config.toml` if absent (via `config.save()`)
  - [ ] Write empty `catalog.toml` if absent (via `Catalog::default().save()`)
  - [ ] Create `bare_root` directory
  - [ ] Create `worktree_root` directory
  - [ ] Idempotent: never overwrite existing files
- [ ] Add `CatalogAction::Init` variant to CLI enum in `commands/mod.rs`
- [ ] Add `Init` handler: print created/existing paths

### Tests
- [ ] `init_creates_directories` — verify bare_root and worktree_root exist after init
- [ ] `init_is_idempotent` — second call reports nothing new created

### Validation
- [ ] `cargo build` compiles
- [ ] `cargo test` passes (all existing + new)
- [ ] `cargo clippy --all-targets -- -D warnings` clean
- [ ] `cargo fmt -- --check` clean
- [ ] `mw catalog init --help` shows correct description

---

## Feature: `mw catalog add <url>`

### Implementation
- [ ] Add `Catalog::catalog_add_url(&mut self, url, config) -> Result<String, CatalogError>` to `catalog.rs`
  - [ ] Parse URL via `parse_remote_url()`
  - [ ] Derive repo name from last URL segment
  - [ ] Idempotency check (return existing name if already registered)
  - [ ] Compute bare clone destination from parsed URL (host/segments path)
  - [ ] `clone_bare(url, &bare_dest)` — hard error on failure
  - [ ] `fetch`, `get_default_branch`, `create_worktree`, `maintenance_register`
  - [ ] Build Repository entry with URL stored, insert, save
- [ ] Rename `CatalogAction::Add { path }` field to `source`, update help text
- [ ] Modify `Add` handler: detect URL via `parse_remote_url().is_some()`, dispatch to `catalog_add_url` or `catalog_add`

### Tests
- [ ] `catalog_add_url_rejects_invalid_url` — pass garbage, assert error
- [ ] `catalog_add_url_is_idempotent` — add same URL twice via local bare repo, assert single entry
- [ ] `catalog_add_url_from_github` — `#[ignore]` network test with real public repo
- [ ] Verify existing `catalog_add` tests still pass (local path flow unchanged)

### Validation
- [ ] `cargo build` compiles
- [ ] `cargo test` passes (all existing + new)
- [ ] `cargo clippy --all-targets -- -D warnings` clean
- [ ] `cargo fmt -- --check` clean
- [ ] `mw catalog add --help` shows "Local path or git URL" description

---

## Final validation
- [ ] `cargo test` — full suite green
- [ ] `cargo clippy --all-targets -- -D warnings` — clean
- [ ] `cargo fmt -- --check` — clean
- [ ] Update `PLAN.md` — remove `init catalog` from out-of-scope list
