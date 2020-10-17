package main

import (
	"fmt"
	"net/http"
	"strings"
	"regexp"
	"sync"
	"reflect"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/pkg/errors"
)

type configuration struct {
		Channel string
		AllowedUsers string
		ChannelNoUpdate string
}

// Clone shallow copies the configuration. Your implementation may require a deep copy if
// your configuration has reference types.
func (c *configuration) Clone() *configuration {
	var clone = *c
	return &clone
}

// getConfiguration retrieves the active configuration under lock, making it safe to use
// concurrently. The active configuration may change underneath the client of this method, but
// the struct returned by this API call is considered immutable.
func (p *Plugin) getConfiguration() *configuration {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()

	if p.configuration == nil {
		return &configuration{}
	}

	return p.configuration
}

// setConfiguration replaces the active configuration under lock.
//
// Do not call setConfiguration while holding the configurationLock, as sync.Mutex is not
// reentrant. In particular, avoid using the plugin API entirely, as this may in turn trigger a
// hook back into the plugin. If that hook attempts to acquire this lock, a deadlock may occur.
//
// This method panics if setConfiguration is called with the existing configuration. This almost
// certainly means that the configuration was modified without being cloned and may result in
// an unsafe access.
func (p *Plugin) setConfiguration(configuration *configuration) {
	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()

	if configuration != nil && p.configuration == configuration {
		// Ignore assignment if the configuration struct is empty. Go will optimize the
		// allocation for same to point at the same memory address, breaking the check
		// above.
		if reflect.ValueOf(*configuration).NumField() == 0 {
			return
		}

		panic("setConfiguration called with the existing configuration")
	}

	p.configuration = configuration
}

// OnConfigurationChange is invoked when configuration changes may have been made.
func (p *Plugin) OnConfigurationChange() error {
	var configuration = new(configuration)

	// Load the public configuration fields from the Mattermost server configuration.
	if err := p.API.LoadPluginConfiguration(configuration); err != nil {
		return errors.Wrap(err, "failed to load plugin configuration")
	}

	p.setConfiguration(configuration)
	return nil
}

type Plugin struct {
	plugin.MattermostPlugin

	configurationLock        sync.RWMutex
	configuration *configuration
}

func main() {
	plugin.ClientMain(&Plugin{})
}

func (p *Plugin) OnActivate() error {
	return nil
}

func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	switch path := r.URL.Path; path {
	default:
		http.NotFound(w, r)
	}
}

func (p *Plugin) FilterPost(post *model.Post) (*model.Post, string) {
	configuration := p.getConfiguration()
	// Check if channel is the forbidden channel
	channel, err := p.API.GetChannel(post.ChannelId)
	if err != nil {
		p.API.LogError("Failed to find channel in post")
		return nil, ""
	}
	if strings.Contains(post.Message, "updated the channel header") {
		noUpdateChannels := strings.Split(configuration.ChannelNoUpdate, " ")
		for _, noUpdateChannel := range noUpdateChannels {
			if noUpdateChannel == channel.Name {
				return nil, "You are not allowed to post message updates in this channel"
			}
		}
	}

	if channel.Name == "ssm" {
		reg, err := regexp.Compile("[^a-z0-9 ]")
    if err != nil {
        p.API.LogError("Failed to compile regex")
    }
    ssm := reg.ReplaceAllString(post.Message, "")
		if ssm == "" {
			return nil, "NO"
		}
		post.Message = ssm
		return post, ""
	}

	if channel.Name == "ssm-v0" {
		reg, err := regexp.Compile("[^A-Z0-9! ]")
    if err != nil {
        p.API.LogError("Failed to compile regex")
    }
    ssm := reg.ReplaceAllString(post.Message, "")
		if ssm == "" {
			return nil, "NO"
		}
		post.Message = ssm
		return post, ""
	}
	// Check if the user is allowed
	user, err := p.API.GetUser(post.UserId)
	if err != nil {
		p.API.LogError("Failed to find user in post")
		return nil, ""
	}
	allowedUsers := strings.Split(configuration.AllowedUsers, " ")

	if channel.Name == "bestuur-intern"  {
		for _, allowedUser := range allowedUsers {
			if allowedUser == user.Username {
				return nil, ""
			}
		}
		if strings.HasPrefix(post.Message, "!") {
			return nil, ""
		}
		p.API.SendEphemeralPost(post.UserId, &model.Post{
			ChannelId: post.ChannelId,
			Message:   "This is the internal channel of the board, so only the board can post in it. If you think it's appropriate, override by starting your message with '!'",
			Props: map[string]interface{}{
				"sent_by_plugin": true,
			},
		})
		return nil, "This is the internal channel of the board, so only the board can post in it. If you think it's appropriate, override by starting your message with '!'"
	}

	if channel.Name != configuration.Channel {
		return post, ""
	}

	for _, allowedUser := range allowedUsers {
		if allowedUser == user.Username {
			return nil, ""
		}
	}

	p.API.SendEphemeralPost(post.UserId, &model.Post{
		ChannelId: post.ChannelId,
		Message:   "You are not allowed to post in this channel",
		Props: map[string]interface{}{
			"sent_by_plugin": true,
		},
	})
	return nil, "You are not allowed to post in this channel"
}

func (p *Plugin) MessageWillBePosted(c *plugin.Context, post *model.Post) (*model.Post, string) {
	return p.FilterPost(post)
}

func (p *Plugin) MessageWillBeUpdated(c *plugin.Context, newPost *model.Post, _ *model.Post) (*model.Post, string) {
	return p.FilterPost(newPost)
}

func (p *Plugin) UserHasLeftChannel(c *plugin.Context, channelMember *model.ChannelMember, actor *model.User) {
	if actor == nil || channelMember.UserId == actor.Id {
		// User removed themselves from the channel
		return
	}

	kickedUser, err := p.API.GetUser(channelMember.UserId)
	if err != nil {
		p.API.LogError("Failed to find user")
		return
	}

	msg := fmt.Sprintf("[BOT] I just kicked @%s from the channel", kickedUser.Username)
	p.API.CreatePost(&model.Post{
		UserId:    actor.Id,
		ChannelId: channelMember.ChannelId,
		Message:   msg,
	})

}
