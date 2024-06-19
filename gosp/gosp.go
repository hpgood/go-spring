package gosp

import (
	"fmt"
	"log"
	"reflect"
	"strings"
	"sync"
)

// ModuleBean 拥有依赖注入方法Before()，依赖注入后的处理方法Start()
type ModuleBean interface {
	Before()
	Start()
	Name() string
}
type SyncModuleBean interface {
	Before()
	Start(*sync.WaitGroup)
	Name() string
}

// Bean
type Bean interface {
	Name() string
}

// Spring
type Spring struct {
	instances      map[string]*Bean
	modules        map[string]*ModuleBean
	syncModules    map[string]*SyncModuleBean
	debug          bool
	logTag         string
	inited         bool
	beanType       reflect.Type
	moduleType     reflect.Type
	syncModuleType reflect.Type
	logger         Logger
}

type Logger interface {
	Println(...interface{})
	Fatalln(...interface{})
	Printf(string, ...interface{})
	Fatalf(string, ...interface{})
}

func (t *Spring) SetDebug(b bool) {
	t.debug = b
}

func (t *Spring) SetLogger(logger Logger) {
	t.logger = logger
}

// init 初始化
func (t *Spring) Init() {

	if !t.inited {
		if t.instances == nil {
			t.instances = make(map[string]*Bean)
		}
		if t.modules == nil {
			t.modules = make(map[string]*ModuleBean)
		}
		if t.syncModuleType == nil {
			t.syncModules = make(map[string]*SyncModuleBean)
		}
		if t.logger == nil {
			t.logger = &log.Logger{}
		}
		if t.logTag == "" {
			t.logTag = "[go-spring] "
		}
		t.beanType = reflect.TypeOf((*Bean)(nil)).Elem()
		t.moduleType = reflect.TypeOf((*ModuleBean)(nil)).Elem()
		t.syncModuleType = reflect.TypeOf((*SyncModuleBean)(nil)).Elem()
		t.inited = true
	}

}

// Add add one been to spring
func (t *Spring) Add(cls interface{}) {

	clsType := reflect.TypeOf(cls)
	isModule := false
	log := t.logger

	if clsType.Implements(t.moduleType) {
		module := cls.(ModuleBean)
		old, ok := t.modules[module.Name()]
		isModule = true
		if ok && old != nil {
			log.Fatalln(t.logTag, " Error: exist old bean=", module.Name(), "old=", *old)
		}
		t.modules[module.Name()] = &module
		if t.debug {
			log.Println(t.logTag, "Add module/bean=", module.Name())
		}
	} else if clsType.Implements(t.syncModuleType) {
		syncModule := cls.(SyncModuleBean)
		old, ok := t.modules[syncModule.Name()]
		isModule = true
		if ok && old != nil {
			log.Fatalln(t.logTag, " Error: exist old bean=", syncModule.Name(), "old=", *old)
		}
		t.syncModules[syncModule.Name()] = &syncModule
		if t.debug {
			log.Println(t.logTag, "Add syncModule/bean=", syncModule.Name())
		}
	} else if !clsType.Implements(t.beanType) {

		log.Fatalln(t.logTag, " Error: the struct do not implement the Name() method ,struct=", cls)
	}

	bean := cls.(Bean)

	old, ok := t.instances[bean.Name()]
	if ok && old != nil {
		log.Fatalln(t.logTag, " Error: exist old bean=", bean.Name(), "old=", *old)
	}

	t.instances[bean.Name()] = &bean
	if !isModule && t.debug {
		log.Println(t.logTag, "Add bean=", bean.Name())
	}

}

// GetModule get bean by name
func (t *Spring) Get(name string) *Bean {
	return t.instances[name]
}

// GetModule get module by name
func (t *Spring) GetModule(name string) *ModuleBean {
	return t.modules[name]
}

// GetSyncModule get SyncModule by name
func (t *Spring) GetSyncModule(name string) *SyncModuleBean {
	return t.syncModules[name]
}

// autoInjection
func (t *Spring) autoInjection() {
	log := t.logger
	for beanName, ins := range t.instances {

		value := reflect.ValueOf(ins)
		realValue := value.Elem().Elem().Elem()

		reflectType := realValue.Type()

		for i := 0; i < reflectType.NumField(); i++ {

			field := reflectType.Field(i)

			ref := field.Tag.Get("bean")
			if ref != "" {

				tmp, ok := t.instances[ref]
				if ok {

					_field := realValue.FieldByName(field.Name)

					_type := _field.Type()

					newPtr := reflect.ValueOf(*tmp)
					matchTyped := newPtr.Convert(_type)

					if t.debug {
						log.Println(t.logTag, "@autoInjection ", beanName, "inject name=", field.Name, "ref=", ref, "type=", _type)
					}

					if _field.CanSet() {
						_field.Set(matchTyped)
						if t.debug {
							log.Println(t.logTag, "@autoInjection ", beanName, "inject ref=", ref, " success.")
						}
					} else {
						name := field.Name
						if len(name) <= 1 {
							name = "Set" + strings.ToUpper(name)
						} else {
							name = "Set" + strings.ToUpper(name[0:1]) + name[1:]
						}
						realPtrValue := value.Elem().Elem()
						_fieldSet := realPtrValue.MethodByName(name)
						if _fieldSet.IsValid() {
							_fieldSet.Call([]reflect.Value{newPtr})
							if t.debug {
								log.Printf("%s @autoInjection  %s.%s(%s) Success. ", t.logTag, beanName, name, ref)
							}
						} else {
							structName := reflectType.Name()
							fmt.Printf(`请添加以下代码到结构体%s :
func (t *%s) %s(arg %s) {
	t.%s = arg
}
`, structName, structName, name, _type, field.Name)
							log.Fatalln(t.logTag, "@autoInjection ", beanName, " Error: please defind function ", name, "for", structName)

						}

					}

				} else {
					log.Fatalf("%s @autoInjection error: do not exist ref=%s for bean %s ", t.logTag, ref, (*ins).Name())
				}
			}

		}

	}
}

func (t *Spring) syncBefore() {
	log := t.logger
	if len(t.syncModules) > 0 {
		wg := &sync.WaitGroup{}
		for _, ins := range t.syncModules {
			wg.Add(1)
			go func() {
				defer wg.Done()
				(*ins).Before()
			}()

			if t.debug {
				log.Printf("%s @before run %s.Before() ok ", t.logTag, (*ins).Name())
			}
		}
		wg.Wait()
	}
}
func (t *Spring) before() {

	log := t.logger
	for _, ins := range t.modules {
		(*ins).Before()
		if t.debug {
			log.Printf("%s @before run %s.Before() ok ", t.logTag, (*ins).Name())
		}
	}
}
func (t *Spring) syncStart() {

	log := t.logger
	if len(t.syncModules) > 0 {
		wg := &sync.WaitGroup{}
		for _, ins := range t.syncModules {
			wg.Add(1)
			if t.debug {
				log.Printf("%s [Parallel Function] run %s.Start() ", t.logTag, (*ins).Name())
			}
			(*ins).Start(wg)
			if t.debug {
				log.Printf("%s [Parallel Function] finish %s.Start() ", t.logTag, (*ins).Name())
			}
		}
		wg.Wait()
	}

}
func (t *Spring) start() {
	log := t.logger
	for _, ins := range t.modules {
		(*ins).Start()
		if t.debug {
			log.Printf("%s @start run  %s.Start() ok ", t.logTag, (*ins).Name())
		}
	}
}
func (t *Spring) Start() {

	wg1 := sync.WaitGroup{}
	wg1.Add(1)
	go func() {
		defer wg1.Done()
		t.syncBefore()
	}()
	wg1.Add(1)
	go func() {
		defer wg1.Done()
		t.before()
	}()
	wg1.Wait()

	t.autoInjection()

	wgStart := sync.WaitGroup{}
	wgStart.Add(1)
	go func() {
		defer wgStart.Done()
		t.syncStart()
	}()
	wgStart.Add(1)
	go func() {
		defer wgStart.Done()
		t.start()
	}()

	wgStart.Wait()

}
