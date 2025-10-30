# DevPrivOps

This repository holds the code for a tool that ensures a system follows privacy regulations.

The tool takes system descriptions and rules and tells whether the system abides or not by the privacy rules.

# Deployment

The tool can be used as a docker container, a native binary or included in a CI/CD pipeline.

## Docker Container

The tool can be executed from a local docker conatainer.
The container we provide already has a working Fuseki instance exposed on port 3030, without user or password, and a dataset called `tmp`.

To run the tool with docker containers, we can use bind mounts when starting the container

```sh
docker run -d \ 
    --name devprivops \ 
    -v "<host directory>:/<container directory>/:ro" \ 
    devprivops
```

where `host directory` is where the tool files are located. By convention, it should be the local directory.

And then when running commands from the host, simply run

```sh
docker exec devprivops devprivops test user pass 127.0.0.1 3030 tmp --local-dir <container directory>
```

This will give access to the configuration directories in the host to the container.

**Note**:
This tool requires a base image that we call "Golang Fuseki".
In the case that this tool is being executed outside of the environment recommended in the [docs](https://github.com/ATNoG/rigourous-devprivops-docs), this image may be built with:
```sh
docker build -t golang-fuseki -f Dockerfile.golang_fuseki .
```

The tool's Dockerfile might have to be changed to point to the new image.

## Native Binary

The tool can be installed nativelly by compiling the source code.
To do this, it is needed to install the dependencies in the `shell.nix`, either through `nix` itself, or by manually installing them through other means.
To use the nix shell, run `nix-shell`.

Then, to have the binary, run `go build`.

To execute the tool, run `./devprivops <args>` or place the binary in a directory in `$PATH`.

**NOTE**: this aproach requires an instance of Apache Jena Fuseki to be set up and accessible.

## CI/CD pipeline

To use this tool within an action, install it on the pipeline, either natively through shell commands or by running the pipeline in our docker container, and execute it as a native binary.

# Development

For better dependency management, we provide a `shell.nix` file with all needed dependencies.
To use it, install `nix` and execute `nix-shell`.

# Usage

<!--
The tool can be isntalled nativelly by compiling the source code.
To do this, it is needed to install the dependencies in the `shell.nix`, either through `nix` itself, or by procedurally installing them through other means.

Then, to have the binary, we simply need to run `go build`.

To execute the tool, run `./devprivops <args>` or place the binary in a directory in `$PATH`.

**NOTE**: this aproach requires an instance of Apache Jena Fuseki to be set up and accessible.
-->

To use the tool, simply call `devprivops <user> <pass> <ip> <port> <dataset>`.

## Arguments

- `user`: The username for authentication with the Fuseki triple store
- `pass`: The password for authentication with the Fuseki triple store
- `ip`: The triple store's IP
- `port`: The triple store's port
- `dataset`: The dataset within the triple store to use

# Features

This tool allows for:

- Representation of
    + attack and harm trees
    + system descriptions
    + reasoner rules
    + regulations
    + arbitrary data queries
    + requirements and use cases
- Assistance to writing descriptions through JSON schemas 
- Execution report generation
- Analysis of system compliance with privacy regulations
- Distribution of regulations
- Tests for regulations

# License

The project is licensed under ...

The file `.devprivops/schemas/gh-action-schema.json` was taken from the [SchemaStore repository](https://github.com/SchemaStore/schemastore/blob/f728a2d857a938979f09b0a7f014fbe0bc1898ee/src/schemas/json/github-action.json), with some slight alterations, distributed under the [Apache 2.0 license](https://www.apache.org/licenses/LICENSE-2.0).
