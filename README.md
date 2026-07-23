# git-clonepath

`git-clonepath` is a Git external subcommand that derives every clone
destination from the remote:

```text
<clonepath.root>/<host>/<repository-path>
```

For example:

```sh
git config --global clonepath.root ~/Developer
git clonepath git@github.com:nkaewam/git-clonepath.git
```

clones into:

```text
~/Developer/github.com/nkaewam/git-clonepath
```

## Install

### GitHub Releases

Download the archive for the current operating system and CPU architecture from
[GitHub Releases](https://github.com/nkaewam/git-clonepath/releases). Set
`VERSION` to the release tag you want to install:

```sh
VERSION=v0.1.1

case "$(uname -s)" in
  Darwin) OS=darwin ;;
  Linux) OS=linux ;;
  *) echo "Unsupported operating system: $(uname -s)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) echo "Unsupported CPU architecture: $(uname -m)" >&2; exit 1 ;;
esac

PACKAGE="git-clonepath_${VERSION#v}_${OS}_${ARCH}"
ARCHIVE="${PACKAGE}.tar.gz"
BASE_URL="https://github.com/nkaewam/git-clonepath/releases/download/${VERSION}"

curl -fLO "${BASE_URL}/${ARCHIVE}"
curl -fLO "${BASE_URL}/SHA256SUMS"

if [ "${OS}" = darwin ]; then
  grep " ${ARCHIVE}$" SHA256SUMS | shasum -a 256 -c -
else
  grep " ${ARCHIVE}$" SHA256SUMS | sha256sum -c -
fi

tar -xzf "${ARCHIVE}"
mkdir -p "${HOME}/.local/bin"
install -m 0755 "${PACKAGE}/git-clonepath" "${HOME}/.local/bin/git-clonepath"
```

Ensure `~/.local/bin` is on `PATH`, adding the export to your shell profile if
needed:

```sh
export PATH="${HOME}/.local/bin:${PATH}"
command -v git-clonepath
```

Git automatically exposes an executable named `git-clonepath` as
`git clonepath`.

### Build from source

With Go installed:

```sh
go install github.com/nkaewam/clone-path/cmd/git-clonepath@latest
```

## Configure

The clone root is required. It must resolve to an absolute, existing directory:

```sh
mkdir -p ~/Developer
git config --global clonepath.root ~/Developer
```

Normal Git configuration precedence applies, including one-off overrides:

```sh
git -c clonepath.root=~/Work clonepath https://github.com/acme/widgets.git
```

## Use

SCP-style SSH, `ssh://`, and `https://` remotes are supported:

```sh
git clonepath git@github.com:acme/widgets.git
git clonepath ssh://git@github.com:2222/acme/widgets.git
git clonepath https://github.com/acme/widgets.git
```

Clone options before the remote are forwarded unchanged:

```sh
git clonepath --depth 1 --branch main https://github.com/acme/widgets.git
```

The original remote is passed to Git unchanged. Usernames and ports are omitted
only when deriving the local path. Local and `file://` remotes are rejected, as
are unsafe or malformed repository paths.

An existing destination is always an error. On a failed clone, the command
removes only the empty host and namespace directories it created. Git's
standard input, output, error, credential handling, and exact clone exit status
are preserved.

## Develop

```sh
task test
task integration
```

Build the supported release targets:

```sh
task release
```

## Release

Push a version tag to build and publish macOS and Linux archives for AMD64 and
ARM64:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The release workflow tests the project, builds all four targets, generates
`SHA256SUMS`, and creates a GitHub Release with generated release notes.
