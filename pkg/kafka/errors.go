package kafka

import (
	"errors"
	"strings"

	"github.com/twmb/franz-go/pkg/kerr"
)

var kerrTopicExists = kerr.TopicAlreadyExists

func isTopicExists(err error) bool {
	return errors.Is(err, kerr.TopicAlreadyExists) || strings.Contains(err.Error(), "TOPIC_ALREADY_EXISTS")
}
