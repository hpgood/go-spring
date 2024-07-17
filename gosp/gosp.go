package gosp

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"strings"
	"sync"
)

const DefaultContextName = "spring_context"

// BeforeBean run Before before inject()
type BeforeBean interface {
	Before()
	BeanName() string
}

// BeforeBean run Start after inject()
type StartBean interface {
	Start()
	BeanName() string
}

type SyncModuleBean interface {
	Start(*sync.WaitGroup)
	BeanName() string
}

// Bean
type Bean interface {
	BeanName() string
}

// Spring
type Spring struct {
	instances      map[string]*Bean
	startModules   map[string]*StartBean
	beforeModules  map[string]*BeforeBean
	syncModules    map[string]*SyncModuleBean
	methodMap      map[string]*BeanMap
	started        sync.Map
	debug          bool
	logTag         string
	inited         bool
	beanType       reflect.Type
	startBeanType  reflect.Type
	beforeBeanType reflect.Type
	syncModuleType reflect.Type
	logger         Logger
	lock           sync.Locker
	ctx            SpringContext
	count          int
	once           sync.Once
	instanceCount  int
}

type SpringContext interface {
	Get(name string) Bean
	CreateInstance(ins interface{}) (interface{}, error)
	GetSyncModule(name string) SyncModuleBean
}

type contextImpl struct {
	spring *Spring
}

func (t *contextImpl) Get(name string) Bean {

	return t.spring.Get(name)
}

func (t *contextImpl) CreateInstance(ins interface{}) (interface{}, error) {
	return t.spring.CreateInstance(ins)
}

func (t *contextImpl) GetSyncModule(name string) SyncModuleBean {
	return t.spring.GetSyncModule(name)
}

func (t contextImpl) BeanName() string {
	return DefaultContextName
}

type Logger interface {
	Println(...interface{})
	Fatalln(...interface{})
	Printf(string, ...interface{})
	Fatalf(string, ...interface{})
}

type BeanMethod struct {
	Method   string
	Index    int
	IsSetter bool
	Arg      reflect.Value
}
type BeanMap struct {
	TypeName string
	Methods  []BeanMethod
}

func (t *Spring) SetDebug(b bool) {
	t.debug = b
}

func (t *Spring) SetLogger(logger Logger) {
	t.logger = logger
}

// init 初始化
func (t *Spring) Init() {

	t.once.Do(func() {

		t.instances = make(map[string]*Bean)
		t.startModules = make(map[string]*StartBean)
		t.beforeModules = make(map[string]*BeforeBean)
		t.syncModules = make(map[string]*SyncModuleBean)
		t.methodMap = make(map[string]*BeanMap)
		if t.logger == nil {
			t.logger = &log.Logger{}
		}
		t.logTag = "[go-spring] "
		t.lock = &sync.Mutex{}

		ctx := contextImpl{t}
		t.ctx = &ctx
		var bean Bean = &ctx
		t.instances[DefaultContextName] = &bean

		t.count = 0
		t.beanType = reflect.TypeOf((*Bean)(nil)).Elem()
		t.startBeanType = reflect.TypeOf((*StartBean)(nil)).Elem()
		t.beforeBeanType = reflect.TypeOf((*BeforeBean)(nil)).Elem()
		t.syncModuleType = reflect.TypeOf((*SyncModuleBean)(nil)).Elem()

		t.inited = true
	})

}

// Add add one been to spring
func (t *Spring) Add(cls interface{}) {

	if t == nil {
		log.Fatalln("Spring@Add this spring is nil!")
		return
	}
	if !t.inited {
		t.Init()
	}

	clsType := reflect.TypeOf(cls)
	isModule := false
	log := t.logger

	// has Start() method
	if clsType.Implements(t.startBeanType) {
		module := cls.(StartBean)
		old, ok := t.startModules[module.BeanName()]
		isModule = true
		if ok && old != nil {
			log.Fatalln(t.logTag, " Error: startModule exist old bean=", module.BeanName(), "old=", *old)
		}
		t.startModules[module.BeanName()] = &module
		if t.debug {
			log.Println(t.logTag, "Add startModule=", module.BeanName())
		}
	}
	// has Before() method
	if clsType.Implements(t.beforeBeanType) {
		module := cls.(BeforeBean)
		old, ok := t.beforeModules[module.BeanName()]
		isModule = true
		if ok && old != nil {
			log.Fatalln(t.logTag, " Error: beforeModules exist old bean=", module.BeanName(), "old=", *old)
		}
		t.beforeModules[module.BeanName()] = &module
		if t.debug {
			log.Println(t.logTag, "Add beforeModule=", module.BeanName())
		}
	}
	// has Start(*sync.WaitGroup) method
	if clsType.Implements(t.syncModuleType) {
		syncModule := cls.(SyncModuleBean)
		old, ok := t.startModules[syncModule.BeanName()]
		isModule = true
		if ok && old != nil {
			log.Fatalln(t.logTag, " Error: syncModule exist old bean=", syncModule.BeanName(), "old=", *old)
		}
		t.syncModules[syncModule.BeanName()] = &syncModule
		if t.debug {
			log.Println(t.logTag, "Add syncModule/bean=", syncModule.BeanName())
		}
	}
	if !clsType.Implements(t.beanType) {

		log.Fatalln(t.logTag, " Error: the struct do not implement the BeanName() method ,struct=", cls)
	}

	if reflect.ValueOf(cls).IsNil() {
		log.Fatalln(t.logTag, " Error: can not Add a nil var to spring! clsType is ", clsType)
	}
	bean := cls.(Bean)

	old, ok := t.instances[bean.BeanName()]
	if ok && old != nil {
		log.Fatalln(t.logTag, " Error: exist old bean=", bean.BeanName(), "old=", *old)
	}

	t.instances[bean.BeanName()] = &bean
	if !isModule && t.debug {
		log.Println(t.logTag, "Add bean=", bean.BeanName())
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

func CreateInstance[T any](t SpringContext, ins T) (T, error) {
	_, err := t.CreateInstance(ins)
	return ins, err
}

// GetModule get bean by name
func (t *Spring) Get(name string) Bean {
	if !t.inited {
		t.Init()
	}
	bean, ok := t.instances[name]
	if ok && bean != nil {
		return *bean
	}
	return nil
}

// GetModule get module by name
func (t *Spring) GetStartModule(name string) StartBean {
	if !t.inited {
		t.Init()
	}
	module, ok := t.startModules[name]
	if ok && module != nil {
		return *module
	}
	return nil
}

// GetSyncModule get SyncModule by name
func (t *Spring) GetSyncModule(name string) SyncModuleBean {
	if !t.inited {
		t.Init()
	}
	syncModule, ok := t.syncModules[name]
	if ok && syncModule != nil {
		return *syncModule
	}
	return nil
}

// autoInjection
func (t *Spring) autoInjection() {
	// log := t.logger
	for beanName, ins := range t.instances {

		_, ok := t.started.Load(beanName)
		if ok {
			// do not inject which is started.
			continue
		}

		t.injection(beanName, ins, true)

	}
}

// CreateInstance 创建实例,并依赖
func (t *Spring) CreateInstance(ins interface{}) (interface{}, error) {

	t.instanceCount++

	if reflect.ValueOf(ins).IsNil() {
		typeName := reflect.ValueOf(ins).Type().String()
		return ins, fmt.Errorf("%s @Create Error:the instance(%s) is nil", t.logTag, typeName)
	}
	// get the mapper of method
	beanMap, err := t.getMethodMapper(ins)
	if err != nil {
		return ins, err
	}
	ptr := reflect.ValueOf(ins)
	for _, m := range beanMap.Methods {
		if m.IsSetter {
			_fieldSet := ptr.Method(m.Index)
			_fieldSet.Call([]reflect.Value{m.Arg})
		} else {
			_field := ptr.Field(m.Index)
			_field.Set(m.Arg)
		}
	}

	return ins, nil
}

// Import 存放已经依赖好的实例
func (t *Spring) Import(ins Bean) error {
	t.instances[ins.BeanName()] = &ins
	return nil
}

// checkError 检查错误,是否抛出异常
func (t *Spring) checkError(msg string, throw bool) error {
	if throw {
		t.logger.Fatalln(msg)
		return nil
	} else {
		return errors.New(msg)
	}
}

// getMethodMapper  get the mapper of struct
func (t *Spring) getMethodMapper(ins interface{}) (*BeanMap, error) {

	insType := reflect.TypeOf(ins).Elem()
	typeName := fmt.Sprintf("%s/%s", insType.PkgPath(), insType.Name())

	method, ok := t.methodMap[typeName]
	if ok {
		return method, nil
	}
	m := BeanMap{}

	{
		log := t.logger
		value := reflect.ValueOf(ins)
		realPtrValue := value
		realValue := value.Elem()
		maxLevel := 10
		beanName := typeName
		throw := false
		// interface{} , ptr ,struct
		{ //find the struct
			for level := 0; level < maxLevel && realValue.Kind() != reflect.Struct; level++ {
				realPtrValue = realValue
				realValue = realValue.Elem()
			}
		}

		methodToIndex := make(map[string]int)
		typ := reflect.TypeOf(ins)

		for i := 0; i < typ.NumMethod(); i++ {
			method := typ.Method(i)
			methodToIndex[method.Name] = i
		}

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
						log.Println(t.logTag, "@getMethodMap ", beanName, "inject name=", field.Name, "ref=", ref, "type=", _type)
					}

					if _field.CanSet() {

						method := BeanMethod{}
						method.Arg = matchTyped
						method.Method = field.Name
						method.Index = i
						method.IsSetter = false
						m.Methods = append(m.Methods, method)

					} else {
						name := field.Name
						if len(name) <= 1 {
							name = "Set" + strings.ToUpper(name)
						} else {
							name = "Set" + strings.ToUpper(name[0:1]) + name[1:]
						}
						_fieldSet := realPtrValue.MethodByName(name)
						if _fieldSet.IsValid() {

							method := BeanMethod{}
							method.Arg = newPtr
							method.Method = name
							method.Index = methodToIndex[name]
							method.IsSetter = true

							m.Methods = append(m.Methods, method)

						} else {
							structName := reflectType.Name()
							fmt.Printf(`请添加以下代码到结构体%s :
func (t *%s) %s(arg %s) {
t.%s = arg
}
`, structName, structName, name, _type, field.Name)

							msg := fmt.Sprint(t.logTag, "@getMethodMap ", beanName, " Error: please defind function ", name, " for ", structName)
							return nil, t.checkError(msg, throw)

						}
					}

				} else {
					msg := fmt.Sprintf("%s @autoInjection error: do not exist ref=%s for bean %s ", t.logTag, ref, beanName)
					return nil, t.checkError(msg, throw)
				}
			}

		}
	}

	t.methodMap[typeName] = &m
	return &m, nil

}

// Injection 依赖注入
func (t *Spring) injection(beanName string, ins interface{}, throw bool) error {

	log := t.logger
	value := reflect.ValueOf(ins)
	realPtrValue := value
	realValue := value.Elem()
	maxLevel := 10
	// interface{} , ptr ,struct
	{ //find the struct
		for level := 0; level < maxLevel && realValue.Kind() != reflect.Struct; level++ {
			realPtrValue = realValue
			realValue = realValue.Elem()
		}
	}

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

						msg := fmt.Sprint(t.logTag, "@autoInjection ", beanName, " Error: please defind function ", name, " for ", structName)
						return t.checkError(msg, throw)

					}
				}

			} else {
				msg := fmt.Sprintf("%s @autoInjection error: do not exist ref=%s for bean %s ", t.logTag, ref, beanName)
				return t.checkError(msg, throw)
			}
		}

	}
	return nil
}

func (t *Spring) before() {

	log := t.logger
	for _, _ins := range t.beforeModules {
		ins := *_ins
		name := ins.BeanName()
		_, ok := t.started.Load(name)
		if !ok {
			ins.Before()
			if t.debug {
				log.Printf("%s @before run %s.Before() ok ", t.logTag, ins.BeanName())
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
			name := ins.BeanName()
			_, ok := t.started.Load(name)
			if !ok {
				wg.Add(1)
				if t.debug {
					log.Printf("%s [Parallel Function] run %s.Start() ", t.logTag, ins.BeanName())
				}
				ins.Start(wg)
				t.started.Store(name, true)
				if t.debug {
					log.Printf("%s [Parallel Function] finish %s.Start() ", t.logTag, ins.BeanName())
				}
			} else {
				if t.debug {
					log.Printf("%s [Parallel Function]  %s.Start() had called before! ", t.logTag, ins.BeanName())
				}
			}

		}
		wg.Wait()
	}
}
func (t *Spring) start() {
	log := t.logger
	for _, _ins := range t.startModules {
		ins := *_ins
		name := ins.BeanName()
		_, ok := t.started.Load(name)
		if !ok {
			ins.Start()
			t.started.Store(name, true)
			if t.debug {
				log.Printf("%s @start run  %s.Start() ok ", t.logTag, ins.BeanName())
			}
		} else {
			if t.debug {
				log.Printf("%s @start  %s.Start() had called before. ", t.logTag, ins.BeanName())
			}
		}
	}
}

// 上下文
func (t *Spring) GetContext() SpringContext {
	return t.ctx
}

// Start run
func (t *Spring) Start() {

	if !t.inited {
		t.Init()
	}
	t.count++
	if t.debug {
		t.logger.Printf("%s @Start start count=%d ", t.logTag, t.count)
	}
	t.lock.Lock()
	defer t.lock.Unlock()

	t.before()
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
