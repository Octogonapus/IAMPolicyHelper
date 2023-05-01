# IAMPolicyHelper: :zap: :mag: Lightning fast interactive search to help you write IAM policies

![](./demo.svg)

## Install

```sh
go install github.com/Octogonapus/IAMPolicyHelper
```

## How does it work?

The latest IAM documentation is scraped from the AWS website and saved locally the first time you run the program.
Your filter term is then searched against the local definitions.
