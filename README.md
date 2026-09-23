# go-canopen

Implement canopen protocol with [https://github.com/angelodlfrtr/go-can](https://github.com/angelodlfrtr/go-can)

Port of [https://github.com/christiansandberg/canopen](https://github.com/christiansandberg/canopen)
written in Python

## Installation

```bash
go get github.com/angelodlfrtr/go-canopen
```

## Basic usage

```go
package main

import (
  "github.com/angelodlfrtr/go-can"
  "github.com/angelodlfrtr/go-canopen"
  "github.com/angelodlfrtr/go-can/transports"
  "log"
)

func main() {
}
```

## SDO abort handling

When the SDO server answers a read or write with an abort transfer (for example
because the object does not exist), the request is not retried and an
`*canopen.SDOAbortError` is returned immediately. It carries the index, subindex
and CiA 301 abort code of the rejected request, the `SDOAbort...` constants name
the common codes.

```go
data, err := node.SDOClient.Read(0x2008, 1)

var abortErr *canopen.SDOAbortError
if errors.As(err, &abortErr) && abortErr.Code == canopen.SDOAbortObjectDoesNotExist {
  // The node firmware does not implement 0x2008
}
```

Only an abort sent on the server TX COB-ID (0x580 + node id) for the same index
and subindex ends the request, frames of other nodes or objects are ignored.
Without any matching answer the request is retried with a doubling timeout and
finally fails with a timeout error.

## License

Copyright (c) 2019 The go-canopen contributors

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"),
to deal in the Software without restriction, including without limitation the
rights to use, copy, modify, merge, publish, distribute, sublicense, and/or
sell copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS
OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
