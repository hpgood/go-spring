package gosp

import (
	"log"
	"reflect"
	"strings"
	"sync"
)

var locker sync.Mutex

// ModuleBean 拥有依赖注入方法Before()，依赖注入后的处理方法Start()
type ModuleBean interface {
	Before()
	Start()
	Name() string
}

// Bean
type Bean interface {
	Name() string
}

// Spring
type Spring struct {
	instances  map[string]*Bean
	modules    map[string]*ModuleBean
	debug      bool
	logTag     string
	inited     bool
	beanType   reflect.Type
	moduleType reflect.Type
}

func (t *Spring) SetDebug(b bool) {
	t.debug = b
}

// init 初始化
func (t *Spring) init() {

	locker.Lock()
	defer locker.Unlock()

	if !t.inited {
		if t.instances == nil {
			t.instances = make(map[string]*Bean)
		}
		if t.modules == nil {
			t.modules = make(map[string]*ModuleBean)
		}
		if t.logTag == "" {
			t.logTag = "[go-spring] "
		}
		t.moduleType = reflect.TypeOf((*ModuleBean)(nil)).Elem()
		t.beanType = reflect.TypeOf((*Bean)(nil)).Elem()
		t.inited = true
	}

}

// Add add one been to spring
func (t *Spring) Add(cls interface{}) {
	t.init()

	clsType := reflect.TypeOf(cls)
	isModule := false

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

// autoInjection
func (t *Spring) autoInjection() {
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
								log.Printf("%s @autoInjection  %s.%s(%s) Success. \n", t.logTag, beanName, name, ref)
							}
						} else {
							if t.debug {
								log.Println(t.logTag, "@autoInjection ", beanName, " Error: please defind function ", name, "for", reflectType.Name())
							}
						}

					}

				} else {
					log.Fatalf("%s @autoInjection error: do not exist ref=%s for bean %s \n", t.logTag, ref, (*ins).Name())
				}
			}

		}

	}
}
func (t *Spring) before() {
	for _, ins := range t.modules {
		(*ins).Before()
		if t.debug {
			log.Printf("%s @before run %s.Before() ok \n", t.logTag, (*ins).Name())
		}
	}
}
func (t *Spring) start() {
	for _, ins := range t.modules {
		(*ins).Start()
		if t.debug {
			log.Printf("%s @start run  %s.Start() ok \n", t.logTag, (*ins).Name())
		}
	}
}
func (t *Spring) Start() {

	t.init()
	t.before()
	t.autoInjection()
	t.start()

}
