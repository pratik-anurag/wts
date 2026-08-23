# macOS application plan

## Why desktop fits WTS

WTS needs capabilities that a normal browser application cannot own cleanly:

- persistent access to user-approved repository folders.
- Git and linked-worktree operations.
- child process and PTY supervision.
- native repository-folder selection and notifications.
- opening `.code-workspace` files in VS Code.
- deep links from Jira or a terminal.
- a stable local workspace window while editor windows come and go.

Tauri supplies the native shell while keeping the interface in React and the
trusted implementation in Rust.

## Current native boundary

- One main window, not one WTS window per issue.
- Workspace Board is the default home view.
- Selecting a Board card opens its integrated Workbench in the same window.
- New sessions start on the Board and finish in the Workbench after
  provisioning.
- A user-selected `.code-workspace` file is bounded, one-time setup input. It
  is distinct from the generated workspace WTS later opens in VS Code. Its
  folder entries are hints for matching already discovered local repositories.
  Safe multi-component relative suffixes can disambiguate trusted checkout
  paths or aliases, but imported paths are never opened and do not trigger an
  implicit clone or fetch. The user can separately review and run **Clone from
  URL** to add another trusted source checkout.
- Workbench owns detailed issue operation and keyboard-led local commands.
- VS Code opens as a separate window with the generated multi-root workspace.
- Native notifications, remembered window state, single-instance behavior, and
  deep links remain planned native integrations.

## First Apple Silicon preview

The checked-in helper builds one Apple Silicon `.app` and `.dmg` for local QA.
It uses an explicit ad-hoc identity so macOS can verify that each generated
artifact is internally consistent, but it does not use a Developer ID
certificate, submit anything to Apple, or claim that its output is ready for
other users. It does not install dependencies or the Tauri CLI.

Prerequisites:

```bash
npm ci
npm ci --prefix ui
```

Run:

```bash
npm run build:macos:qa
```

Build and install the verified application in `/Applications`:

```bash
npm run install:macos:qa
```

The installer replaces only an existing app with the `dev.wts.desktop`
identifier. It stops if another app uses the `WTS.app` name. Closing the WTS
window hides it on macOS. Select WTS in the Dock or Applications to restore the
window. Use **WTS > Quit WTS** or Command-Q to stop WTS.

The helper requires an Apple Silicon Mac, the `aarch64-apple-darwin` Rust
target, Tauri CLI 2.11.4, UI dependencies, and at least 6144 MiB of free disk
by default. Set `WTS_MACOS_MIN_FREE_MIB` only when the builder has a measured
lower requirement. An explicitly managed standalone Tauri binary can be
selected with the absolute `WTS_TAURI_BIN` path.
`WTS_TAURI_CLI_VERSION` changes the required version.

For WTS version `0.1.2`, output is:

```text
target/release/bundle/macos/WTS.app
target/release/bundle/dmg/WTS_0.1.2_aarch64.dmg
target/release/bundle/dmg/WTS_0.1.2_aarch64.dmg.sha256
```

The helper leaves Tauri's artifacts in their canonical build directories. It
does not make a second copy. It verifies the `.app` and `.dmg` with `codesign`,
checks that the executable is arm64-only, verifies the disk image with
`hdiutil`, and emits and rechecks a SHA-256 checksum for the transferable DMG.

The helper also creates an updater archive and signature for local QA. It
stages only the newest update in the current user's WTS application-support
directory. WTS checks that fixed local feed at startup, when its window gains
focus, and every five minutes. It verifies the signature, digest, and size, and
then installs the update through the Tauri updater. WTS does not restart during
active work. The installed update starts after the user next restarts WTS.
Feed paths, signatures, and digests do not cross the WebView boundary.

An ad-hoc signature is not an Apple trust credential. This output is not
Developer ID signed or notarized, and Gatekeeper will normally reject or warn
about it after download. Run it only as an internal preview. Do not publish it
as a WTS release.

## Runtime environment limitations

A Finder-launched application does not inherit the interactive shell startup
environment. The current WTS executable discovery uses `PATH`, so Git, VS Code,
Codex, OpenCode, Hermes, Graphify, Node, and Jira MCP launchers may appear
unavailable even when they work in Terminal. For a source-tree QA run, launch
the bundle executable from a configured terminal:

```bash
export WTS_REPOSITORY_ROOT=/absolute/path/to/repositories
export WTS_WORKSPACE_ROOT=/absolute/path/to/workspaces
export WTS_DATA_DIR=/absolute/path/to/wts-data
target/release/bundle/macos/WTS.app/Contents/MacOS/wts-desktop
```

A source-tree QA run can instead configure more than one trusted repository
root with a colon-separated macOS path list:

```bash
export WTS_REPOSITORY_ROOTS="/absolute/path/to/repositories:/absolute/path/to/other-repositories"
```

`WTS_REPOSITORY_ROOTS` takes precedence over `WTS_REPOSITORY_ROOT`. WTS scans
those roots with bounded nested discovery and later creates managed worktrees
below `WTS_WORKSPACE_ROOT`. It does not treat either repository-root setting as
a clone destination. Both repository-root variables are optional in the
desktop app. If neither is configured and no root was previously saved, WTS
starts with an empty catalog and opens the native folder prompt.

A public build needs reviewed executable-path preferences or a narrowly scoped
login-environment bootstrap. It must not blindly source arbitrary shell startup
files.

OpenProject REST API v3 verification and work-package import are implemented.
Configure the adapter in the environment before launching WTS:

```bash
export WTS_OPENPROJECT_URL=https://openproject.example.test
export WTS_OPENPROJECT_TOKEN=replace-with-a-runtime-secret
```

The token is currently environment-managed and remains inside the Rust host.
The UI and setup snapshots receive only secret-free readiness and result data.
Do not place a real token in source control, a package script, or the
application bundle. A future native credential flow should move it to the
user's Keychain.

The local browser-journey stack is also not bundled: Node, the fixed helper,
Playwright, and Chromium remain source-checkout dependencies. The current WTS
self-journey targets an authenticated loopback browser host, not the Tauri
WebView, so it remains unavailable in this preview `.app`.

## Future direct distribution

The `Desktop build and release` GitHub Actions workflow now builds an ad-hoc
signed Apple Silicon QA artifact on pushes to `wts-ui` and manual runs. A tag
matching the desktop version, such as `v0.1.2`, takes the release path and
publishes a macOS app, a DMG, and a SHA-256 checksum. The release job verifies
that the tagged commit belongs to `wts-ui`.

The workflow uses Developer ID signing and Apple notarization when all release
credentials exist. If a credential is missing, the workflow publishes an
ad-hoc-signed GitHub prerelease. Apple does not notarize this preview.

Configure these GitHub Actions secrets before creating a release tag:

```text
APPLE_CERTIFICATE
APPLE_CERTIFICATE_PASSWORD
APPLE_ID
APPLE_PASSWORD
APPLE_TEAM_ID
```

`APPLE_CERTIFICATE` is the base64-encoded Developer ID Application `.p12`.
`APPLE_PASSWORD` is an app-specific Apple password. The workflow publishes a
preview if any release credential is missing. The tag must equal `v` plus the
version in `src-tauri/tauri.conf.json`.

Set the secrets with GitHub CLI:

```bash
gh secret set APPLE_CERTIFICATE < certificate-base64.txt
gh secret set APPLE_CERTIFICATE_PASSWORD
gh secret set APPLE_ID
gh secret set APPLE_PASSWORD
gh secret set APPLE_TEAM_ID
```

Bump `src-tauri/tauri.conf.json`, commit the change, then publish the matching
tag:

```bash
git switch wts-ui
git tag -a v0.1.2 -m "WTS v0.1.2"
git push origin v0.1.2
gh run watch
```

The remaining distribution work is:

1. Join the Apple Developer Program.
2. Create a Developer ID Application certificate.
3. Finalize an organization-owned bundle identifier and release icon set.
4. Build universal or separate Apple Silicon/Intel bundles.
5. Sign nested executables and the app bundle with hardened runtime.
6. Submit with `notarytool` through Tauri's signing workflow.
7. Staple the notarization ticket.
8. Verify with `codesign`, `spctl`, and a clean macOS account.

Tauri documents the required signing identities and notarization environment
variables at https://v2.tauri.app/distribute/sign/macos/.

The current bundle configuration points only to a 256×256 PNG. Before public
distribution, create a release-quality square transparent source, run
`cargo tauri icon <source> --output src-tauri/icons`, retain the generated
`icon.icns`, and verify the icon at Finder, Dock, and installer sizes.

The equivalent local release command is `cargo tauri build --target
aarch64-apple-darwin --bundles app,dmg` after a Developer ID Application
certificate is available and one notarization credential set is supplied:

```text
APPLE_API_ISSUER + APPLE_API_KEY + APPLE_API_KEY_PATH
or
APPLE_ID + APPLE_PASSWORD + APPLE_TEAM_ID
```

These values belong in the release environment or secret store, never in
source control. A universal build also needs both Rust targets and uses
`cargo tauri build --target universal-apple-darwin --bundles dmg`. The local
preview helper deliberately does neither.

The release gate must still be tested from a clean macOS account before broad
distribution.

## Install a GitHub release

The recommended command-line installation uses the small Go installer. Modern
Go versions use `go run` or `go install`, because `go get` manages module
dependencies and no longer installs executable commands:

```bash
go run github.com/pratik-anurag/wts/cmd/wts-ui@latest
```

To keep the installer command available, install it once and ensure Go's binary
directory is on `PATH`:

```bash
go install github.com/pratik-anurag/wts/cmd/wts-ui@latest
wts-ui
```

The installer downloads the release's signed `.app.tar.gz`, verifies the
published SHA-256 checksum, validates the WTS bundle identifier and executable,
and installs it in `~/Applications`. It does not download or mount a DMG. To
install a specific release or use the system Applications directory:

```bash
wts-ui -version v0.1.2
wts-ui -applications-dir /Applications
```

The system Applications directory can require administrator-owned permissions.
The DMG remains available as a manual fallback.

Download the DMG and checksum for a specific version with GitHub CLI:

```bash
mkdir -p "$PWD/wts-download"
gh release download v0.1.2 --pattern '*.dmg' --pattern '*.sha256' --dir "$PWD/wts-download"
cd "$PWD/wts-download"
shasum -a 256 -c WTS-macOS-arm64.sha256
open ./*.dmg
```

Drag **WTS** into **Applications**, eject the mounted image, then launch it:

```bash
open -a WTS
```

For an untagged QA build, open the workflow run in GitHub Actions, download the
`WTS-macOS-arm64-<commit>` artifact, verify its included checksum, and open the
DMG. QA artifacts are ad-hoc signed and can still require approval in macOS
**Privacy & Security**. Tagged releases require Developer ID signing and
notarization.

## Why not the Mac App Store initially

The Mac App Store requires App Sandbox. Apple documents that even a
user-selected folder does not permit a sandboxed application to run programs
outside its bundle/container. WTS must invoke the installed Git binary,
repository toolchains, Codex, OpenCode, Hermes, and VS Code.

Direct Developer ID distribution keeps Gatekeeper and notarization protections
without removing the capabilities WTS exists to provide.

## Native features, phased

### Implemented

- opening the generated workspace in VS Code.
- restoring a hidden WTS window from the Dock or Applications.
- automatic signed local QA update checks and installation, with a
  user-controlled restart.
- GitLab CLI readiness checks for an existing local configuration.

### Next native integrations

- folder picker for repository registration.
- single-instance window.
- window-state persistence.
- native failure/approval notifications.

### After the workflow stabilizes

- `wts://issue/<key>` deep links.
- menu-bar status and quick session switching.
- a public HTTPS update channel with key rotation and rollback procedures.
- Touch ID confirmation for high-risk capability expansion.

Do not add native features merely because Tauri exposes them. Each one must
reduce context switching or make an authority decision safer.
