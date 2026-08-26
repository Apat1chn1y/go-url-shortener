package pool

import (
	"sync"
	"testing"
)

// testObject — локальная заглушка для тестов.
type testObject struct {
	ID    int
	Name  string
	Tags  []string
	Data  map[string]string
	Child *testObject
}

func (o *testObject) Reset() {
	if o == nil {
		return
	}
	o.ID = 0
	o.Name = ""
	o.Tags = o.Tags[:0]
	clear(o.Data)
	if o.Child != nil {
		o.Child.Reset()
	}
}

func TestPool_Basic(t *testing.T) {
	p := New[*testObject](func() *testObject {
		return &testObject{}
	})

	obj := p.Get()
	if obj == nil {
		t.Fatal("expected non-nil object")
	}

	obj.ID = 42
	obj.Name = "test"
	obj.Tags = []string{"a", "b"}
	obj.Data = map[string]string{"key": "value"}

	p.Put(obj)

	obj2 := p.Get()
	if obj2 == nil {
		t.Fatal("expected non-nil object")
	}

	if obj2.ID != 0 {
		t.Errorf("expected ID=0, got %d", obj2.ID)
	}
	if obj2.Name != "" {
		t.Errorf("expected Name='', got %q", obj2.Name)
	}
	if len(obj2.Tags) != 0 {
		t.Errorf("expected empty Tags, got len=%d", len(obj2.Tags))
	}
	if len(obj2.Data) != 0 {
		t.Errorf("expected empty Data, got len=%d", len(obj2.Data))
	}
	if obj2.Child != nil {
		t.Errorf("expected Child=nil, got %v", obj2.Child)
	}
}

func TestPool_NilFactory(t *testing.T) {
	p := New[*testObject](nil)
	obj := p.Get()
	// zero-значение для указателя — nil
	if obj != nil {
		t.Errorf("expected nil, got %v", obj)
	}
	// Put не должен паниковать при nil
	p.Put(obj)
}

func TestPool_Concurrent(t *testing.T) {
	p := New[*testObject](func() *testObject {
		return &testObject{}
	})

	const goroutines = 10
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				obj := p.Get()
				obj.ID = 123
				obj.Name = "hello"
				p.Put(obj)
			}
		}()
	}

	wg.Wait()

	obj := p.Get()
	if obj == nil {
		t.Fatal("expected non-nil object")
	}
	if obj.ID != 0 || obj.Name != "" {
		t.Errorf("object not reset properly: ID=%d, Name=%q", obj.ID, obj.Name)
	}
}

func BenchmarkPool_GetPut(b *testing.B) {
	p := New[*testObject](func() *testObject {
		return &testObject{}
	})
	for b.Loop() {
		obj := p.Get()
		p.Put(obj)
	}
}

func BenchmarkNewObject(b *testing.B) {
	for b.Loop() {
		_ = &testObject{}
	}
}
