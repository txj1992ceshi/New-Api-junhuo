package model

import (
	"sort"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func IsChannelEnabledForGroupModel(group string, modelName string, channelID int) bool {
	if group == "" || modelName == "" || channelID <= 0 {
		return false
	}
	if !common.MemoryCacheEnabled {
		return isChannelEnabledForGroupModelDB(group, modelName, channelID)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	if group2model2channels == nil {
		return false
	}

	if isChannelIDInList(group2model2channels[group][modelName], channelID) {
		return true
	}
	normalized := ratio_setting.FormatMatchingModelName(modelName)
	if normalized != "" && normalized != modelName {
		return isChannelIDInList(group2model2channels[group][normalized], channelID)
	}
	return false
}

func IsChannelEnabledForAnyGroupModel(groups []string, modelName string, channelID int) bool {
	if len(groups) == 0 {
		return false
	}
	for _, g := range groups {
		if IsChannelEnabledForGroupModel(g, modelName, channelID) {
			return true
		}
	}
	return false
}

func GetEnabledChannelsForGroupModel(group string, modelName string) []*Channel {
	if group == "" || modelName == "" {
		return nil
	}
	if !common.MemoryCacheEnabled {
		return getEnabledChannelsForGroupModelDB(group, modelName)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	if group2model2channels == nil {
		return nil
	}

	channelIDs := group2model2channels[group][modelName]
	if len(channelIDs) == 0 {
		normalized := ratio_setting.FormatMatchingModelName(modelName)
		if normalized != "" && normalized != modelName {
			channelIDs = group2model2channels[group][normalized]
		}
	}
	if len(channelIDs) == 0 {
		return nil
	}

	channels := make([]*Channel, 0, len(channelIDs))
	for _, channelID := range channelIDs {
		if channel, ok := channelsIDM[channelID]; ok && channel != nil && channel.Status == common.ChannelStatusEnabled {
			channels = append(channels, channel)
		}
	}
	return channels
}

func getEnabledChannelsForGroupModelDB(group string, modelName string) []*Channel {
	var channels []*Channel
	query := DB.Table("channels").
		Select("channels.*").
		Joins("JOIN abilities ON abilities.channel_id = channels.id").
		Where("abilities."+commonGroupCol+" = ? AND abilities.model = ? AND abilities.enabled = ? AND channels.status = ?", group, modelName, true, common.ChannelStatusEnabled).
		Order("channels.priority DESC, channels.weight DESC, channels.id ASC")

	if err := query.Find(&channels).Error; err == nil && len(channels) > 0 {
		return channels
	}

	normalized := ratio_setting.FormatMatchingModelName(modelName)
	if normalized == "" || normalized == modelName {
		return nil
	}

	channels = nil
	query = DB.Table("channels").
		Select("channels.*").
		Joins("JOIN abilities ON abilities.channel_id = channels.id").
		Where("abilities."+commonGroupCol+" = ? AND abilities.model = ? AND abilities.enabled = ? AND channels.status = ?", group, normalized, true, common.ChannelStatusEnabled).
		Order("channels.priority DESC, channels.weight DESC, channels.id ASC")
	if err := query.Find(&channels).Error; err != nil {
		return nil
	}
	return channels
}

func SortChannelsByPriorityAndWeight(channels []*Channel) {
	sort.SliceStable(channels, func(i, j int) bool {
		if channels[i] == nil || channels[j] == nil {
			return channels[j] != nil
		}
		if channels[i].GetPriority() != channels[j].GetPriority() {
			return channels[i].GetPriority() > channels[j].GetPriority()
		}
		if channels[i].GetWeight() != channels[j].GetWeight() {
			return channels[i].GetWeight() > channels[j].GetWeight()
		}
		return channels[i].Id < channels[j].Id
	})
}

func isChannelEnabledForGroupModelDB(group string, modelName string, channelID int) bool {
	var count int64
	err := DB.Model(&Ability{}).
		Where(commonGroupCol+" = ? and model = ? and channel_id = ? and enabled = ?", group, modelName, channelID, true).
		Count(&count).Error
	if err == nil && count > 0 {
		return true
	}
	normalized := ratio_setting.FormatMatchingModelName(modelName)
	if normalized == "" || normalized == modelName {
		return false
	}
	count = 0
	err = DB.Model(&Ability{}).
		Where(commonGroupCol+" = ? and model = ? and channel_id = ? and enabled = ?", group, normalized, channelID, true).
		Count(&count).Error
	return err == nil && count > 0
}

func isChannelIDInList(list []int, channelID int) bool {
	for _, id := range list {
		if id == channelID {
			return true
		}
	}
	return false
}
