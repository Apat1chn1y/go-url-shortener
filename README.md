# go-musthave-shortener-tpl

Шаблон репозитория для трека «Сервис сокращения URL».

## Начало работы

1. Склонируйте репозиторий в любую подходящую директорию на вашем компьютере.
2. В корне репозитория выполните команду `go mod init <name>` (где `<name>` — адрес вашего репозитория на GitHub без префикса `https://`) для создания модуля.

## Обновление шаблона

Чтобы иметь возможность получать обновления автотестов и других частей шаблона, выполните команду:

```
git remote add -m v2 template https://github.com/Yandex-Practicum/go-musthave-shortener-tpl.git
```

Для обновления кода автотестов выполните команду:

```
git fetch template && git checkout template/v2 .github
```

Затем добавьте полученные изменения в свой репозиторий.

## Запуск автотестов

Для успешного запуска автотестов называйте ветки `iter<number>`, где `<number>` — порядковый номер инкремента. Например, в ветке с названием `iter4` запустятся автотесты для инкрементов с первого по четвёртый.

При мёрже ветки с инкрементом в основную ветку `main` будут запускаться все автотесты.

Подробнее про локальный и автоматический запуск читайте в [README автотестов](https://github.com/Yandex-Practicum/go-autotests).

## Структура проекта

Приведённая в этом репозитории структура проекта является рекомендуемой, но не обязательной.

Это лишь пример организации кода, который поможет вам в реализации сервиса.

При необходимости можно вносить изменения в структуру проекта, использовать любые библиотеки и предпочитаемые структурные паттерны организации кода приложения, например:
- **DDD** (Domain-Driven Design)
- **Clean Architecture**
- **Hexagonal Architecture**
- **Layered Architecture**






## Профилирование памяти

В рамках оптимизации производительности был проведён анализ использования памяти с помощью профилировщика `pprof`.  
Были сняты два профиля: базовый (до оптимизаций) и итоговый (после внесённых изменений).

### 1. Базовый профиль

```bash
go test -bench=. -benchmem -memprofile profiles/base.pprof ./internal/handlers
```

Профиль показал, что наибольшее количество аллокаций приходится на:

- bufio.NewReaderSize (58.23% от всех аллокаций)
- net/http.(*Request).WithContext (8.98%)
- net/textproto.MIMEHeader.Set (7.28%)
- net/http.Header.Clone (5.55%)

Эти аллокации связаны с внутренней работой стандартной библиотеки при обработке HTTP-запросов, и мы не можем на них повлиять напрямую.
Однако мы обнаружили, что в хендлерах HandleBatchShorten и GetUserURLs создаются избыточные промежуточные структуры для JSON-ответов.

### 2. Оптимизации

- HandleBatchShorten – вместо создания промежуточного слайса анонимных структур, мы стали использовать прямой вызов json.NewEncoder(w).Encode(results), где results уже содержит правильные теги JSON.

- GetUserURLs – аналогично, убрали создание промежуточного слайса, изменив структуру storage.UserURL и заполняя поле ShortURL напрямую перед сериализацией.

### 3. Повторный профиль и сравнение

```bash
go test -bench=. -benchmem -memprofile profiles/result.pprof ./internal/handlers
go tool pprof -top -diff_base=profiles/base.pprof profiles/result.pprof
```

Результат сравнения:
```text
(pprof) top -diff_base=base.pprof result.pprof
Showing nodes accounting for 2081.06MB, 52.11% of 3993.45MB total
Dropped 97 nodes (cum <= 19.97MB)
      flat  flat%   sum%        cum   cum%
 1130.29MB 28.30% 28.30%  1130.29MB 28.30%  bufio.NewReaderSize (inline)
  226.57MB  5.67% 33.98%   226.57MB  5.67%  net/textproto.MIMEHeader.Set (inline)
  182.06MB  4.56% 38.54%   182.06MB  4.56%  net/http.(*Request).WithContext (inline)
  118.03MB  2.96% 41.49%   118.03MB  2.96%  net/http.Header.Clone (inline)
   79.02MB  1.98% 43.47%   163.04MB  4.08%  net/http.readRequest
   62.53MB  1.57% 45.04%    62.01MB  1.55%  encoding/json.(*Decoder).refill
   60.51MB  1.52% 46.55%    60.51MB  1.52%  net/url.parse
   56.03MB  1.40% 47.95%    56.03MB  1.40%  io.ReadAll
   51.50MB  1.29% 49.24%    51.50MB  1.29%  net/http/httptest.NewRecorder (inline)
   30.51MB  0.76% 50.01%    30.51MB  0.76%  encoding/json.NewDecoder (inline)
   17.50MB  0.44% 50.45%    17.50MB  0.44%  bytes.NewBuffer (inline)
   16.50MB  0.41% 50.86%       19MB  0.48%  encoding/json.(*decodeState).object
   15.50MB  0.39% 51.25%    15.50MB  0.39%  context.WithValue
      13MB  0.33% 51.57%       13MB  0.33%  net/textproto.readMIMEHeader
      12MB   0.3% 51.87%    19.50MB  0.49%  bytes.(*Buffer).grow
    4.50MB  0.11% 51.99%   180.06MB  4.51%  github.com/.../handlers.(*ShortenHandler).Create
    4.50MB  0.11% 52.10%   139.62MB  3.50%  github.com/.../handlers.(*ShortenHandler).HandleShortenJSON
    3.50MB 0.088% 52.19%  1405.36MB 35.19%  net/http/httptest.NewRequestWithContext
      -3MB 0.075% 52.11%   117.53MB  2.94%  github.com/.../handlers.(*ShortenHandler).HandleBatchShorten
```

Ключевой показатель: для хендлера HandleBatchShorten мы видим отрицательное значение -3MB в столбце flat, что свидетельствует об уменьшении использования памяти.

### 4. Вывод

Оптимизации позволили сократить аллокации в обработчике батча на 3 МБ без потери функциональности. Это подтверждает эффективность внесённых изменений и улучшает общую производительность сервиса.
