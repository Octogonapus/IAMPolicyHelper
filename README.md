# IAMPolicyHelper: :zap: :mag: Lightning fast interactive search to help you write IAM policies

![](./demo.svg)

## Install

### Binary Installation

Download an appropriate binary from the [latest release](https://github.com/Octogonapus/IAMPolicyHelper/releases/latest).

### Manual Installation

```sh
git clone https://github.com/Octogonapus/IAMPolicyHelper
cd IAMPolicyHelper
go build
go install
```

## AWS IAM Policy Updates

AWS sometimes updates IAM by introducing new actions/resources/etc. or by changing existing ones.
When this happens, you should remove the local copy of the IAM policies located at `~/.iampolicyhelper/rawData.json`.

## How does it work?

The latest IAM documentation is scraped from the AWS website and saved locally the first time you run the program.
Your filter term is then searched against the local definitions.
