package core

import (
	"log"

	"github.com/hpgood/go-spring/simple/infrastruction"
)

type Core struct {
	MyName string
	Cfg    *infrastruction.Config `bean:"config" `
}

func (c Core) BeanName() string {
	return "core"
}
func (c *Core) Before() {
	log.Println("core do something")
	c.MyName = "Gance"
}

func (c *Core) Start() {

	log.Println("core@Start MyName=", c.MyName)
	if c.Cfg != nil {
		log.Println("core@Start Host=", c.Cfg.Host)
		log.Println("core@Start Port=", c.Cfg.Port)
		log.Println("core@Start User=", c.Cfg.User)
	}

}
