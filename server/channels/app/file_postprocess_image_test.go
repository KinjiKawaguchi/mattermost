// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/channels/app/imaging"
)

func TestPostprocessImagePreviewFields(t *testing.T) {
	decoder, err := imaging.NewDecoder(imaging.DecoderOptions{ConcurrencyLevel: 1})
	require.NoError(t, err)
	encoder, err := imaging.NewEncoder(imaging.EncoderOptions{ConcurrencyLevel: 1})
	require.NoError(t, err)

	var pngBuf bytes.Buffer
	require.NoError(t, png.Encode(&pngBuf, image.NewRGBA(image.Rect(0, 0, 40, 30))))
	validPNG := pngBuf.Bytes()

	// The header (signature and IHDR chunk) is intact, so the image config can
	// be read, but the pixel data is cut off, so a full decode fails.
	const pngHeaderLength = 8 + 25
	truncatedPNG := validPNG[:pngHeaderLength+7]
	_, _, err = decoder.DecodeConfig(bytes.NewReader(truncatedPNG))
	require.NoError(t, err)
	_, _, release, err := decoder.DecodeMemBounded(bytes.NewReader(truncatedPNG))
	require.Error(t, err)
	require.Nil(t, release)

	newTask := func(t *testing.T) *UploadFileTask {
		t.Helper()
		return &UploadFileTask{
			Logger: mlog.CreateConsoleTestLogger(t),
			fileinfo: &model.FileInfo{
				Name:            "image.png",
				Extension:       "png",
				MimeType:        "image/png",
				HasPreviewImage: true,
				PreviewPath:     "dir/image_preview.png",
				ThumbnailPath:   "dir/image_thumb.png",
			},
			writeFile: func(r io.Reader, _ string) (int64, *model.AppError) {
				n, _ := io.Copy(io.Discard, r)
				return n, nil
			},
			imgDecoder: decoder,
			imgEncoder: encoder,
		}
	}

	t.Run("keeps the preview and thumbnail fields when the image decodes", func(t *testing.T) {
		task := newTask(t)

		task.postprocessImage(bytes.NewReader(validPNG))

		assert.True(t, task.fileinfo.HasPreviewImage)
		assert.Equal(t, "dir/image_preview.png", task.fileinfo.PreviewPath)
		assert.Equal(t, "dir/image_thumb.png", task.fileinfo.ThumbnailPath)
	})

	for name, data := range map[string][]byte{
		"the pixel data is truncated": truncatedPNG,
		"the data is not an image":    []byte("not an image"),
	} {
		t.Run("clears the preview and thumbnail fields when "+name, func(t *testing.T) {
			task := newTask(t)

			task.postprocessImage(bytes.NewReader(data))

			assert.False(t, task.fileinfo.HasPreviewImage)
			assert.Empty(t, task.fileinfo.PreviewPath)
			assert.Empty(t, task.fileinfo.ThumbnailPath)
		})
	}
}
