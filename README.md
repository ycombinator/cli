# Xata Command Line Interface (CLI)

> ⚠️ **Warning**: This feature is still under development and this document is likely to change. Proceed with caution.

Xata CLI is convenience tool for developers to interact with Xata. It is intended to provide feature parity with Xata's [Web UI](https://app.xata.io) to enable developers to build applications at **rapid pace** with **as little friction as possible**.

Check the [Getting Started](https://docs.xata.io/cli/getting-started) guide for in depth documentation.

## Installing

### macOS

Run the install script according to your platform. The script will install the `xata` binary in `$HOME/.xata/bin` and will print instructions on how to add it to your `PATH`.

```sh
curl -L  https://xata.io/install.sh | sh
```

### Linux

Run the install script according to your platform. The script will install the `xata` binary in `$HOME/.xata/bin` and will print instructions on how to add it to your `PATH`.

```sh
curl -L  https://xata.io/install.sh | sh
```

### Windows

Run the Powershell script:

```powershell
iwr https://xata.io/install.ps1 -useb | iex
```

## Authentication

To authenticate your CLI installation, create a Personal API key in your **Account settings** page and copy it to clipboard.

Run `xata auth login` and paste the key from your clipboard when prompted.

## Initializing a New Database

To initialize a new database,

- run `xata init` in your project's directory
- choose a name for your new database
- select a [workspace](https://docs.xata.io/concepts/workspaces) to connect it to

This will create a folder called `xata` in your project that contains some required configuration for Xata to do its magic. It's a good idea to commit this folder to your project.

In this folder, there is a file called `schema.json` that contains the [schema](https://docs.xata.io/concepts/schema) of your database on Xata. Edit this file to match the intended design of your database. When you're done, save and commit this file. Then, run `xata deploy` to bring your database online.

Once this is done, your project is connected to its database on Xata. Proceed to the end of this page for next steps.