package spring

import "github.com/hpgood/go-spring/gosp"

func NewSpring() *gosp.Spring {
	s := gosp.Spring{}
	s.Init()
	s.SetDebug(false)
	return &s
}
