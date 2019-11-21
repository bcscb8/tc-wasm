module github.com/xunleichain/tc-wasm

go 1.12

// replace github.com/go-interpreter/wagon => github.com/xunleichain/wagon v0.5.3
replace github.com/go-interpreter/wagon => github.com/bcscb8/wagon v0.0.0-20191121064711-ee7998ad91d3

require (
	github.com/go-interpreter/wagon v0.0.0
	golang.org/x/crypto v0.0.0-20190701094942-4def268fd1a4
)
