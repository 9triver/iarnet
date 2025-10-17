package runtime

import (
	"strings"
	"time"

	"github.com/9triver/ignis/actor/functions"
	"github.com/9triver/ignis/objects"
	"github.com/9triver/ignis/proto"
	"github.com/9triver/ignis/proto/cluster"
	"github.com/9triver/ignis/utils/errors"
)

type Funciton struct {
	functions.FuncDec
	name          string
	params        []string
	requirements  []string
	pickledObject []byte
	language      proto.Language
	manager       Manager
}

func NewFunciton(manager Manager, funcMsg *cluster.Function) *Funciton {
	dec := functions.Declare(funcMsg.Name, funcMsg.Params)
	manager.Setup(funcMsg)
	return &Funciton{
		FuncDec:       dec,
		language:      funcMsg.Language,
		name:          funcMsg.Name,
		params:        funcMsg.Params,
		requirements:  funcMsg.Requirements,
		pickledObject: funcMsg.PickledObject,
	}
}

func (f *Funciton) Call(params map[string]objects.Interface) (objects.Interface, error) {
	segs := strings.Split(f.Name(), ".")
	var obj, method string
	if len(segs) >= 2 {
		obj, method = segs[0], segs[1]
	} else {
		obj, method = f.Name(), ""
	}
	result, err := f.manager.Execute(obj, method, params).Result()
	if err != nil {
		return nil, errors.WrapWith(err, "%s: execution failed", f.name)
	}
	return result, nil
}

func (f *Funciton) TimedCall(params map[string]objects.Interface) (time.Duration, objects.Interface, error) {
	start := time.Now()
	obj, err := f.Call(params)
	return time.Since(start), obj, err
}

func (f *Funciton) Language() proto.Language {
	return f.language
}
