*CandyJS* is an intent of create a fully **transparent bridge between Go and the
JavaScript** engine [duktape](http://duktape.org/). Basicly is a syntax-sugar
library built it on top of [go-duktape](https://github.com/crazytyper/go-duktape)
using reflection techniques.

This is a fork of https://github.com/mcuadros/go-candyjs.

It encodes UTF-8 strings passed to duktape into [CESU-8](https://github.com/svaarala/duktape/blob/master/doc/utf8-internal-representation.rst).

It decodes strings returned by duktape from [CESU-8](https://github.com/svaarala/duktape/blob/master/doc/utf8-internal-representation.rst) into UTF-8.

It uses https://github.com/crazytyper/go-cesu8 to encode/decode CESU-8.


#### ok but what for ...

build extensible applications that allow to the user **execute** arbitrary
**code** (let's say plugins) **without** the requirement of **compile** it.

Demo
----

[![asciicast](https://raw.githubusercontent.com/crazytyper/go-candyjs/master/examples/demo/cast.gif)](https://asciinema.org/a/21430)

Features
--------
Embeddable **Ecmascript E5/E5.1 compliant** engine ([duktape](http://duktape.org/)).
```go
ctx := candyjs.NewContext()
ctx.EvalString(`
  function factorial(n) {
    if (n === 0) return 1;
    return n * factorial(n - 1);
  }

  print(factorial(10));
`)  //3628800
```

Call **Go functions** from JavaScript and vice versa.
```go
ctx := candyjs.NewContext()
ctx.PushGlobalGoFunction("golangMultiply", func(a, b int) int {
    return a * b
})

ctx.EvalString(`print(golangMultiply(5, 10));`) //50
```

Transparent interface between **Go structs** and JavaScript.
```go
type MyStruct struct {
    Number int
}

func (m *MyStruct) Multiply(x int) int {
    return m.Number * x
}
...
ctx := candyjs.NewContext()
ctx.PushGlobalStruct("golangStruct", &MyStruct{10})

ctx.EvalString(`print(golangStruct.number);`) //10
ctx.EvalString(`print(golangStruct.multiply(5));`) //50
```

Import of **Go packages** into the JavaScript context.
```go
//go:generate candyjs import fmt
...
ctx := candyjs.NewContext()
ctx.EvalString(`
    var fmt = CandyJS.require('fmt');
    fmt.printf('candyjs is %s', 'awesome')
`) // 'candyjs is awesome'
```


Installation
------------

The recommended way to install go-candyjs is:

```
go get -u github.com/crazytyper/go-candyjs/...
```

> *CandyJS* includes a binary tool used by [go generate](http://blog.golang.org/generate),
please be sure that `$GOPATH/bin` is on your `$PATH`


Examples
--------

### JavaScript running a HTTP server

In this example a [`gin`](https://github.com/gin-gonic/gin) server is executed
and a small JSON is server. In CandyJS you can import Go packages directly if
they are [defined](https://github.com/crazytyper/go-candyjs/blob/master/examples/complex/main.go#L10:L13)
previously on the Go code.

**Interpreter code** (`main.go`)

```go
...
//go:generate candyjs import time
//go:generate candyjs import github.com/gin-gonic/gin
func main() {
    ctx := candyjs.NewContext()
    ctx.PevalFile("example.js")
}

```

**Program code** (`example.js`)

```js
var time = CandyJS.require('time');
var gin = CandyJS.require('github.com/gin-gonic/gin');

var engine = gin.default();
engine.get("/back", CandyJS.proxy(function(ctx) {
  var future = time.date(2015, 10, 21, 4, 29 ,0, 0, time.UTC);
  var now = time.now();

  ctx.json(200, {
    future: future.string(),
    now: now.string(),
    nsecs: future.sub(now)
  });
}));

engine.run(':8080');
```

Caveats
-------
Due to an [incompatibility](https://github.com/svaarala/duktape/issues/154#issuecomment-87077208) with Duktape's error handling system and Go, you can't throw errors from Go. All errors generated from Go functions are generic ones `error error (rc -100)`

License
-------

MIT, see [LICENSE](LICENSE)
