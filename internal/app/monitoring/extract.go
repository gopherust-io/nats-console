package monitoring

import (
	"maps"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
)

// ParsePayload unmarshals raw /jsz bytes into the typed topology payload.
func ParsePayload(raw []byte) (JSZTopologyPayload, error) {
	var payload JSZTopologyPayload
	if err := serializer.Unmarshal(raw, &payload); err != nil {
		return JSZTopologyPayload{}, err
	}
	return payload, nil
}

// ParsePayload is a Service method alias for ParsePayload.
func (s *Service) ParsePayload(raw []byte) (JSZTopologyPayload, error) {
	return ParsePayload(raw)
}

// ZombieStreamsFromJSZ extracts zombie-analysis inputs from a /jsz body.
func ZombieStreamsFromJSZ(raw []byte) ([]domain.ZombieStreamInput, error) {
	payload, err := ParsePayload(raw)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ZombieStreamInput, 0)
	for _, acct := range payload.AccountDetails {
		for _, stream := range acct.StreamDetail {
			in := domain.ZombieStreamInput{Name: stream.Name}
			if stream.Config != nil {
				in.Subjects = append([]string(nil), stream.Config.Subjects...)
			}
			if stream.State != nil {
				in.Messages = stream.State.Messages
				in.LastSeq = stream.State.LastSeq
			}
			for _, c := range stream.ConsumerDetail {
				cin := domain.ZombieConsumerInput{Name: c.Name}
				if c.Config != nil {
					cin.FilterSubject = c.Config.FilterSubject
					cin.FilterSubjects = append([]string(nil), c.Config.FilterSubjects...)
				}
				if c.Delivered != nil {
					cin.DeliveredConsSeq = c.Delivered.ConsumerSeq
					cin.DeliveredStrSeq = c.Delivered.StreamSeq
				}
				in.Consumers = append(in.Consumers, cin)
			}
			out = append(out, in)
		}
	}
	return out, nil
}

// ZombieStreamsFromJSZ is a Service method alias.
func (s *Service) ZombieStreamsFromJSZ(raw []byte) ([]domain.ZombieStreamInput, error) {
	return ZombieStreamsFromJSZ(raw)
}

// SubjectNamingInputsFromJSZ extracts subject-naming inputs from a /jsz body.
func SubjectNamingInputsFromJSZ(raw []byte) ([]domain.SubjectNamingInput, error) {
	payload, err := ParsePayload(raw)
	if err != nil {
		return nil, err
	}
	out := make([]domain.SubjectNamingInput, 0)
	for _, acct := range payload.AccountDetails {
		for _, stream := range acct.StreamDetail {
			in := domain.SubjectNamingInput{Name: stream.Name}
			if stream.Config != nil {
				in.Subjects = append([]string(nil), stream.Config.Subjects...)
			}
			for _, c := range stream.ConsumerDetail {
				cin := domain.SubjectNamingConsumerInput{Name: c.Name}
				if c.Config != nil {
					cin.FilterSubject = c.Config.FilterSubject
					cin.FilterSubjects = append([]string(nil), c.Config.FilterSubjects...)
				}
				in.Consumers = append(in.Consumers, cin)
			}
			out = append(out, in)
		}
	}
	return out, nil
}

// SubjectNamingInputsFromJSZ is a Service method alias.
func (s *Service) SubjectNamingInputsFromJSZ(raw []byte) ([]domain.SubjectNamingInput, error) {
	return SubjectNamingInputsFromJSZ(raw)
}

// EventGenomeInputsFromJSZ extracts event-genome inputs from a /jsz body.
func EventGenomeInputsFromJSZ(raw []byte) ([]domain.EventGenomeInput, error) {
	payload, err := ParsePayload(raw)
	if err != nil {
		return nil, err
	}
	out := make([]domain.EventGenomeInput, 0)
	for _, acct := range payload.AccountDetails {
		for _, stream := range acct.StreamDetail {
			in := domain.EventGenomeInput{Name: stream.Name}
			if stream.Config != nil {
				in.Subjects = append([]string(nil), stream.Config.Subjects...)
			}
			for _, c := range stream.ConsumerDetail {
				cin := domain.EventGenomeConsumerInput{Name: c.Name}
				if c.Config != nil {
					cin.FilterSubject = c.Config.FilterSubject
					cin.FilterSubjects = append([]string(nil), c.Config.FilterSubjects...)
				}
				in.Consumers = append(in.Consumers, cin)
			}
			out = append(out, in)
		}
	}
	return out, nil
}

// EventGenomeInputsFromJSZ is a Service method alias.
func (s *Service) EventGenomeInputsFromJSZ(raw []byte) ([]domain.EventGenomeInput, error) {
	return EventGenomeInputsFromJSZ(raw)
}

// ChaosStoryInputsFromJSZ extracts chaos-story inventory inputs from a /jsz body.
func ChaosStoryInputsFromJSZ(raw []byte) ([]domain.ChaosStoryInventoryInput, error) {
	payload, err := ParsePayload(raw)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ChaosStoryInventoryInput, 0)
	for _, acct := range payload.AccountDetails {
		for _, stream := range acct.StreamDetail {
			in := domain.ChaosStoryInventoryInput{Name: stream.Name}
			if stream.Config != nil {
				in.Subjects = append([]string(nil), stream.Config.Subjects...)
			}
			for _, c := range stream.ConsumerDetail {
				in.Consumers = append(in.Consumers, c.Name)
			}
			out = append(out, in)
		}
	}
	return out, nil
}

// ChaosStoryInputsFromJSZ is a Service method alias.
func (s *Service) ChaosStoryInputsFromJSZ(raw []byte) ([]domain.ChaosStoryInventoryInput, error) {
	return ChaosStoryInputsFromJSZ(raw)
}

// EventCatalogLiveFromJSZ extracts event-catalog live streams from a /jsz body.
func EventCatalogLiveFromJSZ(raw []byte) ([]domain.EventCatalogLiveStream, error) {
	payload, err := ParsePayload(raw)
	if err != nil {
		return nil, err
	}
	out := make([]domain.EventCatalogLiveStream, 0)
	for _, acct := range payload.AccountDetails {
		for _, stream := range acct.StreamDetail {
			in := domain.EventCatalogLiveStream{Name: stream.Name}
			if stream.Config != nil {
				in.Subjects = append([]string(nil), stream.Config.Subjects...)
			}
			for _, c := range stream.ConsumerDetail {
				cin := domain.EventCatalogLiveConsumer{Name: c.Name}
				if c.Config != nil {
					cin.FilterSubject = c.Config.FilterSubject
					cin.FilterSubjects = append([]string(nil), c.Config.FilterSubjects...)
					cin.DurableName = c.Config.DurableName
					if len(c.Config.Metadata) > 0 {
						cin.Metadata = cloneStringMap(c.Config.Metadata)
					}
				}
				in.Consumers = append(in.Consumers, cin)
			}
			out = append(out, in)
		}
	}
	return out, nil
}

// EventCatalogLiveFromJSZ is a Service method alias.
func (s *Service) EventCatalogLiveFromJSZ(raw []byte) ([]domain.EventCatalogLiveStream, error) {
	return EventCatalogLiveFromJSZ(raw)
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	maps.Copy(out, in)
	return out
}
