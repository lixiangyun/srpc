package srpc

import (
	"errors"
	"sync"
)

// rpc调用的方法信息
type MethodInfo struct {
	ID   uint32 // 方法的ID（和方法一一对应），用于寻址RPC请求的方法
	Name string // 方法的名称（请求的符号名称）
	Req  string // 请求的参数类型名称
	Rsp  string // 应答的参数类型名称
}

//服务端被调用的方法管理结构
type Method struct {
	rwlock       sync.RWMutex           // 用于被调用的方法添加、查询的读写锁
	ID           uint32                 // 用于分配方法ID的变量
	methodByName map[string]*MethodInfo // 通过名称查询方法信息的map表
	methodById   map[uint32]*MethodInfo // 通过ID查询方法信息的map表
}

//申请一个方法管理结构对象
func NewMethod() *Method {
	m := new(Method)
	m.methodById = make(map[uint32]*MethodInfo, 100)
	m.methodByName = make(map[string]*MethodInfo, 100)
	return m
}

//添加一个方法，参数包括：方法名称、请求、应答的参数类型名称
func (m *Method) Add(name, req, rsp string) (uint32, error) {
	m.rwlock.Lock()
	defer m.rwlock.Unlock()

	// 查找是否已经存在的方法信息
	_, b := m.methodByName[name]
	if b == true {
		return 0, errors.New("method had been add.")
	}

	// 分配一个ID
	m.ID++

	// 申请一个新的方法信息结构对象
	newmethod := &MethodInfo{Name: name, ID: m.ID, Req: req, Rsp: rsp}

	m.methodById[m.ID] = newmethod
	m.methodByName[name] = newmethod

	return m.ID, nil
}

// 批量添加方法信息
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

		newmethod := vfun
		m.methodByName[vfun.Name] = &newmethod
		m.methodById[vfun.ID] = &newmethod

		if vfun.ID > m.ID {
			m.ID = vfun.ID
		}
	}

	return nil
}

// 批量获取方法信息
func (m *Method) GetBatch() []MethodInfo {
	m.rwlock.RLock()
	defer m.rwlock.RUnlock()

	functable := make([]MethodInfo, 0)

	for _, vfun := range m.methodByName {
		functable = append(functable, *vfun)
	}

	return functable
}

// 通过方法名称获取方法信息
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

//通过ID获取方法信息
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
