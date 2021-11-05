package messages

import (
	"encoding/json"
	"errors"
)

type Service struct {
	Name      string   `json:"name"`
	Port      int      `json:"port"`
	Proto     string   `json:"proto"`
	Tags      []string `json:"tags"`
	Interface string   `json:"interface"`
}

type AgentInfoMessage struct {
	Name     string    `json:"name"`
	Services []Service `json:"services"`
}

func UnmarshalAgentInfo(buf []byte) (AgentInfoMessage, error) {
	var message AgentInfoMessage
	err := json.Unmarshal(buf, &message)

	if err == nil {
		return message, nil
	}

	return message, errors.New("unable to unmarshal message")
}

func (m AgentInfoMessage) Marshal() []byte {
	message, err := json.Marshal(m)

	if err == nil {
		return message
	}

	return nil
}
