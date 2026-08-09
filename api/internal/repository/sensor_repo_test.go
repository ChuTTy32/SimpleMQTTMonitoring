package repository

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ChuTTy32/SimpleMQTTMonitoring/api/internal/model"
)

// Интеграционные тесты: проверяют то, что подделкой репозитория проверить нельзя —
// перевод ошибок реального Postgres (FOREIGN KEY, CHECK, UNIQUE) в доменные ошибки.
//
// Запускаются только при заданном TEST_DB_DSN, иначе пропускаются: `go test ./...` должен
// проходить у того, у кого не поднят Docker.
//
//	TEST_DB_DSN='postgres://mqtt_monitor:...@localhost:5432/mqtt_monitoring?sslmode=disable' go test ./...
var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn != "" {
		pool, err := pgxpool.New(context.Background(), dsn)
		if err != nil {
			panic("TEST_DB_DSN задан, но подключиться не удалось: " + err.Error())
		}
		testPool = pool
		defer pool.Close()
	}

	os.Exit(m.Run())
}

// beginTx выдаёт транзакцию, которая откатывается после теста. Репозиторий строится
// поверх неё (sqlc.DBTX принимает и пул, и транзакцию), поэтому тесты не видят изменений
// друг друга, не зависят от порядка выполнения и не оставляют мусор в БД — ручная чистка
// не нужна.
func beginTx(t *testing.T) pgx.Tx {
	t.Helper()

	if testPool == nil {
		t.Skip("TEST_DB_DSN не задан — интеграционный тест пропущен")
	}

	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	require.NoError(t, err)

	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	return tx
}

// newTestController создаёт контроллер, к которому можно цеплять датчики. Переиспользует
// боевой ControllerRepository, а не отдельный INSERT в тесте — заодно проверяется, что
// два модуля стыкуются.
func newTestController(t *testing.T, tx pgx.Tx) uuid.UUID {
	t.Helper()

	c, err := NewControllerRepository(tx).Create(context.Background(), model.CreateControllerInput{
		Name:          "Тестовый контроллер",
		MQTTGatewayID: "test-" + uuid.NewString(),
	})
	require.NoError(t, err)

	return c.ID
}

func validSensorInput(controllerID uuid.UUID) model.CreateSensorInput {
	return model.CreateSensorInput{
		ControllerID: controllerID,
		Name:         "Температура",
		Topic:        "gateway/test-" + uuid.NewString() + "/temperature",
		MetricType:   "temperature",
		Unit:         "°C",
	}
}

func TestSensorRepoCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("сохраняет и возвращает все поля", func(t *testing.T) {
		tx := beginTx(t)
		repo := NewSensorRepository(tx)

		in := validSensorInput(newTestController(t, tx))
		minT, maxT := 15.0, 30.0
		in.MinThreshold, in.MaxThreshold = &minT, &maxT

		got, err := repo.Create(ctx, in)

		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, got.ID)
		assert.Equal(t, in.ControllerID, got.ControllerID)
		assert.Equal(t, in.Topic, got.Topic)
		assert.Equal(t, in.Unit, got.Unit)
		require.NotNil(t, got.MinThreshold)
		assert.InDelta(t, 15.0, *got.MinThreshold, 0.0001)
		assert.True(t, got.IsActive, "is_active не передан — должен взяться DEFAULT true из схемы")
		assert.False(t, got.CreatedAt.IsZero())
	})

	t.Run("несуществующий controller_id → ErrReferenceNotFound, а не сырая ошибка драйвера", func(t *testing.T) {
		tx := beginTx(t)

		_, err := NewSensorRepository(tx).Create(ctx, validSensorInput(uuid.New()))

		assert.ErrorIs(t, err, model.ErrReferenceNotFound)
	})

	t.Run("занятый topic → ErrDuplicate", func(t *testing.T) {
		tx := beginTx(t)
		repo := NewSensorRepository(tx)

		in := validSensorInput(newTestController(t, tx))
		_, err := repo.Create(ctx, in)
		require.NoError(t, err)

		// Тот же topic под другим именем — UNIQUE стоит именно на topic.
		second := in
		second.Name = "Другое имя"
		_, err = repo.Create(ctx, second)

		assert.ErrorIs(t, err, model.ErrDuplicate)
	})

	t.Run("min_threshold больше max_threshold → ErrConstraintViolation", func(t *testing.T) {
		tx := beginTx(t)

		in := validSensorInput(newTestController(t, tx))
		minT, maxT := 50.0, 10.0
		in.MinThreshold, in.MaxThreshold = &minT, &maxT

		_, err := NewSensorRepository(tx).Create(ctx, in)

		assert.ErrorIs(t, err, model.ErrConstraintViolation)
	})
}

func TestSensorRepoGetAndDelete(t *testing.T) {
	ctx := context.Background()

	t.Run("GetByID несуществующего → ErrNotFound", func(t *testing.T) {
		tx := beginTx(t)

		_, err := NewSensorRepository(tx).GetByID(ctx, uuid.New())

		assert.ErrorIs(t, err, model.ErrNotFound)
	})

	t.Run("Delete несуществующего → ErrNotFound", func(t *testing.T) {
		tx := beginTx(t)

		err := NewSensorRepository(tx).Delete(ctx, uuid.New())

		assert.ErrorIs(t, err, model.ErrNotFound)
	})

	t.Run("после Delete датчик не читается", func(t *testing.T) {
		tx := beginTx(t)
		repo := NewSensorRepository(tx)

		created, err := repo.Create(ctx, validSensorInput(newTestController(t, tx)))
		require.NoError(t, err)

		require.NoError(t, repo.Delete(ctx, created.ID))

		_, err = repo.GetByID(ctx, created.ID)
		assert.ErrorIs(t, err, model.ErrNotFound)
	})
}

func TestSensorRepoListFilter(t *testing.T) {
	ctx := context.Background()

	t.Run("фильтр по controller_id отдаёт только его датчики", func(t *testing.T) {
		tx := beginTx(t)
		repo := NewSensorRepository(tx)

		mine := newTestController(t, tx)
		other := newTestController(t, tx)

		for i := 0; i < 2; i++ {
			_, err := repo.Create(ctx, validSensorInput(mine))
			require.NoError(t, err)
		}
		_, err := repo.Create(ctx, validSensorInput(other))
		require.NoError(t, err)

		got, err := repo.List(ctx, model.SensorFilter{ControllerID: &mine}, 100, 0)

		require.NoError(t, err)
		assert.Len(t, got, 2)
		for _, s := range got {
			assert.Equal(t, mine, s.ControllerID)
		}
	})

	t.Run("Count учитывает тот же фильтр", func(t *testing.T) {
		tx := beginTx(t)
		repo := NewSensorRepository(tx)

		mine := newTestController(t, tx)
		_, err := repo.Create(ctx, validSensorInput(mine))
		require.NoError(t, err)
		_, err = repo.Create(ctx, validSensorInput(newTestController(t, tx)))
		require.NoError(t, err)

		filtered, err := repo.Count(ctx, model.SensorFilter{ControllerID: &mine})
		require.NoError(t, err)
		assert.Equal(t, int64(1), filtered, "иначе total в пагинации не совпадёт с отфильтрованным списком")

		all, err := repo.Count(ctx, model.SensorFilter{})
		require.NoError(t, err)
		assert.Greater(t, all, filtered, "без фильтра должно быть больше — в базе есть seed-данные")
	})

	t.Run("фильтр по контроллеру без датчиков — пустой срез, не ошибка", func(t *testing.T) {
		tx := beginTx(t)
		empty := newTestController(t, tx)

		got, err := NewSensorRepository(tx).List(ctx, model.SensorFilter{ControllerID: &empty}, 100, 0)

		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestSensorRepoUpdate(t *testing.T) {
	ctx := context.Background()

	t.Run("частичное обновление не затирает остальные поля", func(t *testing.T) {
		tx := beginTx(t)
		repo := NewSensorRepository(tx)

		in := validSensorInput(newTestController(t, tx))
		minT := 10.0
		in.MinThreshold = &minT
		created, err := repo.Create(ctx, in)
		require.NoError(t, err)

		newName := "Переименован"
		got, err := repo.Update(ctx, created.ID, model.UpdateSensorInput{Name: &newName})

		require.NoError(t, err)
		assert.Equal(t, newName, got.Name)
		assert.Equal(t, created.Topic, got.Topic, "непереданное поле не должно обнуляться")
		assert.Equal(t, created.Unit, got.Unit)
		assert.Equal(t, created.MetricType, got.MetricType)
		require.NotNil(t, got.MinThreshold)
		assert.InDelta(t, minT, *got.MinThreshold, 0.0001)
	})

	t.Run("несуществующий id → ErrNotFound", func(t *testing.T) {
		tx := beginTx(t)
		name := "x"

		_, err := NewSensorRepository(tx).Update(ctx, uuid.New(), model.UpdateSensorInput{Name: &name})

		assert.ErrorIs(t, err, model.ErrNotFound)
	})

	t.Run("PATCH, ломающий CHECK порогов, → ErrConstraintViolation", func(t *testing.T) {
		tx := beginTx(t)
		repo := NewSensorRepository(tx)

		in := validSensorInput(newTestController(t, tx))
		minT, maxT := 10.0, 20.0
		in.MinThreshold, in.MaxThreshold = &minT, &maxT
		created, err := repo.Create(ctx, in)
		require.NoError(t, err)

		// Go-валидация этот случай не ловит: передан только один порог, второй лежит
		// в базе. Последняя линия обороны — CHECK.
		broken := 99.0
		_, err = repo.Update(ctx, created.ID, model.UpdateSensorInput{MinThreshold: &broken})

		assert.ErrorIs(t, err, model.ErrConstraintViolation)
	})
}
