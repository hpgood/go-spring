package gosp

import (
	"log"
	"reflect"
	"strings"
)

// Bean
type Bean interface {
	Before()
	Start()
	Name() string
}

// Spring
type Spring struct {
	instances map[string]*Bean
	debug     bool
	logTag    string
	inited    bool
}

func (t *Spring) SetDebug(b bool) {
	t.debug = b
}

// init 初始化
func (t *Spring) init() {
	if !t.inited {
		if t.instances == nil {
			t.instances = make(map[string]*Bean)
		}
		if t.logTag == "" {
			t.logTag = "[go-spring] "
		}
		t.inited = true
	}

}

// Add add one been to spring
func (t *Spring) Add(cls interface{}) {
	t.init()

	bean := cls.(Bean)
	if t.debug {
		log.Println(t.logTag, "Add bean=", bean.Name())
	}
	old, ok := t.instances[bean.Name()]
	if ok && old != nil {
		log.Fatalln(t.logTag, " Error: exist old bean=", bean.Name(), "old=", *old)
	}
	t.instances[bean.Name()] = &bean
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
						log.Println(t.logTag, "@autoInjection ", beanName, " set ref=", ref)
						log.Println(t.logTag, "@autoInjection ", beanName, " set type=", _type)
						log.Println(t.logTag, "@autoInjection ", beanName, " set field.Name=", field.Name)
					}

					if _field.CanSet() {
						_field.Set(matchTyped)
						if t.debug {
							log.Println(t.logTag, "@autoInjection ", beanName, " set ref=", ref, " success.")
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
	for _, ins := range t.instances {
		(*ins).Before()
		if t.debug {
			log.Printf("%s @before run %s.Before() ok \n", t.logTag, (*ins).Name())
		}
	}
}
func (t *Spring) start() {
	for _, ins := range t.instances {
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
