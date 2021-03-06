package gdnative

/*
#include <cgo_gateway_class.h>
#include <nativescript.wrapper.gen.h>
#include <gdnative.wrapper.gen.h>
#include <stdlib.h>
#include <string.h>
*/
import "C"
import (
	"reflect"
	"unsafe"

	"github.com/godot-go/godot-go/pkg/log"
)

var (
	wrappedSetClassNameMethodBind *C.godot_method_bind
	wrappedSetLibraryMethodBind   *C.godot_method_bind
	wrappedSetScriptMethodBind    *C.godot_method_bind
	nilptr                        = unsafe.Pointer(uintptr(0))
	strNativeScript               *C.char
)

func init() {
	registerInternalInitCallback(wrappedInitCallback)
	registerInternalTerminateCallback(wrappedTerminateCallback)
}

// Wrapped is used to decorate the godot_object with godot-go specific binding information.
type Wrapped interface {
	GetOwnerObject() *GodotObject
	GetTypeTag() TypeTag

	setOwnerObject(owner *GodotObject)
	setTypeTag(tt TypeTag)
}

// WrappedImpl implements Wrapped
type WrappedImpl struct {
	Owner   *GodotObject
	TypeTag TypeTag

	UserDataIdentifiableImpl
}

// GetOwnerObject returns the underlying GodotObject.
func (w *WrappedImpl) GetOwnerObject() *GodotObject {
	return w.Owner
}

// GetTypeTag returns the TypeTag for the class.
func (w *WrappedImpl) GetTypeTag() TypeTag {
	return w.TypeTag
}

func (w *WrappedImpl) setOwnerObject(owner *GodotObject) {
	w.Owner = owner
}

func (w *WrappedImpl) setTypeTag(tt TypeTag) {
	w.TypeTag = tt
}

func wrappedInitCallback() {
	strNativeScript = C.CString("NativeScript")

	// Ported from godot-cpp: https://github.com/godotengine/godot-cpp/blob/master/include/core/Godot.hpp#L39
	// these are static members in create_custom_class_instance()
	strSetClassName := C.CString("set_class_name")
	defer C.free(unsafe.Pointer(strSetClassName))
	wrappedSetClassNameMethodBind = (*C.godot_method_bind)(unsafe.Pointer(C.go_godot_method_bind_get_method(CoreApi, strNativeScript, strSetClassName)))

	if wrappedSetClassNameMethodBind == nil {
		log.Debug("failed to initialize method bind wrappedNativeScriptSetClassNameMethodBind")
	}

	strSetLibrary := C.CString("set_library")
	defer C.free(unsafe.Pointer(strSetLibrary))
	wrappedSetLibraryMethodBind = (*C.godot_method_bind)(unsafe.Pointer(C.go_godot_method_bind_get_method(CoreApi, strNativeScript, strSetLibrary)))

	if wrappedSetLibraryMethodBind == nil {
		log.Debug("failed to initialize method bind wrappedNativeScriptSetLibraryMethodBind")
	}

	strObject := C.CString("Object")
	defer C.free(unsafe.Pointer(strObject))
	strSetScript := C.CString("set_script")
	defer C.free(unsafe.Pointer(strSetScript))
	wrappedSetScriptMethodBind = (*C.godot_method_bind)(unsafe.Pointer(C.go_godot_method_bind_get_method(CoreApi, strObject, strSetScript)))
}

func wrappedTerminateCallback() {
	C.free(unsafe.Pointer(strNativeScript))
}

// GetCustomClassInstanceWithOwner creates a NativeScriptClass instance from the specified GodotObject.
// This is used to create constructors for NativeScriptClass instances.
//
func GetCustomClassInstanceWithOwner(owner *GodotObject) NativeScriptClass {
	ud := (UserData)(uintptr(C.go_godot_nativescript_get_userdata(NativescriptApi, unsafe.Pointer(owner))))

	classInst, ok := nativeScriptInstanceMap[ud]

	if !ok {
		log.Panic("unable to find NativeScriptClass instance")
	}

	return classInst
}

// CreateCustomClassInstance creates an NativeScriptClass instance that matches the
// specified class name and base class name.
func CreateCustomClassInstance(className, baseClassName string) NativeScriptClass {
	// Comments and code ported from godot-cpp: https://github.com/godotengine/godot-cpp/blob/master/include/core/Godot.hpp#L39

	// Usually, script instances hold a reference to their NativeScript resource.
	// that resource is obtained from a `.gdns` file, which in turn exists because
	// of the resource system of Godot. We can't cleanly hardcode that here,
	// so the easiest for now (though not really clean) is to create new resource instances,
	// individually attached to the script instances.

	// We cannot use wrappers because of https://github.com/godotengine/godot/issues/39181
	//	godot::NativeScript *script = godot::NativeScript::_new();
	//	script->set_library(get_wrapper<godot::GDNativeLibrary>((godot_object *)godot::gdnlib));
	//	script->set_class_name(T::___get_type_name());

	// So we use the C API directly.

	// TODO: determine if we need to call: Free(unsafe.Pointer(script))
	script := (*C.godot_object)(C.go_godot_get_class_constructor_new(CoreApi, strNativeScript))

	setLibraryArgs := [...]unsafe.Pointer{unsafe.Pointer(GDNativeLibObject)}

	C.go_godot_method_bind_ptrcall(
		CoreApi,
		wrappedSetLibraryMethodBind,
		unsafe.Pointer(script),
		(*unsafe.Pointer)(unsafe.Pointer(&setLibraryArgs[0])),
		nilptr,
	)

	strClassName := C.CString(className)
	defer C.free(unsafe.Pointer(strClassName))

	setClassNameArgs := [...]unsafe.Pointer{unsafe.Pointer(&strClassName)}

	C.go_godot_method_bind_ptrcall(
		CoreApi,
		wrappedSetClassNameMethodBind,
		unsafe.Pointer(script),
		(*unsafe.Pointer)(unsafe.Pointer(&setClassNameArgs[0])),
		nilptr,
	)

	// Now to instanciate T, we initially did this, however in case of Reference it returns a variant with refcount
	// already initialized, which woud cause inconsistent behavior compared to other classes (we still have to return a pointer).
	//Variant instance_variant = script->new_();
	//T *instance = godot::get_custom_class_instance<T>(instance_variant);

	// So we should do this instead, however while convenient, it uses unnecessary wrapper objects.
	//	Object *base_obj = T::___new_godot_base();
	//	base_obj->set_script(script);
	//	return get_custom_class_instance<T>(base_obj);

	// Again using the C API to do exactly what we have to do.
	strBaseClassName := C.CString(baseClassName)
	defer C.free(unsafe.Pointer(strBaseClassName))

	baseObject := (*C.godot_object)(C.go_godot_get_class_constructor_new(CoreApi, strBaseClassName))

	setScriptArgs := [...]unsafe.Pointer{unsafe.Pointer(script)}

	C.go_godot_method_bind_ptrcall(
		CoreApi,
		wrappedSetScriptMethodBind,
		unsafe.Pointer(baseObject),
		(*unsafe.Pointer)(unsafe.Pointer(&setScriptArgs[0])),
		nilptr,
	)

	ud := (UserData)(uintptr(C.go_godot_nativescript_get_userdata(NativescriptApi, unsafe.Pointer(baseObject))))

	classInst, ok := nativeScriptInstanceMap[ud]

	if !ok {
		log.Panic("unable to find NativeScriptClass instance")
	}

	rt := reflect.TypeOf(classInst)

	log.Info("CreateCustomClassInstance: returned type", StringField("type", rt.Name()))

	return classInst
}
