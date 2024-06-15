package main

import (
	"github.com/hpgood/go-spring/gosp"
	"github.com/hpgood/go-spring/simple/core"
	"github.com/hpgood/go-spring/simple/infrastruction"
)

func main() {

	app := gosp.Spring{}
	app.SetDebug(true)
	app.Add(&core.Core{MyName: "00000"})
	app.Add(&infrastruction.Config{Host: "127.0.0.1"})
	app.Start()
}
