// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/platform/shared/web"
)

// lazyMissingReader behaves like the reader an S3 file store hands out for an
// object that doesn't exist: opening it succeeds and the first Seek fails.
type lazyMissingReader struct{}

func (lazyMissingReader) Read([]byte) (int, error) {
	return 0, errors.New("The specified key does not exist.")
}

func (lazyMissingReader) Seek(int64, int) (int64, error) {
	return 0, errors.New("The specified key does not exist.")
}

func TestWriteImageFileResponse(t *testing.T) {
	info := &model.FileInfo{Id: model.NewId(), Name: "image.png", UpdateAt: model.GetMillis()}
	request := func() *http.Request {
		return httptest.NewRequest(http.MethodGet, "/api/v4/files/"+info.Id+"/thumbnail", nil)
	}

	t.Run("serving a missing object directly answers 500", func(t *testing.T) {
		recorder := httptest.NewRecorder()

		web.WriteFileResponse(info.Name, ThumbnailImageType, 0, model.GetTimeForMillis(info.UpdateAt), "", lazyMissingReader{}, false, recorder, request())

		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	})

	t.Run("a missing object is reported as 404 without writing a response", func(t *testing.T) {
		recorder := httptest.NewRecorder()

		appErr := writeImageFileResponse(recorder, request(), info, ThumbnailImageType, "", false, lazyMissingReader{})

		require.NotNil(t, appErr)
		assert.Equal(t, http.StatusNotFound, appErr.StatusCode)
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Zero(t, recorder.Body.Len())
		assert.Empty(t, recorder.Header().Get("Content-Type"))
	})

	t.Run("an existing object is written in full", func(t *testing.T) {
		data := []byte("thumbnail bytes")
		recorder := httptest.NewRecorder()

		appErr := writeImageFileResponse(recorder, request(), info, ThumbnailImageType, "", false, bytes.NewReader(data))

		require.Nil(t, appErr)
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, ThumbnailImageType, recorder.Header().Get("Content-Type"))
		assert.Equal(t, data, recorder.Body.Bytes())
	})
}
