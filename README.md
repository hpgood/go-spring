# go-spring/gosp
Inversion of Control (IoC) and Dependency Injection (DI) for go.

go spring 迷你版，快速依赖注入使用。常用于 DDD(Domain-Driven Design) 工程。

example:

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

