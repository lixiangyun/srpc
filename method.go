package srpc

import (
	"errors"
	"sync"
)

type MethodInfo struct {
	ID   uint32
	Name string
	Req  string
	Rsp  string
}

type Method struct {
	rwlock       sync.RWMutex
	ID           uint32
	methodByName map[string]*MethodInfo
	methodById   map[uint32]*MethodInfo
}

func NewMethod() *Method {
	m := new(Method)
	m.methodById = make(map[uint32]*MethodInfo, 100)
	m.methodByName = make(map[string]*MethodInfo, 100)
	return m
}

func (m *Method) Add(name, req, rsp string) (uint32, error) {
	m.rwlock.Lock()
	defer m.rwlock.Unlock()

	_, b := m.methodByName[name]
	if b == true {
		return 0, errors.New("method had been add.")
	}

	m.ID++

	newmethod := &MethodInfo{Name: name, ID: m.ID, Req: req, Rsp: rsp}

	m.methodById[m.ID] = newmethod
	m.methodByName[name] = newmethod

	return m.ID, nil
}

func (m *Method) BatchAdd(method []MethodInfo) error {
	m.rwlock.Lock()
	defer m.rwlock.Unlock()

	for _, vfun := range method {
		_, b := m.methodByName[vfun.Name]
		if b == true {
			return errors.New("method had been add.")
		}
	}

	for _, vfun := range method {
		m.methodByName[vfun.Name] = &vfun
		m.methodById[vfun.ID] = &vfun

		if vfun.ID > m.ID {
			m.ID = vfun.ID
		}
	}

	return nil
}

func (m *Method) GetBatch() []MethodInfo {
	m.rwlock.RLock()
	defer m.rwlock.RUnlock()

	functable := make([]MethodInfo, 0)

	for idx, vfun := range m.methodByName {
		functable = append(functable, *vfun)
	}

	return functable
}

func (m *Method) GetByName(name string) (MethodInfo, error) {
	m.rwlock.RLock()
	defer m.rwlock.RUnlock()

	var method MethodInfo
	vfun, b := m.methodByName[name]
	if b == false {
		return method, errors.New("method not found.")
	}

	return *vfun, nil
}

func (m *Method) GetByID(id uint32) (MethodInfo, error) {
	m.rwlock.RLock()
	defer m.rwlock.RUnlock()

	var method MethodInfo
	vfun, b := m.methodById[id]
	if b == false {
		return method, errors.New("method not found.")
	}

	return *vfun, nil
}
