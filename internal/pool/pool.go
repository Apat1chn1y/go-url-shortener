// Package pool предоставляет реализацию пула объектов с автоматическим сбросом состояния.
package pool

import (
	"sync"
)

// Resetter определяет контракт для объектов, которые могут сбрасывать своё состояние.
type Resetter interface {
	Reset()
}

// Pool представляет собой пул объектов типа T, где T должен реализовывать интерфейс Resetter.
// При возврате объекта в пул автоматически вызывается метод Reset().
type Pool[T Resetter] struct {
	pool sync.Pool
}

// New создаёт новый пул объектов.
// Если newFunc равна nil, пул будет возвращать zero-значение T (для указателей — nil).
// Иначе используется переданная фабрика для создания новых объектов.
func New[T Resetter](newFunc func() T) *Pool[T] {
	var newFn func() any
	if newFunc == nil {
		newFn = func() any {
			var zero T
			return zero // zero-значение T (nil для указателей)
		}
	} else {
		newFn = func() any {
			return newFunc()
		}
	}
	return &Pool[T]{
		pool: sync.Pool{
			New: newFn,
		},
	}
}

// Get возвращает объект из пула. Если пул пуст, создаётся новый объект через фабрику
// (или возвращается zero-значение, если фабрика не задана).
func (p *Pool[T]) Get() T {
	return p.pool.Get().(T)
}

// Put возвращает объект в пул. Перед помещением вызывается метод Reset() объекта.
// Если объект равен zero-значению (например, nil для указателей), вызов Reset() пропускается.
func (p *Pool[T]) Put(obj T) {
	// Проверяем, не является ли obj zero-значением (для указателей это nil)
	if any(obj) == nil {
		return
	}
	obj.Reset()
	p.pool.Put(obj)
}
