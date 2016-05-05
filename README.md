# drone-git
Drone plugin for cloning git repositories.

## Overview

This plugin is responsible for cloning `git` repositories. It is capable of cloning a specific commit, branch, tag or pull request. The clone path is provided in the `dir` field.

## Usage

Clone a commit

```sh
./drone-git <<EOF
{
    "repo": {
	"clone": "git://github.com/drone/drone"
    },
    "build": {
	"event": "push",
	"branch": "master",
	"commit": "436b7a6e2abaddfd35740527353e78a227ddcb2c",
	"ref": "refs/heads/master"
    },
    "workspace": {
	"root": "/drone/src",
	"path": "/drone/src/github.com/drone/drone",
    }
}
EOF
```

Clone a pull request

```sh
./drone-git <<EOF
{
    "repo": {
	"clone": "git://github.com/drone/drone"
    },
    "build": {
	"event": "pull_request",
	"branch": "master",
	"commit": "8d6a233744a5dcacbf2605d4592a4bfe8b37320d",
	"ref": "refs/pull/892/merge"
    },
    "workspace": {
	"root": "/drone/src",
	"path": "/drone/src/github.com/drone/drone",
    }
}
EOF
```

Clone a tag

```sh
./drone-git <<EOF
{
    "repo": {
	"clone": "git://github.com/drone/drone"
    },
    "build": {
	"event": "tag",
	"branch": "master",
	"commit": "339fb92b9629f63c0e88016fffb865e3e1055483",
	"ref": "refs/tags/v0.2.0"
    },
    "workspace": {
	"root": "/drone/src",
	"path": "/drone/src/github.com/drone/drone",
    }
}
EOF
```

## Build instructions

This code relies on unmerged pull request drone/drone-plugin-go#4 so there are some preparations steps required:

You need to fetch locally pull request drone/drone-plugin-go#4 using the following commands

```sh
cd $GOPATH/src/github.com/drone/drone-plugin-go
git fetch origin pull/4/head:pull-4
git checkout pull-4
```

Then you may be able to build `drone-git` usual way

## Docker

Building Docker container requires the same preparation steps as a regular build does.

Build the Docker container using the `netgo` build tag to eliminate
the CGO dependency:

```sh
CGO_ENABLED=0 go build -a -tags netgo
docker build --rm=true -t sourcegraph/drone-git .
```

Clone a repository inside the Docker container:

```sh
docker run -i plugins/drone-git <<EOF
{
    "repo": {
	"clone": "git://github.com/drone/drone"
    },
    "build": {
	"event": "push",
	"branch": "master",
	"commit": "436b7a6e2abaddfd35740527353e78a227ddcb2c",
	"ref": "refs/heads/master"
    },
    "workspace": {
	"root": "/drone/src",
	"path": "/drone/src/github.com/drone/drone",
    }
}
EOF
```
