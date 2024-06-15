package main

import (
	"github.com/hpgood/go-spring"
	"github.com/hpgood/go-spring/simple/core"
	"github.com/hpgood/go-spring/simple/infrastruction"
)

func main() {

	app := spring.NewSpring()
	app.SetDebug(true)
	app.Add(&core.Core{})
	app.Add(&infrastruction.Config{})
	app.Start()
}
