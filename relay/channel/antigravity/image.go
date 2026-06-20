package antigravity

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/gemini"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func convertAntigravityImageRequest(c *gin.Context, adaptor *Adaptor, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	if info == nil {
		return nil, errors.New("antigravity channel: missing relay info")
	}
	resolvedModel, err := resolveRequestedModel(info.UpstreamModelName)
	if err != nil {
		return nil, err
	}
	info.UpstreamModelName = resolvedModel

	parts := []any{dto.MediaContent{Type: dto.ContentTypeText, Text: strings.TrimSpace(request.Prompt)}}
	if info.RelayMode == relayconstant.RelayModeImagesEdits {
		images, err := antigravityImageEditParts(c)
		if err != nil {
			return nil, err
		}
		if len(images) == 0 {
			return nil, errors.New("image is required for image edits")
		}
		for _, image := range images {
			parts = append(parts, image)
		}
	}

	chatRequest := &dto.GeneralOpenAIRequest{
		Model: request.Model,
		Messages: []dto.Message{{
			Role:    "user",
			Content: parts,
		}},
	}
	geminiAdaptor := gemini.Adaptor{}
	converted, err := geminiAdaptor.ConvertOpenAIRequest(c, info, chatRequest)
	if err != nil {
		return nil, err
	}
	geminiRequest, ok := converted.(*dto.GeminiChatRequest)
	if !ok {
		return nil, errors.New("antigravity channel: failed to convert image request to gemini format")
	}
	geminiRequest.GenerationConfig.ResponseModalities = []string{"IMAGE"}
	if request.N != nil && *request.N > 1 {
		count := int(*request.N)
		geminiRequest.GenerationConfig.CandidateCount = &count
	}
	if aspectRatio := antigravityImageAspectRatio(request.Size); aspectRatio != "" {
		imageConfig, err := common.Marshal(map[string]string{"aspectRatio": aspectRatio})
		if err != nil {
			return nil, err
		}
		geminiRequest.GenerationConfig.ImageConfig = imageConfig
	}
	return adaptor.buildAntigravityEnvelope(c, info, resolvedModel, geminiRequest)
}

func antigravityImageEditParts(c *gin.Context) ([]dto.MediaContent, error) {
	form, err := c.MultipartForm()
	if err != nil {
		return nil, fmt.Errorf("parse image edit form: %w", err)
	}
	if form == nil || len(form.File) == 0 {
		return nil, nil
	}

	orderedFields := []string{"image", "image[]", "reference_images", "reference_images[]", "input_reference"}
	seen := make(map[*multipart.FileHeader]struct{})
	files := make([]*multipart.FileHeader, 0, 4)
	appendFiles := func(items []*multipart.FileHeader) {
		for _, item := range items {
			if item == nil || len(files) >= 4 {
				continue
			}
			if _, exists := seen[item]; exists {
				continue
			}
			seen[item] = struct{}{}
			files = append(files, item)
		}
	}
	for _, field := range orderedFields {
		appendFiles(form.File[field])
	}
	for field, items := range form.File {
		if strings.HasPrefix(field, "image[") || strings.HasPrefix(field, "reference_images[") {
			appendFiles(items)
		}
	}

	parts := make([]dto.MediaContent, 0, len(files))
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			return nil, fmt.Errorf("open edit image: %w", err)
		}
		data, readErr := io.ReadAll(file)
		closeErr := file.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read edit image: %w", readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close edit image: %w", closeErr)
		}
		if len(data) == 0 {
			return nil, errors.New("image edit contains an empty reference image")
		}
		mimeType := strings.ToLower(strings.TrimSpace(fileHeader.Header.Get("Content-Type")))
		if !strings.HasPrefix(mimeType, "image/") {
			mimeType = http.DetectContentType(data)
		}
		if !strings.HasPrefix(strings.ToLower(mimeType), "image/") {
			return nil, fmt.Errorf("unsupported edit image content type %q", mimeType)
		}
		dataURL := "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
		parts = append(parts, dto.MediaContent{
			Type: dto.ContentTypeImageURL,
			ImageUrl: &dto.MessageImageUrl{
				Url:      dataURL,
				MimeType: mimeType,
				Detail:   "high",
			},
		})
	}
	return parts, nil
}

func antigravityImageAspectRatio(size string) string {
	switch strings.TrimSpace(size) {
	case "1024x1792", "9:16":
		return "9:16"
	case "1792x1024", "16:9":
		return "16:9"
	case "1536x1024", "3:2":
		return "3:2"
	case "1024x1536", "2:3":
		return "2:3"
	case "", "256x256", "512x512", "1024x1024", "1:1":
		return "1:1"
	default:
		return ""
	}
}

func antigravityImageHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	var geminiResponse dto.GeminiChatResponse
	if err := common.Unmarshal(body, &geminiResponse); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	imageResponse := dto.ImageResponse{Created: common.GetTimestamp(), Data: make([]dto.ImageData, 0)}
	for _, candidate := range geminiResponse.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.InlineData == nil || !strings.HasPrefix(strings.ToLower(part.InlineData.MimeType), "image/") || part.InlineData.Data == "" {
				continue
			}
			imageResponse.Data = append(imageResponse.Data, dto.ImageData{B64Json: part.InlineData.Data})
		}
	}
	if len(imageResponse.Data) == 0 {
		return nil, types.NewOpenAIError(errors.New("antigravity image response contained no image data"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	responseBody, err := common.Marshal(imageResponse)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
	}
	service.IOCopyBytesGracefully(c, resp, responseBody)

	promptTokens := info.GetEstimatePromptTokens()
	if promptTokens < 1 {
		promptTokens = 1
	}
	usage := &dto.Usage{PromptTokens: promptTokens, CompletionTokens: 0, TotalTokens: promptTokens}
	return usage, nil
}
