package srpc

import (
	"errors"
	"log"
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
	m.methodById = make(map[string]*MethodInfo, 100)
	m.methodByName = make(map[uint32]*MethodInfo, 100)
	return m
}

func (m *Method) Add(name, req, rsp string) error {
	m.rwlock.Lock()
	defer m.rwlock.Unlock()

	_, b := m.methodByName[name]
	if b == true {
		return errors.New("method had been add.", name)
	}

	m.ID++

	newmethod := &MethodInfo{Name: name, ID, m.ID, Req: req, Rsp: rsp}

	m.methodById[m.ID] = newmethod
	m.methodByName[name] = newmethod

	return nil
}

func (m *Method) BatchAdd(method []MethodInfo) error {
	m.rwlock.Lock()
	defer m.rwlock.Unlock()

	for _, vfun := range method {
		_, b := m.methodByName[vfun.Name]
		if b == true {
			return errors.New("method had been add.", vfun)
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

	len := len(m.methodByName)
	functable := make([]MethodInfo, len)

	for idx, vfun := range m.methodByName {
		functable[idx] = *vfun
	}

	return functable
}

func (m *Method) GetByName(name string) (MethodInfo, error) {
	m.rwlock.RLock()
	defer m.rwlock.RUnlock()

	method, b := m.methodByName[name]
	if b == false {
		return 0, errors.New("method not found.")
	}

	return *method, nil
}

func (m *Method) GetByID(id uint32) MethodInfo {
	m.rwlock.RLock()
	defer m.rwlock.RUnlock()

	method, b := m.methodById[id]
	if b == false {
		log.Println("method not found.", id)
		return nil
	}

	return *method
}
