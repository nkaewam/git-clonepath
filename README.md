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

Build and place the executable somewhere on `PATH`:

```sh
go install github.com/nkaewam/clone-path/cmd/git-clonepath@latest
```

Or build the current checkout:

```sh
go build -o git-clonepath ./cmd/git-clonepath
install git-clonepath /usr/local/bin/git-clonepath
```

Git automatically exposes an executable named `git-clonepath` as
`git clonepath`.

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
