package gosp

import (
	"fmt"
	"log"
	"reflect"
	"strings"
	"sync"
)

const DefaultContextName = "spring_context"

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
	started        sync.Map
	debug          bool
	logTag         string
	inited         bool
	beanType       reflect.Type
	moduleType     reflect.Type
	syncModuleType reflect.Type
	logger         Logger
	lock           sync.Locker
	ctx            SpringContext
	count          int
}

type SpringContext interface {
	Get(name string) Bean
	GetModule(name string) ModuleBean
	GetSyncModule(name string) SyncModuleBean
}

type contextImpl struct {
	spring *Spring
}

func (t *contextImpl) Get(name string) Bean {

	return t.spring.Get(name)
}

func (t *contextImpl) GetModule(name string) ModuleBean {
	return t.spring.GetModule(name)
}

func (t *contextImpl) GetSyncModule(name string) SyncModuleBean {
	return t.spring.GetSyncModule(name)
}

func (t contextImpl) Name() string {
	return DefaultContextName
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
		if t.lock == nil {
			t.lock = &sync.Mutex{}
		}
		if t.ctx == nil {
			ctx := contextImpl{t}
			t.ctx = &ctx
			var bean Bean = &ctx
			t.instances[DefaultContextName] = &bean
		}
		t.count = 0
		t.beanType = reflect.TypeOf((*Bean)(nil)).Elem()
		t.moduleType = reflect.TypeOf((*ModuleBean)(nil)).Elem()
		t.syncModuleType = reflect.TypeOf((*SyncModuleBean)(nil)).Elem()

		t.inited = true

		// t.Add(t.ctx)
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

// GetBean get bean from SpringContext,by name.
func GetBean[T any](t SpringContext, name string) (T, error) {

	bean := t.Get(name)
	if bean != nil {
		var ins T = bean.(T)
		return ins, nil
	}
	var null T
	return null, fmt.Errorf("the bean named '%s' do not exist", name)
}

// GetModule get bean by name
func (t *Spring) Get(name string) Bean {
	bean, ok := t.instances[name]
	if ok && bean != nil {
		return *bean
	}
	return nil
}

// GetModule get module by name
func (t *Spring) GetModule(name string) ModuleBean {
	module, ok := t.modules[name]
	if ok && module != nil {
		return *module
	}
	return nil
}

// GetSyncModule get SyncModule by name
func (t *Spring) GetSyncModule(name string) SyncModuleBean {
	syncModule, ok := t.syncModules[name]
	if ok && syncModule != nil {
		return *syncModule
	}
	return nil
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
		for _, _ins := range t.syncModules {

			ins := *_ins
			name := ins.Name()
			_, ok := t.started.Load(name)
			if !ok {
				wg.Add(1)
				go func() {
					defer wg.Done()
					ins.Before()
				}()
				if t.debug {
					log.Printf("%s @before run %s.Before() ok ", t.logTag, ins.Name())
				}
			}
		}
		wg.Wait()
	}
}
func (t *Spring) before() {

	log := t.logger
	for _, _ins := range t.modules {
		ins := *_ins
		name := ins.Name()
		_, ok := t.started.Load(name)
		if !ok {
			ins.Before()
			if t.debug {
				log.Printf("%s @before run %s.Before() ok ", t.logTag, ins.Name())
			}
		}
	}
}
func (t *Spring) syncStart() {

	log := t.logger
	if len(t.syncModules) > 0 {
		wg := &sync.WaitGroup{}
		for _, _ins := range t.syncModules {
			ins := *_ins
			name := ins.Name()
			_, ok := t.started.Load(name)
			if !ok {
				wg.Add(1)
				if t.debug {
					log.Printf("%s [Parallel Function] run %s.Start() ", t.logTag, ins.Name())
				}
				ins.Start(wg)
				t.started.Store(name, true)
				if t.debug {
					log.Printf("%s [Parallel Function] finish %s.Start() ", t.logTag, ins.Name())
				}
			} else {
				if t.debug {
					log.Printf("%s [Parallel Function]  %s.Start() had called before! ", t.logTag, ins.Name())
				}
			}

		}
		wg.Wait()
	}

}
func (t *Spring) start() {
	log := t.logger
	for _, _ins := range t.modules {
		ins := *_ins
		name := ins.Name()
		_, ok := t.started.Load(name)
		if !ok {
			ins.Start()
			t.started.Store(name, true)
			if t.debug {
				log.Printf("%s @start run  %s.Start() ok ", t.logTag, ins.Name())
			}
		} else {
			if t.debug {
				log.Printf("%s @start  %s.Start() had called before. ", t.logTag, ins.Name())
			}
		}
	}
}
func (t *Spring) Start() {

	t.count++
	if t.debug {
		t.logger.Printf("%s @Start start count=%d ", t.logTag, t.count)
	}
	t.lock.Lock()
	defer t.lock.Unlock()

	wgBefore := sync.WaitGroup{}
	wgBefore.Add(1)
	go func() {
		defer wgBefore.Done()
		t.syncBefore()
	}()
	wgBefore.Add(1)
	go func() {
		defer wgBefore.Done()
		t.before()
	}()
	wgBefore.Wait()

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
	if t.debug {
		t.logger.Printf("%s @Start finish count=%d ", t.logTag, t.count)
	}
}
