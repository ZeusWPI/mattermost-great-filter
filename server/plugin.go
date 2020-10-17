package main

import (
	"fmt"
	"net/http"
	"strings"
	"regexp"

	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"
)

type Plugin struct {
	plugin.MattermostPlugin

	Channel         string
	AllowedUsers    string
	ChannelNoUpdate string
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
	// Check if channel is the forbidden channel
	channel, err := p.API.GetChannel(post.ChannelId)
	if err != nil {
		p.API.LogError("Failed to find channel in post")
		return nil, ""
	}
	if strings.Contains(post.Message, "updated the channel header") {
		noUpdateChannels := strings.Split(p.ChannelNoUpdate, " ")
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

	if channel.Name != p.Channel {
		return post, ""
	}

	// Check if the user is allowed
	user, err := p.API.GetUser(post.UserId)

	if err != nil {
		p.API.LogError("Failed to find user in post")
		return nil, ""
	}

	allowedUsers := strings.Split(p.AllowedUsers, " ")
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
