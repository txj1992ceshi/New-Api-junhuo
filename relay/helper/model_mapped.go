package helper

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

func ModelMappedHelper(c *gin.Context, info *common.RelayInfo, request dto.Request) error {
	if info.ChannelMeta == nil {
		info.ChannelMeta = &common.ChannelMeta{}
	}

	isResponsesCompact := info.RelayMode == relayconstant.RelayModeResponsesCompact
	originModelName := info.OriginModelName
	baseModelName := originModelName
	if isResponsesCompact && strings.HasSuffix(originModelName, ratio_setting.CompactModelSuffix) {
		baseModelName = strings.TrimSuffix(originModelName, ratio_setting.CompactModelSuffix)
	}

	// map model name
	modelMapping := c.GetString("model_mapping")
	if modelMapping != "" && modelMapping != "{}" {
		modelMap := make(map[string]string)
		err := json.Unmarshal([]byte(modelMapping), &modelMap)
		if err != nil {
			return fmt.Errorf("unmarshal_model_mapping_failed")
		}

		mappingModelName := originModelName
		if isResponsesCompact {
			if _, exists := modelMap[originModelName]; !exists {
				mappingModelName = baseModelName
			}
		}

		// 支持链式模型重定向，最终使用链尾的模型
		currentModel := mappingModelName
		visitedModels := map[string]bool{currentModel: true}
		for {
			mappedModel, exists := modelMap[currentModel]
			if !exists || mappedModel == "" {
				break
			}
			if visitedModels[mappedModel] {
				if mappedModel == currentModel {
					if currentModel == mappingModelName {
						info.IsModelMapped = false
						return nil
					}
					info.IsModelMapped = true
					break
				}
				return errors.New("model_mapping_contains_cycle")
			}
			visitedModels[mappedModel] = true
			currentModel = mappedModel
			info.IsModelMapped = true
		}
		if info.IsModelMapped {
			if isResponsesCompact &&
				!strings.HasSuffix(currentModel, ratio_setting.CompactModelSuffix) &&
				mappingModelName == baseModelName {
				currentModel = ratio_setting.WithCompactModelSuffix(currentModel)
			}
			info.UpstreamModelName = currentModel
		}
	}

	if isResponsesCompact {
		if info.UpstreamModelName == "" {
			info.UpstreamModelName = originModelName
		}
	}
	if request != nil && info.UpstreamModelName != "" {
		request.SetModelName(info.UpstreamModelName)
	}
	return nil
}
