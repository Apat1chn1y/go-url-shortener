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

// New создаёт новый пул объектов с заданной фабрикой создания новых экземпляров.
// Фабрика newFunc должна возвращать новый объект типа T (обычно указатель на структуру).
func New[T Resetter](newFunc func() T) *Pool[T] {
	return &Pool[T]{
		pool: sync.Pool{
			New: func() any {
				return newFunc()
			},
		},
	}
}

// Get возвращает объект из пула. Если пул пуст, создаётся новый объект через фабрику.
func (p *Pool[T]) Get() T {
	return p.pool.Get().(T)
}

// Put возвращает объект в пул. Перед помещением вызывается метод Reset() объекта.
func (p *Pool[T]) Put(obj T) {
	obj.Reset()
	p.pool.Put(obj)
}
