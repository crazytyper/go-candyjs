package candyjs

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"
	"unsafe"

	"github.com/crazytyper/go-cesu8"
	"github.com/crazytyper/go-duktape"
)

const goProxyPtrProp = "\xff" + "goProxyPtrProp"

var typeTime = reflect.TypeOf(time.Time{})

// Context represents a Duktape thread and its call and value stacks.
type Context struct {
	storage      *storage
	lastGoError  error
	errorFactory ErrorFactoryFunc
	*duktape.Context
}

// NewContext returns a new Context
func NewContext() *Context {
	ctx := &Context{Context: duktape.New()}
	ctx.storage = newStorage()
	ctx.pushGlobalCandyJSObject()

	return ctx
}

// ErrorFactoryFunc ...
type ErrorFactoryFunc func(ctx *Context, index int) error

// SetErrorFactory sets the function used to create go errors from Javascript execptions.
// By default `errors.New(execption.toString())` will be used.
func (ctx *Context) SetErrorFactory(f ErrorFactoryFunc) {
	ctx.errorFactory = f
}

func (ctx *Context) pushGlobalCandyJSObject() {
	ctx.PushGlobalObject()
	ctx.PushObject()
	ctx.PushObject()
	ctx.PutPropString(-2, "_functions")
	ctx.PushGoFunction(func(pckgName string) error {
		return ctx.pushPackage(pckgName)
	})
	ctx.PutPropString(-2, "require")
	ctx.PutPropString(-2, "CandyJS")
	ctx.Pop()

	ctx.EvalString(`CandyJS._call = function(ptr, args) {
		return CandyJS._functions[ptr].apply(null, args)
	}`)

	ctx.EvalString(`CandyJS.proxy = function(func) {
		ptr = Duktape.Pointer(func);
		CandyJS._functions[ptr] = func;

		return ptr;
	}`)
}

// SetRequireFunction sets the modSearch function into the Duktape JS object
// http://duktape.org/guide.html#builtin-duktape-modsearch-modloade
func (ctx *Context) SetRequireFunction(f interface{}) int {
	ctx.PushGlobalObject()
	ctx.GetPropString(-1, "Duktape")
	idx := ctx.PushGoFunction(f)
	ctx.PutPropString(-2, "modSearch")
	ctx.Pop()

	return idx
}

// PushGlobalType like PushType but pushed to the global object
func (ctx *Context) PushGlobalType(name string, s interface{}) int {
	ctx.PushGlobalObject()
	cons := ctx.PushType(s)
	ctx.PutPropString(-2, name)
	ctx.Pop()

	return cons
}

// PushType push a constructor for the type of the given value, this constructor
// returns an empty instance of the type. The value passed is discarded, only
// is used for retrieve the type, instead of require pass a `reflect.Type`.
func (ctx *Context) PushType(s interface{}) int {
	return ctx.PushGoFunction(func() {
		value := reflect.New(reflect.TypeOf(s))
		ctx.PushProxy(value.Interface())
	})
}

// PushGlobalProxy like PushProxy but pushed to the global object
func (ctx *Context) PushGlobalProxy(name string, v interface{}) int {
	ctx.PushGlobalObject()
	obj := ctx.PushProxy(v)
	ctx.PutPropString(-2, name)
	ctx.Pop()

	return obj
}

// PushProxy push a proxified pointer of the given value to the stack, this
// refence will be stored on an internal storage. The pushed objects has
// the exact same methods and properties from the original value.
// http://duktape.org/guide.html#virtualization-proxy-object
func (ctx *Context) PushProxy(v interface{}) int {

	ptr := ctx.storage.add(v)

	proxy, ok := v.(Proxy)
	if !ok {
		proxy = p // fallback to the default proxy that uses package reflect
	}

	obj := ctx.PushObject()
	ctx.PushPointer(ptr)
	ctx.PutPropString(-2, goProxyPtrProp)

	ctx.PushGlobalObject()
	ctx.GetPropString(-1, "Proxy")
	ctx.Dup(obj)

	ctx.PushObject()
	ctx.PushGoFunction(proxy.Enumerate)
	ctx.PutPropString(-2, "enumerate")
	ctx.PushGoFunction(proxy.Enumerate)
	ctx.PutPropString(-2, "ownKeys")
	ctx.PushGoFunction(proxy.Get)
	ctx.PutPropString(-2, "get")
	ctx.PushGoFunction(proxy.Set)
	ctx.PutPropString(-2, "set")
	ctx.PushGoFunction(proxy.Has)
	ctx.PutPropString(-2, "has")
	ctx.New(2)

	ctx.Remove(-2)
	ctx.Remove(-2)

	ctx.PushPointer(ptr)
	ctx.PutPropString(-2, goProxyPtrProp)

	return obj
}

// PushGlobalStruct like PushStruct but pushed to the global object
func (ctx *Context) PushGlobalStruct(name string, s interface{}) (int, error) {
	ctx.PushGlobalObject()
	obj, err := ctx.PushStruct(s)
	if err != nil {
		return -1, err
	}

	ctx.PutPropString(-2, name)
	ctx.Pop()

	return obj, nil
}

// PushStruct push a object to the stack with the same methods and properties
// the pushed object is a copy, any change made on JS is not reflected on the
// Go instance.
func (ctx *Context) PushStruct(s interface{}) (int, error) {
	t := reflect.TypeOf(s)
	v := reflect.ValueOf(s)

	obj := ctx.PushObject()
	ctx.pushStructMethods(obj, t, v)

	if t.Kind() == reflect.Ptr {
		v = v.Elem()
		t = v.Type()
	}

	return obj, ctx.pushStructFields(obj, t, v)
}

func (ctx *Context) pushStructFields(obj int, t reflect.Type, v reflect.Value) error {
	fCount := t.NumField()
	for i := 0; i < fCount; i++ {
		value := v.Field(i)

		if value.Kind() != reflect.Ptr || !value.IsNil() {
			fieldName := t.Field(i).Name
			if !isExported(fieldName) {
				continue
			}

			if err := ctx.pushValue(value); err != nil {
				return err
			}

			ctx.PutPropString(obj, nameToJavaScript(fieldName))
		}
	}

	return nil
}

func (ctx *Context) pushStructMethods(obj int, t reflect.Type, v reflect.Value) {
	mCount := t.NumMethod()
	for i := 0; i < mCount; i++ {
		methodName := t.Method(i).Name
		if !isExported(methodName) {
			continue
		}

		ctx.PushGoFunction(v.Method(i).Interface())
		ctx.PutPropString(obj, nameToJavaScript(methodName))

	}
}

// PushGlobalInterface like PushInterface but pushed to the global object
func (ctx *Context) PushGlobalInterface(name string, v interface{}) error {
	return ctx.pushGlobalValue(name, reflect.ValueOf(v))
}

// PushInterface push any type of value to the stack, the following types are
// supported:
//  - Bool
//  - Int, Int8, Int16, Int32, Uint, Uint8, Uint16, Uint32 and Uint64
//  - Float32 and Float64
//  - Strings and []byte
//  - Structs
//  - Functions with any signature
//
// Please read carefully the following notes:
//  - The pointers are resolved and the value is pushed
//  - Structs are pushed ussing PushProxy, if you want to make a copy use PushStruct
//  - Int64 and UInt64 are supported but before push it to the stack are casted
//    to float64
//  - Any unsuported value is pushed as a null
func (ctx *Context) PushInterface(v interface{}) error {
	return ctx.pushValue(reflect.ValueOf(v))
}

func (ctx *Context) pushGlobalValue(name string, v reflect.Value) error {
	ctx.PushGlobalObject()
	if err := ctx.pushValue(v); err != nil {
		return err
	}

	ctx.PutPropString(-2, name)
	ctx.Pop()

	return nil
}

func (ctx *Context) pushValue(v reflect.Value) error {
	if !v.IsValid() {
		ctx.PushNull()
		return nil
	}

	switch v.Kind() {
	case reflect.Interface:
		return ctx.pushValue(v.Elem())
	case reflect.Bool:
		ctx.PushBoolean(v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		ctx.PushInt(int(v.Int()))
	case reflect.Int64: //Caveat: lose of precession casting to float64
		ctx.PushNumber(float64(v.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		ctx.PushUint(uint(v.Uint()))
	case reflect.Uint64: //Caveat: lose of precession casting to float64
		ctx.PushNumber(float64(v.Uint()))
	case reflect.Float64:
		ctx.PushNumber(v.Float())
	case reflect.String:
		ctx.PushString(string(cesu8.EncodeString(v.String())))
	case reflect.Struct:
		// also use `Date` for `time.Time` aliases
		tv := v.Type()
		if tv != typeTime && tv.ConvertibleTo(typeTime) {
			v = v.Convert(typeTime)
		}

		switch i := v.Interface().(type) {
		case time.Time:
			// new Date(...)
			ctx.PushGlobalObject()
			ctx.GetPropString(-1, "Date")
			ctx.PushNumber(float64(i.Unix()*1000 + int64(i.Nanosecond()/int(time.Millisecond))))
			ctx.New(1)
			ctx.Remove(-2) // remove the global object from the stack

		default:
			ctx.PushProxy(i)
		}

	case reflect.Func:
		ctx.PushGoFunction(v.Interface())
	case reflect.Ptr:
		if v.Elem().Kind() == reflect.Struct {
			ctx.PushProxy(v.Interface())
			return nil
		}

		return ctx.pushValue(v.Elem())
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			ctx.PushString(string(v.Interface().([]byte)))
			return nil
		}

		arr := ctx.PushArray()
		for i := 0; i < v.Len(); i++ {
			av := v.Index(i)
			if err := ctx.pushValue(av); err != nil {
				return err
			}
			ctx.PutPropIndex(arr, uint(i))
		}
		return nil

	case reflect.Map:
		obj := ctx.PushObject()
		for _, key := range v.MapKeys() {
			if err := ctx.pushValue(key); err != nil {
				return err
			}
			if err := ctx.pushValue(v.MapIndex(key)); err != nil {
				return err
			}
			ctx.PutProp(obj)
		}
		return nil

	default:
		js, err := json.Marshal(v.Interface())
		if err != nil {
			return err
		}

		ctx.PushString(string(cesu8.EncodeString(string(js))))
		ctx.JsonDecode(-1)
	}

	return nil
}

func (ctx *Context) pushGlobalValues(name string, vs []reflect.Value) error {
	ctx.PushGlobalObject()
	if err := ctx.pushValues(vs); err != nil {
		return err
	}

	ctx.PutPropString(-2, name)
	ctx.Pop()

	return nil
}

func (ctx *Context) pushValues(vs []reflect.Value) error {
	arr := ctx.PushArray()
	for i, v := range vs {
		if err := ctx.pushValue(v); err != nil {
			return err
		}

		ctx.PutPropIndex(arr, uint(i))
	}

	return nil
}

// PushGlobalGoFunction like PushGoFunction but pushed to the global object
func (ctx *Context) PushGlobalGoFunction(name string, f interface{}) (int, error) {
	return ctx.Context.PushGlobalGoFunction(name, ctx.wrapFunction(f))
}

// PushGoFunction push a native Go function of any signature to the stack.
// A pointer to the function is stored in the internals of the context and
// collected by the duktape GC removing any reference in Go also.
//
// The most common types are supported as input arguments, also the variadic
// functions can be used.
//
// You can use JS functions as arguments but you should wrapper it with the
// helper `CandyJS.proxy`. Example:
// 	ctx.PushGlobalGoFunction("test", func(fn func(int, int) int) {
//		...
//	})
//
//	ctx.PevalString(`test(CandyJS.proxy(function(a, b) { return a * b; }));`)
//
// The structs can be delivered to the functions in three ways:
//  - In-line representation as plain JS objects: `{'int':42}`
//  - Using a previous pushed type using `PushGlobalType`: `new MyModel`
//  - Using a previous pushed instance using `PushGlobalProxy`
//
// All other types are loaded into Go using `json.Unmarshal` internally
//
// The following types are not supported chans, complex64 or complex128, and
// the types rune, byte and arrays are not tested.
//
// The returns are handled in the following ways:
//  - The result of functions with a single return value like `func() int` is
//    pushed directly to the stack.
//  - Functions with a n return values like `func() (int, int)` are pushed as
//    an array. The errors are removed from this array.
//  - Returns of functions with a trailling error like `func() (string, err)`:
//    if err is not nil an error is throw in the context, and the other values
//    are discarded. IF err is nil, the values are pushed to the stack, following
//    the previuos rules.
//
// All the non erros returning values are pushed following the same rules of
// `PushInterface` method
func (ctx *Context) PushGoFunction(f interface{}) int {
	return ctx.Context.PushGoFunction(ctx.wrapFunction(f))
}

// GetValue gets and marshals the value at the specified stack index.
// Takes special care for proxies.
func (ctx *Context) GetValue(index int, value interface{}) error {
	t := reflect.ValueOf(value).Elem()
	if proxy := ctx.getProxy(index); proxy != nil {
		t.Set(reflect.ValueOf(proxy)) // return the value as is
		return nil
	}

	js := ctx.JsonEncode(index)
	if len(js) == 0 {
		t.Set(reflect.Zero(t.Type()))
		return nil
	}

	err := json.Unmarshal([]byte(js), value)
	if err != nil {
		return err
	}
	return nil
}

// LastGoError returns the last error returned by a GO function.
func (ctx *Context) LastGoError() error {
	return ctx.lastGoError
}

// ConsumeLastGoError returns the last error returned by a GO function and clears it.
func (ctx *Context) ConsumeLastGoError() error {
	err := ctx.lastGoError
	ctx.lastGoError = nil
	return err
}

func (ctx *Context) wrapFunction(f interface{}) func(ctx *duktape.Context) int {
	tbaContext := ctx
	return func(ctx *duktape.Context) int {
		args := tbaContext.getFunctionArgs(f)
		return tbaContext.callFunction(f, args)
	}
}

func (ctx *Context) getFunctionArgs(f interface{}) []reflect.Value {
	def := reflect.ValueOf(f).Type()
	isVariadic := def.IsVariadic()
	inCount := def.NumIn()

	top := ctx.GetTopIndex()

	var args []reflect.Value
	for index := 0; index <= top; index++ {
		var t reflect.Type
		if (index+1) < inCount || (index < inCount && !isVariadic) {
			t = def.In(index)
		} else if isVariadic {
			t = def.In(inCount - 1).Elem()
		}

		args = append(args, ctx.getValueFromContext(index, t))
	}

	//Optional args
	argc := len(args)
	if inCount > argc {
		for i := argc; i < inCount; i++ {
			//Avoid send empty slice when variadic
			if isVariadic && i-1 < inCount {
				break
			}

			args = append(args, reflect.Zero(def.In(i)))
		}
	}

	return args
}

func (ctx *Context) getValueFromContext(index int, t reflect.Type) reflect.Value {
	if proxy := ctx.getProxy(index); proxy != nil {
		return reflect.ValueOf(proxy)
	}

	if ctx.IsFunction(index) && t.Kind() == reflect.Func {
		return reflect.MakeFunc(t,
			func(args []reflect.Value) (results []reflect.Value) {
				// Bring the function back to the top of the stack
				ctx.Dup(index)

				// Followed by the arguments passed to it
				for _, v := range args {
					if err := ctx.pushValue(v); err != nil {
						ctx.PushUndefined()
					}
				}

				// Pcall replaces the function and args on the stack with the return value.
				// Be a good citizen and clear the return value from the stack.
				// http://duktape.org/api.html#duk_pcall
				defer ctx.Pop()

				if ret := ctx.Pcall(len(args)); ret != duktape.ExecSuccess {
					return ctx.getCallResultError(t, ctx.getError(-1))
				}

				return ctx.getCallResult(t)
			},
		)
	}

	if ctx.IsPointer(index) {
		return ctx.getFunction(index, t)
	}

	if ctx.IsError(index) && t.Kind() == reflect.Interface {
		return reflect.ValueOf(ctx.getError(index))
	}

	return ctx.getValueUsingJSON(index, t)
}

func (ctx *Context) getProxy(index int) interface{} {
	if !ctx.IsObject(index) {
		return nil
	}

	ptr := ctx.getProxyPtrProp(index)
	if ptr == nil {
		return nil
	}

	return ctx.storage.get(ptr)
}

func (ctx *Context) getError(index int) error {
	factory := ctx.errorFactory
	if factory != nil {
		return factory(ctx, index)
	}
	return errors.New(ctx.SafeToString(index))
}

func (ctx *Context) getFunction(index int, t reflect.Type) reflect.Value {
	ptr := ctx.GetPointer(index)

	return reflect.MakeFunc(t, ctx.wrapDuktapePointer(ptr, t))
}

func (ctx *Context) wrapDuktapePointer(
	ptr unsafe.Pointer,
	t reflect.Type,
) func(in []reflect.Value) []reflect.Value {
	return func(in []reflect.Value) []reflect.Value {
		ctx.PushGlobalObject()
		ctx.GetPropString(-1, "CandyJS")
		obj := ctx.NormalizeIndex(-1)
		ctx.PushString("_call")
		ctx.PushPointer(ptr)
		ctx.pushValues(in)
		ctx.CallProp(obj, 2)

		return ctx.getCallResult(t)
	}
}

func (ctx *Context) getCallResult(t reflect.Type) []reflect.Value {
	count := t.NumOut()
	result := make([]reflect.Value, 0, count)
	if count <= 0 {
		return result
	}

	lastOut := t.Out(count - 1)
	hasErrorArg := lastOut.Implements(errorInterface)
	if hasErrorArg {
		count--
	}

	if count == 0 {
		// returns just an error
	} else if count == 1 {
		result = append(result, ctx.getValueFromContext(-1, t.Out(0)))
	} else {
		actualCount := ctx.GetLength(-1)
		if actualCount != count {
			panic(fmt.Sprintf("Invalid count of return value on proxied function. Expected %d had %d", actualCount, count))
		}

		idx := ctx.NormalizeIndex(-1)
		for i := 0; i < count; i++ {
			ctx.GetPropIndex(idx, uint(i))
			result = append(result, ctx.getValueFromContext(-1, t.Out(i)))
		}
	}
	if hasErrorArg {
		result = append(result, reflect.Zero(lastOut))
	}
	return result
}

var errorInterface = reflect.TypeOf((*error)(nil)).Elem()

func (ctx *Context) getCallResultError(t reflect.Type, err error) []reflect.Value {
	count := t.NumOut()
	if count < 1 {
		panic(err)
	}

	out := t.Out(count - 1)
	if !out.Implements(errorInterface) {
		panic(err)
	}

	result := make([]reflect.Value, count)
	for i := 0; i < count-1; i++ {
		result[i] = reflect.Zero(t.Out(i))
	}
	result[count-1] = reflect.ValueOf(&err).Elem()
	return result
}

func (ctx *Context) getProxyPtrProp(index int) unsafe.Pointer {
	defer ctx.Pop()
	ctx.GetPropString(index, goProxyPtrProp)
	if !ctx.IsPointer(-1) {
		return nil
	}

	return ctx.GetPointer(-1)
}

func (ctx *Context) getValueUsingJSON(index int, t reflect.Type) reflect.Value {
	v := reflect.New(t).Interface()

	js := ctx.JsonEncode(index)
	if len(js) == 0 {
		return reflect.Zero(t)
	}

	err := json.Unmarshal([]byte(js), v)
	if err != nil {
		panic(err)
	}

	return reflect.ValueOf(v).Elem()
}

func (ctx *Context) callFunction(f interface{}, args []reflect.Value) int {
	ctx.lastGoError = nil

	var err error
	out := reflect.ValueOf(f).Call(args)
	out, err = ctx.handleReturnError(out)

	if err != nil {
		ctx.lastGoError = err
		return duktape.ErrRetError
	}

	ctx.lastGoError = nil

	if len(out) == 0 {
		return 1
	}

	if len(out) > 1 {
		err = ctx.pushValues(out)
	} else {
		err = ctx.pushValue(out[0])
	}

	if err != nil {
		return duktape.ErrRetInternal
	}

	return 1
}

func (ctx *Context) handleReturnError(out []reflect.Value) ([]reflect.Value, error) {
	c := len(out)
	if c == 0 {
		return out, nil
	}

	last := out[c-1]
	if last.Type().Name() == "error" {
		if !last.IsNil() {
			return nil, last.Interface().(error)
		}

		return out[:c-1], nil
	}

	return out, nil
}
