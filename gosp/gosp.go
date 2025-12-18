package gosp

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"runtime"
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
	AsyncStart()
	BeanName() string
}

// Bean
type Bean interface {
	BeanName() string
}

type AutoInjectBean interface {
	InjectBean(any) bool
}

// Spring
type Spring struct {
	instances     map[string]Bean
	startModules  map[string]StartBean
	beforeModules map[string]BeforeBean
	syncModules   map[string]SyncModuleBean
	methodMap     map[string]*BeanMap
	injectBeanMap map[string]AutoInjectBean
	started       sync.Map
	debug         bool
	logTag        string
	inited        bool
	logger        Logger
	lock          sync.Locker
	ctx           SpringContext
	count         int
	once          sync.Once
	instanceCount int
	// applicationListenerType reflect.Type
}

type SpringContext interface {
	Get(name string) Bean
	Add(interface{}) interface{}
	CreateInstance(ins interface{}) (interface{}, error)
	GetSyncModule(name string) SyncModuleBean
	DispathEvent(event ApplicationEvent)
	DispathEventSync(event ApplicationEvent)
	AddListener(name string, listener any) error
	RemoveListener(name string, listener any) error
}

type ApplicationEvent interface {
	Name() string //Event Name
}

type contextImpl struct {
	spring *Spring
	mu     sync.RWMutex
	events map[string]([]any)
}

// PublicEvent 方法用于异步发布公共事件
// 参数:
//	event ApplicationEvent - 待发布的事件

func (t *contextImpl) DispathEvent(event ApplicationEvent) {
	t.dispathEvent(event, false)
}

// PublicEventSync 方法用于同步发布公共事件
// 参数:
//
//	event ApplicationEvent - 待发布的事件
func (t *contextImpl) DispathEventSync(event ApplicationEvent) {
	t.dispathEvent(event, true)
}

func (t *contextImpl) getListener(name string) []any {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.events != nil {
		ret, ok := t.events[name]
		if ok {
			return ret
		}
	}
	return []any{}
}

func (t *contextImpl) dispathEvent(event ApplicationEvent, sync bool) {
	listeners := t.getListener(event.Name())
	defer func() {
		if r := recover(); r != nil {
			t.spring.logger.Println("gosp@dispathEvent recover err:", r)
		}
	}()
	for _, listener := range listeners {
		val := reflect.ValueOf(listener)
		if sync {
			val.Call([]reflect.Value{reflect.ValueOf(event)})
		} else {
			go func() {
				defer func() {
					if r := recover(); r != nil {
						t.spring.logger.Println("gosp@dispathEvent asyn recover err:", r)
					}
				}()
				val.Call([]reflect.Value{reflect.ValueOf(event)})
			}()
		}
	}

}

func (t *contextImpl) AddListener(name string, listener any) error {

	if reflect.ValueOf(listener).Kind() != reflect.Func {
		typeName := reflect.TypeOf(listener).Elem().Name()
		t.spring.logger.Printf("@AddListener Error: listener must be func,name=%s", typeName)
		return fmt.Errorf("listener must be func")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.events == nil {
		t.events = make(map[string]([]any))
	}

	if listeners, ok := t.events[name]; ok {
		t.events[name] = append(listeners, listener)
	} else {
		t.events[name] = []any{listener}
	}

	t.spring.debugMessagef("AddListener %s", name)

	return nil

}

func (t *contextImpl) RemoveListener(name string, listener any) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.events == nil {
		return nil
	}
	if listeners, ok := t.events[name]; ok {
		for i, l := range listeners {
			if reflect.TypeOf(l).Elem().Name() == reflect.TypeOf(listener).Elem().Name() {
				t.events[name] = append(listeners[:i], listeners[i+1:]...)
			}
		}
	}
	return nil
}
func (t *contextImpl) Get(name string) Bean {

	return t.spring.Get(name)
}
func (t *contextImpl) Add(ins interface{}) interface{} {

	t.spring.Add(ins)
	return ins
}

func (t *contextImpl) CreateInstance(ins interface{}) (interface{}, error) {
	return t.spring.CreateInstance(ins)
}

func (t *contextImpl) GetSyncModule(name string) SyncModuleBean {
	return t.spring.GetSyncModule(name)
}

func (t *contextImpl) BeanName() string {
	return DefaultContextName
}
func (t *contextImpl) InjectBean() bool {
	return false
}

type Logger interface {
	Println(...interface{})
	Fatal(...interface{})
	Printf(string, ...interface{})
	Fatalf(string, ...interface{})
}

type BeanMethod struct {
	Method    string
	Tag       string
	ValName   string
	Index     int
	IsSetter  bool
	IsInject  bool
	Arg       reflect.Value
	InjectArg Bean
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

		t.instances = make(map[string]Bean)
		t.startModules = make(map[string]StartBean)
		t.beforeModules = make(map[string]BeforeBean)
		t.syncModules = make(map[string]SyncModuleBean)
		t.methodMap = make(map[string]*BeanMap)
		t.injectBeanMap = make(map[string]AutoInjectBean)
		if t.logger == nil {
			t.logger = &log.Logger{}
		}
		t.logTag = "[go-spring] "
		t.lock = &sync.Mutex{}

		ctx := contextImpl{spring: t, events: nil}

		t.ctx = &ctx
		var bean Bean = &ctx
		t.instances[DefaultContextName] = bean
		t.count = 0
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

	switch module := cls.(type) {
	case BeforeBean:
		// log.Println(t.logTag, "is BeforeBean ", tmp)
		{

			old, ok := t.beforeModules[module.BeanName()]
			isModule = true
			if ok && old != nil {
				log.Fatal(t.logTag, " Error: beforeModules exist old bean=", module.BeanName(), "old=", old)
			}
			t.beforeModules[module.BeanName()] = module

			t.debugMessagef(t.logTag, "Add beforeModule=", module.BeanName())

		}
	}

	switch module := cls.(type) {
	case StartBean:
		// log.Println(t.logTag, "is startModule ", tmp)
		{

			old, ok := t.startModules[module.BeanName()]
			isModule = true
			if ok && old != nil {
				log.Fatal(t.logTag, " Error: startModule exist old bean=", module.BeanName(), "old=", old)
			}
			t.startModules[module.BeanName()] = module

			t.debugMessage(t.logTag, "Add startModule=", module.BeanName())

		}

	case SyncModuleBean:
		// log.Println(t.logTag, "is SyncModuleBean ", module)
		{
			syncModule := module
			old, ok := t.startModules[syncModule.BeanName()]
			isModule = true
			if ok && old != nil {
				log.Fatal(t.logTag, " Error: syncModule exist old bean=", syncModule.BeanName(), "old=", old)
			}
			t.syncModules[syncModule.BeanName()] = syncModule
			t.debugMessage(t.logTag, "Add syncModule/bean=", syncModule.BeanName())
		}
	}

	typeName := clsType.Elem().String()

	if reflect.ValueOf(cls).IsNil() {
		log.Fatal(t.logTag, " Error: can not Add a nil var to spring! ", typeName, ", clsType is ", clsType)
	}

	switch bean := cls.(type) {
	case Bean:
		{

			old, ok := t.instances[bean.BeanName()]
			if ok && old != nil {
				log.Fatal(t.logTag, " Error: ", typeName, " exist old bean=", bean.BeanName(), "old=", old)
			}

			t.instances[bean.BeanName()] = bean

			switch injectBean := cls.(type) {
			case AutoInjectBean:
				t.injectBeanMap[bean.BeanName()] = injectBean
				t.debugMessage(t.logTag, "Add InjectBean =", bean.BeanName(), " typeName=", typeName)
			}

			if !isModule {
				t.debugMessage(t.logTag, "Add bean=", bean.BeanName(), " typeName=", typeName)
			}
		}
	default:
		{

			log.Fatal(t.logTag, " Error: the struct do not implement the BeanName() method ,struct=", typeName)

		}
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

// Add 函数向 SpringContext 类型的变量 t 中添加一个类型为 T 的实例 ins，并返回该实例
// T 表示泛型类型，可以是任意类型
// SpringContext 是一个接口，表示 Spring 上下文，用于管理 Spring 容器中的 bean
// ins 是要添加到 Spring 上下文中的实例
// 返回值是添加进 Spring 上下文的实例 ins
func Add[T any](t SpringContext, ins T) T {
	t.Add(ins)
	return ins
}

// CreateInstance create instance and  auto inject
// @param t
// @param ins
// @return T
// @return error
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
		return bean
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
		return module
	}
	return nil
}

// GetSyncModule get SyncModule by name
// GetSyncModule 根据模块名获取同步模块对象
//
// 参数：
//
//	name string - 同步模块的名称
//
// 返回值：
//
//	SyncModuleBean - 对应的同步模块对象，若不存在则返回nil
//
// 注意事项：
//
//	如果Spring实例尚未初始化，则会自动调用Init方法进行初始化
func (t *Spring) GetSyncModule(name string) SyncModuleBean {
	if !t.inited {
		t.Init()
	}
	syncModule, ok := t.syncModules[name]
	if ok && syncModule != nil {
		return syncModule
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

		err := t.injection(beanName, ins, true)
		if err != nil {
			t.logger.Fatalf("@autoInjection Error: %s", err.Error())
		}

	}
}

// CreateInstance 创建实例,并依赖
func (t *Spring) CreateInstance(ins interface{}) (interface{}, error) {

	t.instanceCount++

	typeName := reflect.ValueOf(ins).Type().String()

	if reflect.ValueOf(ins).IsNil() {

		return ins, fmt.Errorf("%s @Create Error:the instance(%s) is nil", t.logTag, typeName)
	}

	// 确保ins是指针类型
	ptr := reflect.ValueOf(ins)
	if ptr.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("%s @Create Error: the instance(%v) is not a pointer", t.logTag, ins)
	}
	rValue := ptr.Elem()

	var injectBean AutoInjectBean

	switch _injectBean := ins.(type) {
	case AutoInjectBean:
		injectBean = _injectBean
	}

	// get the mapper of method
	beanMap, err := t.getMethodMapper("", ins, injectBean, false)
	if err != nil {
		return ins, err
	}

	t.runMapper(beanMap, ptr, rValue, injectBean, typeName)
	// for _, m := range beanMap.Methods {
	// 	if m.IsSetter {
	// 		_fieldSet := ptr.Method(m.Index)
	// 		_fieldSet.Call([]reflect.Value{m.Arg})
	// 	} else {
	// 		_field := val.Field(m.Index)
	// 		_field.Set(m.Arg)
	// 	}
	// }

	return ins, nil
}

// Import 存放已经依赖好的实例
func (t *Spring) Import(ins Bean) error {
	name := ins.BeanName()
	t.instances[name] = ins
	t.started.Store(name, true)
	return nil
}

// checkError 检查错误,是否抛出异常
func (t *Spring) checkError(msg string, throw bool) error {
	if throw {
		t.logger.Fatal(msg)
		return nil
	} else {
		return errors.New(msg)
	}
}
func (t *Spring) debugMessage(args ...any) {
	if t.debug {
		t.logger.Println(args...)
	}
}
func (t *Spring) debugMessagef(format string, args ...any) {
	if t.debug {
		t.logger.Printf(format, args...)
	}
}

// getMethodMapper  get the mapper of struct
func (t *Spring) getMethodMapper(beanName string, ins interface{}, injectBean AutoInjectBean, throw bool) (*BeanMap, error) {

	insType := reflect.TypeOf(ins).Elem()
	typeName := fmt.Sprintf("%s/%s", insType.PkgPath(), insType.Name())

	supperSetter := t.isSupportSetter()

	method, ok := t.methodMap[typeName]
	if ok {
		return method, nil
	}
	m := BeanMap{}

	{

		value := reflect.ValueOf(ins)
		realPtrValue := value
		realValue := value.Elem()
		if beanName == "" {
			beanName = typeName
			t.debugMessage(t.logTag, "@getMethodMapper typeName=", beanName)
		}

		methodToIndex := make(map[string]int)
		typ := reflect.TypeOf(ins)

		if supperSetter {
			for i := 0; i < typ.NumMethod(); i++ {
				method := typ.Method(i)
				methodToIndex[method.Name] = i
			}
		}

		reflectType := realValue.Type()

		for i := 0; i < reflectType.NumField(); i++ {

			field := reflectType.Field(i)

			ref := field.Tag.Get("bean")
			if ref != "" {
				fieldName := field.Name
				refValue, ok := t.instances[ref]
				if ok {

					_field := realValue.FieldByName(fieldName)

					_type := _field.Type()

					newPtr := reflect.ValueOf(refValue)

					t.debugMessage(t.logTag, "@getMethodMap ", beanName, "try inject name=", fieldName, " ref=", ref, " type=", _type)

					tryInject := false

					setterName := ""
					if _field.CanSet() && supperSetter {

						method := BeanMethod{}
						method.Tag = ref
						method.ValName = fieldName
						method.Arg = newPtr.Convert(_type)
						method.Method = fieldName
						method.Index = i
						method.IsSetter = false
						method.IsInject = false
						m.Methods = append(m.Methods, method)

					} else if supperSetter {
						name := field.Name
						if len(name) <= 1 {
							name = "Set" + strings.ToUpper(name)
						} else {
							name = "Set" + strings.ToUpper(name[0:1]) + name[1:]
						}
						_fieldSet := realPtrValue.MethodByName(name)
						setterName = name
						if _fieldSet.IsValid() {

							method := BeanMethod{}
							method.Tag = ref
							method.ValName = fieldName
							method.Arg = newPtr
							method.Method = name
							method.Index = methodToIndex[name]
							method.IsSetter = true
							method.IsInject = false

							m.Methods = append(m.Methods, method)

						} else {

							tryInject = true

						}
					} else {
						tryInject = true
					}

					if tryInject {
						if injectBean != nil {
							// injectBean.InjectBean()
							method := BeanMethod{}
							method.Tag = ref
							method.ValName = fieldName
							method.InjectArg = refValue
							method.Method = "InjectBean"
							method.Index = 0
							method.IsSetter = false
							method.IsInject = true //使用injectBean的方式
							m.Methods = append(m.Methods, method)
						} else if supperSetter {
							structName := reflectType.Name()

							t.printSetterCode(structName, setterName, _type.String(), field.Name)
							fmt.Println("或者")
							t.printInjectCode(structName, _type.String(), field.Name, ref)

							msg := fmt.Sprint(t.logTag, "@getMethodMap ", beanName, " Error: please defind function ", setterName, " for ", structName)
							return nil, t.checkError(msg, throw)
						} else {
							structName := reflectType.Name()

							t.printInjectCode(structName, _type.String(), field.Name, ref)

							msg := fmt.Sprint(t.logTag, "@getMethodMap ", beanName, " Error: please defind function ", " for ", structName)
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

func (t *Spring) printSetterCode(structName, setterName, _type, field string) {
	fmt.Printf(`请添加以下代码到结构体%s :
	func (t *%s) %s(arg %s) {
	t.%s = arg
	}
	`, structName, structName, setterName, _type, field)
}

func (t *Spring) printInjectCode(structName, _type, field, ref string) {
	fmt.Printf(`请添加以下代码到结构体%s :
	func (t *%s) InjectBean(arg any) bool {
		switch val := arg.(type) {
		case %s:
			t.%s = val	//%s
		default:
			return false
		}
		return true
	}
	`, structName, structName, _type, field, ref)
}
func (t *Spring) isSupportSetter() bool {
	return runtime.GOARCH != "wasm"
}

func (t *Spring) isSupportWaitGroup() bool {
	return runtime.GOARCH != "wasm"
}

// Injection 依赖注入
// injection 函数为Spring结构体的方法，用于自动注入依赖项到给定的实例中
// beanName：待注入的bean的名称
// ins：需要被注入的实例
// throw：是否抛出异常，如果为true，则在出现错误时抛出异常
// 返回值：如果注入成功则返回nil，否则返回错误信息
func (t *Spring) injection(beanName string, ins interface{}, throw bool) error {

	ptr := reflect.ValueOf(ins)
	rValue := ptr.Elem()

	// 确保ins是指针类型
	if ptr.Kind() != reflect.Ptr {
		return fmt.Errorf("%s @injection Error: the instance(%v) is not a pointer", t.logTag, ins)
	}

	injectBean := t.injectBeanMap[beanName]

	// get the mapper of method
	beanMap, err := t.getMethodMapper(beanName, ins, injectBean, throw)
	if err != nil {
		return err
	}

	return t.runMapper(beanMap, ptr, rValue, injectBean, beanName)

}

// 运行注入方法
func (t *Spring) runMapper(beanMap *BeanMap, ptr reflect.Value, rValue reflect.Value, injectBean AutoInjectBean, beanName string) error {
	for _, m := range beanMap.Methods {
		if m.IsSetter {
			_fieldSet := ptr.Method(m.Index)
			_fieldSet.Call([]reflect.Value{m.Arg})
		} else if m.IsInject {

			if !injectBean.InjectBean(m.InjectArg) {
				t.debugMessagef("%s Error: %s.InjectBean(%s),require true,but false", t.logTag, beanName, m.ValName)
				return fmt.Errorf("%s.InjectBean(%s) ,tag=%s,require true,but false", beanName, m.ValName, m.Tag)
			} else {
				t.debugMessage(t.logTag, "InjectBean(", m.ValName, ") ok for", beanName)
			}
		} else {
			_field := rValue.Field(m.Index)
			t.debugMessage(t.logTag, "try set "+beanName+"."+m.ValName, "=", m.Arg)
			_field.Set(m.Arg)
		}
	}
	return nil
}

func (t *Spring) before() {

	for _, _ins := range t.beforeModules {
		ins := _ins
		name := ins.BeanName()
		_, ok := t.started.Load(name)
		if !ok {
			ins.Before()
			t.debugMessagef("%s @before run %s.Before() ok ", t.logTag, ins.BeanName())

		}
	}
}
func (t *Spring) syncStart() {

	if len(t.syncModules) > 0 {
		if t.isSupportWaitGroup() {
			wg := &sync.WaitGroup{}
			for _, _ins := range t.syncModules {
				ins := _ins
				name := ins.BeanName()
				_, ok := t.started.Load(name)
				if !ok {
					wg.Add(1)
					t.debugMessagef("%s [Parallel Function] run %s.Start() ", t.logTag, ins.BeanName())
					go func() {
						defer wg.Done()
						ins.AsyncStart()
					}()
					t.started.Store(name, true)
					t.debugMessagef("%s [Parallel Function] finish %s.Start() ", t.logTag, ins.BeanName())
				} else {
					t.debugMessagef("%s [Parallel Function]  %s.Start() had called before! ", t.logTag, ins.BeanName())
				}
			}
			wg.Wait()
		} else {
			//不支持异步
			for _, _ins := range t.syncModules {
				ins := _ins
				name := ins.BeanName()
				_, ok := t.started.Load(name)
				if !ok {
					t.debugMessagef("%s [Parallel to Normal] run %s.Start() ", t.logTag, ins.BeanName())
					ins.AsyncStart()
					t.started.Store(name, true)
					t.debugMessagef("%s [Parallel to Normal] finish %s.Start() ", t.logTag, ins.BeanName())
				} else {
					t.debugMessagef("%s [Parallel to Normal]  %s.Start() had called before! ", t.logTag, ins.BeanName())

				}
			}
		}
	}
}
func (t *Spring) start() {

	// defer func() {
	// 	// t.debugMessagef("%s @start end", t.logTag)
	// 	if err := recover(); err != nil {
	// 		t.debugMessagef("%s @start err:%v", t.logTag, err)
	// 	}
	// }()

	for _, _ins := range t.startModules {
		ins := _ins
		name := ins.BeanName()
		_, ok := t.started.Load(name)
		if !ok {
			t.debugMessagef("%s @start try run  %s.Start() ", t.logTag, ins.BeanName())
			ins.Start()
			t.started.Store(name, true)
			// t.debugMessagef("%s @start run  %s.Start() ok ", t.logTag, ins.BeanName())
		} else {
			t.debugMessagef("%s @start  %s.Start() had called before. ", t.logTag, ins.BeanName())
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

	t.debugMessagef("%s @Start start count=%d ", t.logTag, t.count)

	t.lock.Lock()
	defer t.lock.Unlock()

	t.before()
	t.autoInjection()
	t.debugMessagef("%s @Start  finish autoInjection ", t.logTag)

	if t.isSupportWaitGroup() {
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
	} else {
		t.syncStart()
		t.start()
	}

	t.debugMessagef("%s @Start finish count=%d ", t.logTag, t.count)

}
