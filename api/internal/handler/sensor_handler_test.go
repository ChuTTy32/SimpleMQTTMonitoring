package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ChuTTy32/SimpleMQTTMonitoring/api/internal/model"
)

// fakeSensorService — подделка сервиса: тесты этого файла проверяют HTTP-слой
// (разбор запроса, коды ответов, формат ошибок), а не бизнес-логику и не БД.
// Поля-функции позволяют каждому тесту задать своё поведение, не плодя типы.
type fakeSensorService struct {
	listFn   func(ctx context.Context, f model.SensorFilter, limit, offset int32) ([]model.Sensor, int64, error)
	getFn    func(ctx context.Context, id uuid.UUID) (model.Sensor, error)
	createFn func(ctx context.Context, in model.CreateSensorInput) (model.Sensor, error)
	updateFn func(ctx context.Context, id uuid.UUID, in model.UpdateSensorInput) (model.Sensor, error)
	deleteFn func(ctx context.Context, id uuid.UUID) error

	// Записанные аргументы — чтобы проверить, что до сервиса дошло именно то,
	// что было в запросе (лимиты, фильтр, набор изменяемых полей).
	gotFilter model.SensorFilter
	gotLimit  int32
	gotOffset int32
	gotUpdate model.UpdateSensorInput
}

func (f *fakeSensorService) List(ctx context.Context, filter model.SensorFilter, limit, offset int32) ([]model.Sensor, int64, error) {
	f.gotFilter, f.gotLimit, f.gotOffset = filter, limit, offset
	if f.listFn != nil {
		return f.listFn(ctx, filter, limit, offset)
	}
	return []model.Sensor{}, 0, nil
}

func (f *fakeSensorService) GetByID(ctx context.Context, id uuid.UUID) (model.Sensor, error) {
	return f.getFn(ctx, id)
}

func (f *fakeSensorService) Create(ctx context.Context, in model.CreateSensorInput) (model.Sensor, error) {
	return f.createFn(ctx, in)
}

func (f *fakeSensorService) Update(ctx context.Context, id uuid.UUID, in model.UpdateSensorInput) (model.Sensor, error) {
	f.gotUpdate = in
	if f.updateFn != nil {
		return f.updateFn(ctx, id, in)
	}
	return model.Sensor{}, nil
}

func (f *fakeSensorService) Delete(ctx context.Context, id uuid.UUID) error {
	return f.deleteFn(ctx, id)
}

// newSensorTestServer поднимает роутер с примонтированным модулем sensors — маршруты
// должны проверяться через chi, а не прямым вызовом методов: иначе chi.URLParam
// не увидит {id} и тесты разойдутся с продакшен-поведением.
func newSensorTestServer(svc *fakeSensorService) http.Handler {
	r := chi.NewRouter()
	r.Mount("/sensors", NewSensorHandler(svc).Routes())
	return r
}

func doRequest(t *testing.T, h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}

	req := httptest.NewRequest(method, target, reader)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// decodeError разбирает конверт ошибки из docs/api-architecture.md.
func decodeError(t *testing.T, rec *httptest.ResponseRecorder) errorBody {
	t.Helper()
	var env errorEnvelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	return env.Error
}

func validSensorBody(controllerID uuid.UUID) string {
	return `{"controller_id":"` + controllerID.String() + `",
		"name":"Температура — цех №1",
		"topic":"gateway/esp32-01/temperature",
		"metric_type":"temperature",
		"unit":"°C"}`
}

func TestListSensors(t *testing.T) {
	t.Run("отдаёт конверт items+total и лимит по умолчанию", func(t *testing.T) {
		svc := &fakeSensorService{
			listFn: func(_ context.Context, _ model.SensorFilter, _, _ int32) ([]model.Sensor, int64, error) {
				return []model.Sensor{{ID: uuid.New(), Name: "s1"}}, 42, nil
			},
		}

		rec := doRequest(t, newSensorTestServer(svc), http.MethodGet, "/sensors", "")

		require.Equal(t, http.StatusOK, rec.Code)

		var got listEnvelope[model.Sensor]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		assert.Len(t, got.Items, 1)
		assert.Equal(t, int64(42), got.Total, "total должен приходить от сервиса, а не считаться из длины страницы")
		assert.Equal(t, int32(defaultLimit), svc.gotLimit)
		assert.Equal(t, int32(0), svc.gotOffset)
	})

	t.Run("пустой результат сериализуется в [], а не в null", func(t *testing.T) {
		svc := &fakeSensorService{
			listFn: func(_ context.Context, _ model.SensorFilter, _, _ int32) ([]model.Sensor, int64, error) {
				return []model.Sensor{}, 0, nil
			},
		}

		rec := doRequest(t, newSensorTestServer(svc), http.MethodGet, "/sensors", "")

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"items":[]`,
			"фронт не должен обрабатывать null и [] как два разных случая")
	})

	t.Run("limit прокидывается, а превышение максимума тихо зажимается", func(t *testing.T) {
		svc := &fakeSensorService{}
		doRequest(t, newSensorTestServer(svc), http.MethodGet, "/sensors?limit=1", "")
		assert.Equal(t, int32(1), svc.gotLimit)

		svc2 := &fakeSensorService{}
		rec := doRequest(t, newSensorTestServer(svc2), http.MethodGet, "/sensors?limit=9999", "")
		assert.Equal(t, http.StatusOK, rec.Code, "превышение лимита — не ошибка")
		assert.Equal(t, int32(maxLimit), svc2.gotLimit)
	})

	t.Run("controller_id доходит до сервиса как фильтр", func(t *testing.T) {
		id := uuid.New()
		svc := &fakeSensorService{}

		rec := doRequest(t, newSensorTestServer(svc), http.MethodGet, "/sensors?controller_id="+id.String(), "")

		require.Equal(t, http.StatusOK, rec.Code)
		require.NotNil(t, svc.gotFilter.ControllerID)
		assert.Equal(t, id, *svc.gotFilter.ControllerID)
	})

	t.Run("без controller_id фильтр пустой", func(t *testing.T) {
		svc := &fakeSensorService{}
		doRequest(t, newSensorTestServer(svc), http.MethodGet, "/sensors", "")
		assert.Nil(t, svc.gotFilter.ControllerID)
	})

	t.Run("битый controller_id — 400, а не 500 и не молчаливое игнорирование", func(t *testing.T) {
		rec := doRequest(t, newSensorTestServer(&fakeSensorService{}), http.MethodGet, "/sensors?controller_id=garbage", "")

		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, codeValidation, decodeError(t, rec).Code)
	})

	t.Run("фильтр без совпадений — 200 и пустой список, а не 404", func(t *testing.T) {
		svc := &fakeSensorService{
			listFn: func(_ context.Context, _ model.SensorFilter, _, _ int32) ([]model.Sensor, int64, error) {
				return []model.Sensor{}, 0, nil
			},
		}

		rec := doRequest(t, newSensorTestServer(svc), http.MethodGet, "/sensors?controller_id="+uuid.New().String(), "")

		assert.Equal(t, http.StatusOK, rec.Code, "фильтр — не поиск конкретной записи")
	})
}

func TestGetSensor(t *testing.T) {
	t.Run("несуществующий id — 404", func(t *testing.T) {
		svc := &fakeSensorService{
			getFn: func(_ context.Context, _ uuid.UUID) (model.Sensor, error) {
				return model.Sensor{}, model.ErrNotFound
			},
		}

		rec := doRequest(t, newSensorTestServer(svc), http.MethodGet, "/sensors/"+uuid.New().String(), "")

		require.Equal(t, http.StatusNotFound, rec.Code)
		assert.Equal(t, codeNotFound, decodeError(t, rec).Code)
	})

	t.Run("не-uuid в пути — 400", func(t *testing.T) {
		rec := doRequest(t, newSensorTestServer(&fakeSensorService{}), http.MethodGet, "/sensors/not-a-uuid", "")

		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, codeValidation, decodeError(t, rec).Code)
	})
}

func TestCreateSensor(t *testing.T) {
	controllerID := uuid.New()

	t.Run("валидное тело — 201", func(t *testing.T) {
		svc := &fakeSensorService{
			createFn: func(_ context.Context, in model.CreateSensorInput) (model.Sensor, error) {
				return model.Sensor{ID: uuid.New(), ControllerID: in.ControllerID, Name: in.Name}, nil
			},
		}

		rec := doRequest(t, newSensorTestServer(svc), http.MethodPost, "/sensors", validSensorBody(controllerID))

		assert.Equal(t, http.StatusCreated, rec.Code)
	})

	t.Run("без обязательного name — 400 с json-именем поля", func(t *testing.T) {
		body := `{"controller_id":"` + controllerID.String() + `","topic":"t","metric_type":"temperature","unit":"°C"}`

		rec := doRequest(t, newSensorTestServer(&fakeSensorService{}), http.MethodPost, "/sensors", body)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		errBody := decodeError(t, rec)
		assert.Equal(t, codeValidation, errBody.Code)
		assert.Contains(t, errBody.Fields, "name", "фронт сопоставляет ошибку с полем формы по json-имени")
	})

	t.Run("без controller_id — 400", func(t *testing.T) {
		body := `{"name":"s","topic":"t","metric_type":"temperature","unit":"°C"}`

		rec := doRequest(t, newSensorTestServer(&fakeSensorService{}), http.MethodPost, "/sensors", body)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, decodeError(t, rec).Fields, "controller_id")
	})

	t.Run("неизвестное поле — 400, а не тихое игнорирование", func(t *testing.T) {
		body := `{"controller_id":"` + controllerID.String() + `","name":"s","topic":"t","metric_type":"temperature","unit":"°C","typo_field":1}`

		rec := doRequest(t, newSensorTestServer(&fakeSensorService{}), http.MethodPost, "/sensors", body)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, codeValidation, decodeError(t, rec).Code)
	})

	t.Run("min_threshold больше max_threshold — 400 ещё до похода в БД", func(t *testing.T) {
		svc := &fakeSensorService{
			createFn: func(_ context.Context, _ model.CreateSensorInput) (model.Sensor, error) {
				t.Fatal("сервис не должен вызываться: пороги невалидны, это ловится в Go")
				return model.Sensor{}, nil
			},
		}
		body := `{"controller_id":"` + controllerID.String() + `","name":"s","topic":"t",
			"metric_type":"temperature","unit":"°C","min_threshold":50,"max_threshold":10}`

		rec := doRequest(t, newSensorTestServer(svc), http.MethodPost, "/sensors", body)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		errBody := decodeError(t, rec)
		assert.Equal(t, codeValidation, errBody.Code)
		assert.Contains(t, errBody.Fields, "min_threshold")
	})

	t.Run("один порог без второго — валидно", func(t *testing.T) {
		called := false
		svc := &fakeSensorService{
			createFn: func(_ context.Context, _ model.CreateSensorInput) (model.Sensor, error) {
				called = true
				return model.Sensor{ID: uuid.New()}, nil
			},
		}
		body := `{"controller_id":"` + controllerID.String() + `","name":"s","topic":"t",
			"metric_type":"temperature","unit":"°C","min_threshold":10}`

		rec := doRequest(t, newSensorTestServer(svc), http.MethodPost, "/sensors", body)

		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.True(t, called, "с одним порогом сравнивать не с чем — запрос должен дойти до сервиса")
	})

	t.Run("занятый topic — 409", func(t *testing.T) {
		svc := &fakeSensorService{
			createFn: func(_ context.Context, _ model.CreateSensorInput) (model.Sensor, error) {
				return model.Sensor{}, model.ErrDuplicate
			},
		}

		rec := doRequest(t, newSensorTestServer(svc), http.MethodPost, "/sensors", validSensorBody(controllerID))

		require.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, codeDuplicate, decodeError(t, rec).Code)
	})

	t.Run("несуществующий контроллер — 400 с указанием поля, а не 500", func(t *testing.T) {
		svc := &fakeSensorService{
			createFn: func(_ context.Context, _ model.CreateSensorInput) (model.Sensor, error) {
				return model.Sensor{}, model.ErrReferenceNotFound
			},
		}

		rec := doRequest(t, newSensorTestServer(svc), http.MethodPost, "/sensors", validSensorBody(controllerID))

		require.Equal(t, http.StatusBadRequest, rec.Code)
		errBody := decodeError(t, rec)
		assert.Equal(t, codeValidation, errBody.Code)
		assert.Contains(t, errBody.Fields, "controller_id",
			"клиент должен понять, какое именно поле ссылается в пустоту")
	})

	t.Run("нарушение CHECK в БД — 400, а не 500", func(t *testing.T) {
		svc := &fakeSensorService{
			createFn: func(_ context.Context, _ model.CreateSensorInput) (model.Sensor, error) {
				return model.Sensor{}, model.ErrConstraintViolation
			},
		}

		rec := doRequest(t, newSensorTestServer(svc), http.MethodPost, "/sensors", validSensorBody(controllerID))

		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, codeValidation, decodeError(t, rec).Code)
	})
}

func TestUpdateSensor(t *testing.T) {
	t.Run("передано только name — остальные поля уходят в сервис как nil", func(t *testing.T) {
		svc := &fakeSensorService{
			updateFn: func(_ context.Context, _ uuid.UUID, _ model.UpdateSensorInput) (model.Sensor, error) {
				return model.Sensor{ID: uuid.New(), Name: "Новое имя"}, nil
			},
		}

		rec := doRequest(t, newSensorTestServer(svc), http.MethodPatch, "/sensors/"+uuid.New().String(), `{"name":"Новое имя"}`)

		require.Equal(t, http.StatusOK, rec.Code)
		require.NotNil(t, svc.gotUpdate.Name)
		assert.Equal(t, "Новое имя", *svc.gotUpdate.Name)
		assert.Nil(t, svc.gotUpdate.Topic, "непереданное поле не должно превращаться в пустое значение")
		assert.Nil(t, svc.gotUpdate.Unit)
		assert.Nil(t, svc.gotUpdate.IsActive)
	})

	t.Run("несуществующий id — 404", func(t *testing.T) {
		svc := &fakeSensorService{
			updateFn: func(_ context.Context, _ uuid.UUID, _ model.UpdateSensorInput) (model.Sensor, error) {
				return model.Sensor{}, model.ErrNotFound
			},
		}

		rec := doRequest(t, newSensorTestServer(svc), http.MethodPatch, "/sensors/"+uuid.New().String(), `{"name":"x"}`)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("оба порога в неверном порядке — 400", func(t *testing.T) {
		rec := doRequest(t, newSensorTestServer(&fakeSensorService{}), http.MethodPatch,
			"/sensors/"+uuid.New().String(), `{"min_threshold":50,"max_threshold":10}`)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, decodeError(t, rec).Fields, "min_threshold")
	})
}

func TestDeleteSensor(t *testing.T) {
	t.Run("успешное удаление — 204 без тела", func(t *testing.T) {
		svc := &fakeSensorService{
			deleteFn: func(_ context.Context, _ uuid.UUID) error { return nil },
		}

		rec := doRequest(t, newSensorTestServer(svc), http.MethodDelete, "/sensors/"+uuid.New().String(), "")

		require.Equal(t, http.StatusNoContent, rec.Code)
		assert.Empty(t, rec.Body.String())
	})

	t.Run("несуществующий id — 404", func(t *testing.T) {
		svc := &fakeSensorService{
			deleteFn: func(_ context.Context, _ uuid.UUID) error { return model.ErrNotFound },
		}

		rec := doRequest(t, newSensorTestServer(svc), http.MethodDelete, "/sensors/"+uuid.New().String(), "")

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}
