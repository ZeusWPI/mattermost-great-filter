package main

import (
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"
)

type Plugin struct {
	plugin.MattermostPlugin

	Channel      string
	AllowedUsers string
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
