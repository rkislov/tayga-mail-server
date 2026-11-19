package mmplugins

import (
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"
)

// The SDK also exposes APIs added after the 12.0 compatibility target. Fail
// explicitly until their storage and authorization contracts are implemented.
func (a *unsupportedAPI) GetThreadFollowers(threadID string) ([]string, *model.AppError) {
	a.unsupported("GetThreadFollowers")
	return nil, model.NewAppError("GetThreadFollowers", "tms.plugin.unsupported", nil, "API unavailable", 501)
}
func (a *unsupportedAPI) SetThreadFollowing(userID, threadID string, following bool) *model.AppError {
	a.unsupported("SetThreadFollowing")
	return model.NewAppError("SetThreadFollowing", "tms.plugin.unsupported", nil, "API unavailable", 501)
}
func (a *unsupportedAPI) UpdatePostWithOptions(post *model.Post, options *model.UpdatePostOptions) (*model.Post, *model.AppError) {
	a.unsupported("UpdatePostWithOptions")
	return nil, model.NewAppError("UpdatePostWithOptions", "tms.plugin.unsupported", nil, "API unavailable", 501)
}
func (a *unsupportedAPI) KVListWithOptions(page, perPage int, options model.PluginKVListOptions) ([]string, *model.AppError) {
	a.unsupported("KVListWithOptions")
	return nil, model.NewAppError("KVListWithOptions", "tms.plugin.unsupported", nil, "API unavailable", 501)
}
func (a *unsupportedAPI) GetSharedChannelCursor(channelID, remoteID string) (model.GetPostsSinceForSyncCursor, error) {
	a.unsupported("GetSharedChannelCursor")
	return model.GetPostsSinceForSyncCursor{}, fmt.Errorf("unsupported plugin API: GetSharedChannelCursor")
}
