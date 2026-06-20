package antigravity

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

func TestAntigravityImageEditPartsLimitsAndEncodesReferences(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for index := 0; index < 5; index++ {
		part, err := writer.CreateFormFile("image[]", "reference.png")
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := part.Write([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}); err != nil {
			t.Fatalf("write image: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", &body)
	context.Request.Header.Set("Content-Type", writer.FormDataContentType())

	parts, err := antigravityImageEditParts(context)
	if err != nil {
		t.Fatalf("read edit references: %v", err)
	}
	if len(parts) != 4 {
		t.Fatalf("reference count = %d, want 4", len(parts))
	}
	for _, part := range parts {
		image := part.GetImageMedia()
		if image == nil || !strings.HasPrefix(image.Url, "data:image/") {
			t.Fatalf("reference was not encoded as image data URL: %#v", part)
		}
	}
}

func TestAntigravityImageHandlerReturnsInlineImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	response := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       ioNopCloser(`{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"ZmFrZQ=="}}]}}]}`),
	}

	usage, apiErr := antigravityImageHandler(context, &relaycommon.RelayInfo{}, response)
	if apiErr != nil {
		t.Fatalf("image handler failed: %v", apiErr)
	}
	if usage == nil || usage.TotalTokens < 1 {
		t.Fatalf("expected non-empty image usage, got %#v", usage)
	}
	if !strings.Contains(recorder.Body.String(), `"b64_json":"ZmFrZQ=="`) {
		t.Fatalf("response did not contain the inline image payload: %s", recorder.Body.String())
	}
}

func ioNopCloser(value string) *testReadCloser {
	return &testReadCloser{Reader: strings.NewReader(value)}
}

type testReadCloser struct {
	*strings.Reader
}

func (reader *testReadCloser) Close() error { return nil }
