package pool

import (
	"sync"
	"testing"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/storage"
)

// Тест демонстрирует базовую работу пула с объектами, имеющими метод Reset().
func TestPool_Basic(t *testing.T) {
	// Создаём пул для указателей на TestReset
	p := New[*storage.TestReset](func() *storage.TestReset {
		return &storage.TestReset{}
	})

	// Получаем объект из пула (должен быть новым)
	obj := p.Get()
	if obj == nil {
		t.Fatal("expected non-nil object")
	}

	// Модифицируем объект
	obj.ID = 42
	obj.Name = "test"
	obj.Tags = []string{"a", "b"}
	obj.Data = map[string]string{"key": "value"}

	// Возвращаем в пул (автоматически вызывается Reset)
	p.Put(obj)

	// Получаем другой объект из пула (должен быть сброшенным)
	obj2 := p.Get()
	if obj2 == nil {
		t.Fatal("expected non-nil object")
	}

	// Проверяем, что все поля сброшены
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

// Тест проверяет, что пул безопасен для конкурентного использования.
func TestPool_Concurrent(t *testing.T) {
	p := New[*storage.TestReset](func() *storage.TestReset {
		return &storage.TestReset{}
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
				// Изменяем объект, чтобы убедиться, что он сбрасывается
				obj.ID = 123
				obj.Name = "hello"
				// Возвращаем в пул
				p.Put(obj)
			}
		}()
	}

	wg.Wait()

	// Проверяем, что пул всё ещё работает и возвращает корректные объекты
	obj := p.Get()
	if obj == nil {
		t.Fatal("expected non-nil object")
	}
	if obj.ID != 0 || obj.Name != "" {
		t.Errorf("object not reset properly: ID=%d, Name=%q", obj.ID, obj.Name)
	}
}

// Бенчмарк показывает производительность пула по сравнению с созданием новых объектов.
func BenchmarkPool_GetPut(b *testing.B) {
	p := New[*storage.TestReset](func() *storage.TestReset {
		return &storage.TestReset{}
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		obj := p.Get()
		p.Put(obj)
	}
}

// Бенчмарк для сравнения с прямым созданием объектов.
func BenchmarkNewObject(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &storage.TestReset{}
	}
}
