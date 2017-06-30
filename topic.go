package goros

import (
	"encoding/json"
	"fmt"
)

type Topic struct {
	Op    string        `json:"op"`
	Id    string        `json:"id,omitempty"`
	Topic string        `json:"topic"`
	Type  string        `json:"type,omitempty"`
	Throttle_rate int   `json:"throttle_rate,omitempty"` //In msec
	Queue_length  int   `json:"queue_length,omitempty"`  //Default: 1
	Fragment_size int   `json:"fragment_size,omitempty"`
	Compression  string `json:"compression,omitempty"`
	Msg json.RawMessage `json:"msg,omitempty"`
}

type TopicCallback func(*json.RawMessage)

func NewTopic(topicName string) *Topic {
	topic := &Topic{Op: "subscribe", Topic: topicName}
	//topic.Id = fmt.Sprintf("%s:%s:%d", topic.Op, topic.Topic, messageCount)
	//messageCount++
        SetNewTopicId(topic)

	return topic
}

func SetNewTopicId(topic *Topic) {
	topic.Id = fmt.Sprintf("%s:%s:%d", topic.Op, topic.Topic, messageCount)
	messageCount++
}

