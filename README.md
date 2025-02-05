# go-canopen

![GitHub Release](https://img.shields.io/github/v/release/jaster-prj/go-canopen)[![main CI](https://github.com/jaster-prj/go-canopen/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/jaster-prj/go-canopen/actions/workflows/ci.yml)

Implement canopen protocol with [https://github.com/angelodlfrtr/go-can](https://github.com/angelodlfrtr/go-can)

Port of [https://github.com/christiansandberg/canopen](https://github.com/christiansandberg/canopen)
written in Python

## Installation

```bash
go get github.com/jaster-prj/go-canopen
```

## Basic usage

```go
package main

import (
  "github.com/angelodlfrtr/go-can"
  "github.com/angelodlfrtr/go-can/transports"
  "github.com/jaster-prj/go-canopen"
  "log"
)

func main() {
}
```
