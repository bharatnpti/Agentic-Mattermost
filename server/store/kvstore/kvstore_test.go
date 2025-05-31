package kvstore

import (
	"encoding/json"
	"net/http" // For http.StatusNotFound
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	// "github.com/stretchr/testify/mock" // plugintest.API embeds mock.Mock, not directly used here
)

func TestGetTemplateData(t *testing.T) {
	userID := "testUserID"
	expectedKey := "template_key-" + userID
	expectedDataString := "some_template_data"

	t.Run("Successful retrieval", func(t *testing.T) {
		api := &plugintest.API{}
		driver := &plugintest.Driver{} // Use plugintest.Driver

		// The KVService.Get method (which plugin.API.KVGet ultimately wraps after some indirection via pluginapi.Client)
		// expects a pointer to unmarshal into. The actual `plugin.API.KVGet` returns `([]byte, *model.AppError)`.
		// The `startertemplate.go` code passes `&templateData` (a `*string`) to `client.KV.Get`.
		// The `pluginapi.KVService.Get` method itself handles the unmarshaling.
		// So, when mocking `plugin.API.KVGet`, we need to return bytes that will correctly unmarshal to `expectedDataString`.
		expectedBytes, errMarshal := json.Marshal(expectedDataString)
		assert.NoError(t, errMarshal)

		// plugin.API.KVGet(key string) ([]byte, *model.AppError)
		api.On("KVGet", expectedKey).Return(expectedBytes, nil).Once()

		client := pluginapi.NewClient(api, driver)
		kvStore := NewKVStore(client)

		data, err := kvStore.GetTemplateData(userID)

		assert.NoError(t, err)
		assert.Equal(t, expectedDataString, data)
		api.AssertExpectations(t)
	})

	t.Run("KV Get returns an AppError", func(t *testing.T) {
		api := &plugintest.API{}
		driver := &plugintest.Driver{}
		expectedAppError := model.NewAppError("API.KVGet", "kv.get.error", nil, "kv get error", 0)

		api.On("KVGet", expectedKey).Return(nil, expectedAppError).Once()

		client := pluginapi.NewClient(api, driver)
		kvStore := NewKVStore(client)

		data, err := kvStore.GetTemplateData(userID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get template data") // This is the wrapped error message
		cause := errors.Cause(err)
		if assert.NotNil(t, cause) {
			// Check against DetailedError as pluginapi.KVService.Get might return appErr.ToError(),
			// and appErr.ToError().Error() returns DetailedError.
			// Let's use Contains for more robustness with AppError messages
			assert.Contains(t, err.Error(), expectedAppError.DetailedError)
		}
		assert.Empty(t, data)
		api.AssertExpectations(t)
	})

	t.Run("KV Get returns no error but no data (empty string)", func(t *testing.T) {
		api := &plugintest.API{}
		driver := &plugintest.Driver{}

		// If KVGet returns nil bytes and no error, json.Unmarshal(nil, &str) results in an empty string.
		var nilBytes []byte
		api.On("KVGet", expectedKey).Return(nilBytes, nil).Once()

		client := pluginapi.NewClient(api, driver)
		kvStore := NewKVStore(client)

		data, err := kvStore.GetTemplateData(userID)

		assert.NoError(t, err)
		assert.Equal(t, "", data)
		api.AssertExpectations(t)
	})

	t.Run("KV Get returns 'key not found' AppError", func(t *testing.T) {
		api := &plugintest.API{}
		driver := &plugintest.Driver{}
		appErrNotFound := model.NewAppError("API.KVGet", "store.kvstore.get.app_error", nil, "key not found", http.StatusNotFound)

		api.On("KVGet", expectedKey).Return(nil, appErrNotFound).Once()

		client := pluginapi.NewClient(api, driver)
		kvStore := NewKVStore(client)

		data, err := kvStore.GetTemplateData(userID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get template data") // Wrapped error
		cause := errors.Cause(err)
		if assert.NotNil(t, cause) {
			// Check against DetailedError as pluginapi.KVService.Get might return appErr.ToError(),
			// and appErr.ToError().Error() returns DetailedError.
			// The actual error message appears to be simplified to "not found".
			assert.Contains(t, err.Error(), "not found")
		}
		assert.Empty(t, data)
		api.AssertExpectations(t)
	})
}
