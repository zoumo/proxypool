package validator

import (
	"sync"

	"github.com/zoumo/golib/queue"

	v2 "github.com/zoumo/proxypool/pkg/apis/v2"
)

var (
	validateFuncs = sync.Map{}
)

type ValidateFunc func(addr string) error

func RegisterValidateFunc(name string, validate ValidateFunc) {
	validateFuncs.Store(name, validate)
}

func loadInitValidators(vs *Validators) {
	validateFuncs.Range(func(key, value interface{}) bool {
		name := key.(string)
		v := newGeneric(name, nil)

		if value != nil {
			validate := value.(ValidateFunc)
			v.validate = func(addr string) {
				err := validate(addr)
				if err != nil {
					return
				}
				v.Add(&v2.Proxy{
					Validator: name,
					Addr:      addr,
				})
			}
		}

		vs.LoadOrStore(name, v)
		return true
	})

}

// Validators ...
type Validators struct {
	m         sync.Map
	workQueue *queue.Queue
}

func NewValidators() *Validators {
	v := &Validators{
		m: sync.Map{},
	}
	v.workQueue = queue.NewQueue(v.worker)
	return v
}

func (v *Validators) Run() {
	v.workQueue.Run(100)

	// init validators
	loadInitValidators(v)
}

func (v *Validators) Stop() {
	v.workQueue.ShutDown()
}

func (v *Validators) worker(obj interface{}) error {
	addr := obj.(string)
	v.m.Range(func(key, value interface{}) bool {
		vv := value.(Validator)
		vv.Validate(addr)
		return true
	})
	return nil
}

func (v *Validators) Enqueue(addr string) {
	v.workQueue.Enqueue(addr)
}

// Load returns the value stored in the map for a key, or nil if no
// value is present.
// The ok result indicates whether value was found in the map.
func (v *Validators) Load(name string) (Validator, bool) {
	value, ok := v.m.Load(name)
	if !ok {
		return nil, false
	}
	return value.(Validator), true
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
// The loaded result is true if the value was loaded, false if stored.
func (v *Validators) LoadOrStore(name string, vv Validator) (Validator, bool) {
	actual, ok := v.m.LoadOrStore(name, vv)
	if ok {
		return actual.(Validator), true
	}
	vv.Run()
	return vv, false
}

// Delete deletes the value for a key.
func (v *Validators) Delete(name string) {
	vv, ok := v.Load(name)
	if ok {
		v.m.Delete(name)
		vv.Stop()
	}
}
