package engine

/*
#cgo LDFLAGS: -ldl

#include <dlfcn.h>
#include <stdlib.h>

// Uniform native function signature: JSON in, JSON out.
typedef const char* (*native_fn)(const char*);

// C wrapper to call a function pointer (Go can't call C function pointers directly).
static const char* call_native_fn(native_fn fn, const char* json_args) {
    return fn(json_args);
}
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"sync"
	"unsafe"
)

// nativeFunc holds a dlsym'd function pointer and its library info.
type nativeFunc struct {
	symbol C.native_fn
	lib    *nativeLib
}

// nativeLib holds a dlopen'd library handle.
type nativeLib struct {
	handle unsafe.Pointer
	path   string
}

// FFIRegistry manages loaded native libraries and their callable functions.
type FFIRegistry struct {
	mu    sync.RWMutex
	libs  []*nativeLib
	funcs map[string]*nativeFunc // "prefix.name" → callable
}

// NewFFIRegistry creates an empty registry.
func NewFFIRegistry() *FFIRegistry {
	return &FFIRegistry{
		funcs: make(map[string]*nativeFunc),
	}
}

// LoadFromConfig loads all libraries and functions from an FFIConfig.
func (r *FFIRegistry) LoadFromConfig(cfg *FFIConfig) error {
	for _, lib := range cfg.Libraries {
		if err := r.LoadLibrary(lib.Path, lib.Prefix, lib.Functions); err != nil {
			return err
		}
	}
	return nil
}

// LoadLibrary opens a dylib and registers each declared function.
func (r *FFIRegistry) LoadLibrary(path, prefix string, funcs []FuncConfig) error {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	handle := C.dlopen(cPath, C.RTLD_NOW)
	if handle == nil {
		errMsg := C.GoString(C.dlerror())
		return fmt.Errorf("ffi: dlopen %s: %s", path, errMsg)
	}

	lib := &nativeLib{handle: handle, path: path}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.libs = append(r.libs, lib)

	for _, fc := range funcs {
		cSym := C.CString(fc.Symbol)
		sym := C.native_fn(C.dlsym(handle, cSym))
		C.free(unsafe.Pointer(cSym))

		if sym == nil {
			errMsg := C.GoString(C.dlerror())
			return fmt.Errorf("ffi: dlsym %s in %s: %s", fc.Symbol, path, errMsg)
		}

		name := prefix + "." + fc.Name
		r.funcs[name] = &nativeFunc{symbol: sym, lib: lib}
	}

	return nil
}

// Has returns true if the named function is registered.
func (r *FFIRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.funcs[name]
	return ok
}

// Call invokes a registered native function with the given args.
// Args are marshalled to a JSON array, passed to the native function,
// and the JSON result is unmarshalled back.
func (r *FFIRegistry) Call(name string, args []interface{}) (interface{}, error) {
	r.mu.RLock()
	fn, ok := r.funcs[name]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("ffi: unknown function: %s", name)
	}

	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("ffi: marshal args: %w", err)
	}

	cArgs := C.CString(string(argsJSON))
	defer C.free(unsafe.Pointer(cArgs))

	cResult := C.call_native_fn(fn.symbol, cArgs)
	if cResult == nil {
		return nil, fmt.Errorf("ffi: %s returned NULL", name)
	}

	resultStr := C.GoString(cResult)
	// The native function allocated the result string; free it.
	C.free(unsafe.Pointer(cResult))

	var result interface{}
	if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
		return nil, fmt.Errorf("ffi: unmarshal result from %s: %w", name, err)
	}

	return result, nil
}

// Close dlcloses all loaded libraries.
func (r *FFIRegistry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, lib := range r.libs {
		C.dlclose(lib.handle)
	}
	r.libs = nil
	r.funcs = make(map[string]*nativeFunc)
}
